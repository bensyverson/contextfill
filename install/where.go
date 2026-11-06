package install

import (
	"errors"
	"os"
	"path/filepath"
)

// Place is where an app keeps what install edits, and whether the app is
// there at all.
type Place struct {
	// Path is Claude Code's settings file, or OpenCode's plugin link.
	Path string
	// Present means the app's config directory exists.
	Present bool
}

var errNoHome = errors.New("HOME is not set, so there is nowhere to install")

// ClaudePlace is Claude Code's user settings file: settings.json in
// $CLAUDE_CONFIG_DIR, or in ~/.claude.
func ClaudePlace(env func(string) string) (Place, error) {
	dir := env("CLAUDE_CONFIG_DIR")
	if dir == "" {
		home := env("HOME")
		if home == "" {
			return Place{}, errNoHome
		}
		dir = filepath.Join(home, ".claude")
	}
	return Place{Path: filepath.Join(dir, "settings.json"), Present: isDir(dir)}, nil
}

// OpenCodePlace is the plugin's link in OpenCode's global plugins directory,
// under $XDG_CONFIG_HOME/opencode, or ~/.config/opencode.
func OpenCodePlace(env func(string) string) (Place, error) {
	config := env("XDG_CONFIG_HOME")
	if config == "" {
		home := env("HOME")
		if home == "" {
			return Place{}, errNoHome
		}
		config = filepath.Join(home, ".config")
	}
	dir := filepath.Join(config, "opencode")
	return Place{Path: filepath.Join(dir, "plugins", pluginName), Present: isDir(dir)}, nil
}

func isDir(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}
