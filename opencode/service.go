// Package opencode reads OpenCode sessions to report how full a session's
// context window is.
//
// OpenCode keeps sessions in a private SQLite store, and only its running
// server knows every model's window, plugin providers included. So this
// package asks that server, over the HTTP API its own clients use,
// authenticating with the password its background service writes beside its
// address.
package opencode

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
)

// ErrNoServer means OpenCode's background server isn't running, or hasn't
// recorded where it listens.
var ErrNoServer = errors.New("OpenCode's server isn't running")

// Service is how to reach OpenCode's background server.
type Service struct {
	// URL is the server's base address, e.g. "http://127.0.0.1:49374".
	URL string `json:"url"`
	// Password authenticates as user "opencode".
	Password string `json:"password"`
}

// ServicePath is where the background service records its address:
// $XDG_STATE_HOME/opencode/service.json, or ~/.local/state/opencode.
func ServicePath(env func(string) string) (string, error) {
	if state := env("XDG_STATE_HOME"); state != "" {
		return filepath.Join(state, "opencode", "service.json"), nil
	}
	home := env("HOME")
	if home == "" {
		return "", errors.New("neither XDG_STATE_HOME nor HOME is set, so there is nowhere to look for OpenCode's server")
	}
	return filepath.Join(home, ".local", "state", "opencode", "service.json"), nil
}

// LoadService reads the service file at path. A missing file is ErrNoServer.
func LoadService(path string) (Service, error) {
	body, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return Service{}, fmt.Errorf("%w: %s doesn't exist; start it with: opencode service start", ErrNoServer, path)
	}
	if err != nil {
		return Service{}, fmt.Errorf("reading %s: %w", path, err)
	}
	var s Service
	if err := json.Unmarshal(body, &s); err != nil {
		return Service{}, fmt.Errorf("reading %s: %w", path, err)
	}
	if s.URL == "" || s.Password == "" {
		return Service{}, fmt.Errorf("%s has no url or password", path)
	}
	return s, nil
}
