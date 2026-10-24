package main

import (
	"errors"
	"path/filepath"

	"github.com/bensyverson/contextfill/fill"
	"github.com/bensyverson/contextfill/opencode"
	"github.com/bensyverson/contextfill/transcript"
)

// target is a harness's reader, plus what to tell a user who can't find a
// session in it.
type target struct {
	source fill.Source
	// where says where the harness keeps sessions.
	where string
	// list is the command that lists its sessions.
	list string
}

// open returns the reader for the harness. An error is an environment error.
func open(h fill.Harness, env func(string) string) (target, error) {
	if h == fill.OpenCode {
		path, err := opencode.ServicePath(env)
		if err != nil {
			return target{}, err
		}
		service, err := opencode.LoadService(path)
		if err != nil {
			return target{}, err
		}
		return target{
			source: opencode.NewClient(service),
			where:  "OpenCode's server at " + service.URL + " serves its sessions.",
			list:   "opencode session list",
		}, nil
	}
	projects, err := projectsDir(env)
	if err != nil {
		return target{}, err
	}
	return target{
		source: transcript.Projects{Dir: projects},
		where:  "Transcripts are " + projects + "/<project>/<session-id>.jsonl.",
		list:   "ls " + projects + "/*/",
	}, nil
}

// projectsDir is where Claude Code keeps transcripts: $CLAUDE_CONFIG_DIR, or
// ~/.claude, then projects.
func projectsDir(env func(string) string) (string, error) {
	if config := env("CLAUDE_CONFIG_DIR"); config != "" {
		return filepath.Join(config, "projects"), nil
	}
	home := env("HOME")
	if home == "" {
		return "", errors.New("neither CLAUDE_CONFIG_DIR nor HOME is set, so there is nowhere to look for transcripts")
	}
	return filepath.Join(home, ".claude", "projects"), nil
}

// harnessName is how a message names the harness.
func harnessName(h fill.Harness) string {
	if h == fill.OpenCode {
		return "OpenCode"
	}
	return "Claude Code"
}

// display is how a listing shows a session id: Claude Code's UUIDs are told
// apart by their first eight characters, but OpenCode's ids share a long
// time-ordered prefix, so they're shown whole.
func display(h fill.Harness, id string) string {
	if h == fill.OpenCode {
		return id
	}
	return short(id)
}
