// Package install puts contextfill's commit hook into an agent app, and
// takes it out: a PostToolUse hook in Claude Code's settings, or a link to
// the plugin in OpenCode's plugins directory. It touches nothing else in
// either.
package install

import (
	"errors"
	"fmt"

	"github.com/bensyverson/contextfill/fill"
)

// Kind is what happened to one entry.
type Kind int

const (
	// Added means a new hook was written.
	Added Kind = iota + 1
	// Replaced means an older contextfill hook or link now runs the new one.
	Replaced
	// Removed means a contextfill hook or link was taken out.
	Removed
	// Unchanged means it was already as asked.
	Unchanged
	// Linked means the OpenCode plugin was linked.
	Linked
	// Kept means a copy of the plugin the person made was left alone.
	Kept
	// Missing means there was no plugin to link.
	Missing
)

// Change is one thing an install or uninstall did, or didn't do.
type Change struct {
	Harness fill.Harness
	Kind    Kind
	// Path is the file changed: the settings file or the plugin link.
	Path string
	// Old is the command or link target there before; New is the one there
	// now, or the one asked for.
	Old, New string
}

// ErrInvalidSettings means Claude Code's settings file isn't valid JSON, or
// isn't shaped like settings, so it can't be merged into safely.
var ErrInvalidSettings = errors.New("not valid settings JSON")

// ErrNoPlugin means this contextfill has no OpenCode plugin installed beside
// it.
var ErrNoPlugin = errors.New("no OpenCode plugin installed with this contextfill")

const reload = "run `opencode reload` to load it"

// String is the change as one line for a person.
func (c Change) String() string {
	if c.Harness == fill.OpenCode {
		return "OpenCode: " + c.openCode()
	}
	return "Claude Code: " + c.claude()
}

func (c Change) claude() string {
	switch c.Kind {
	case Added:
		return fmt.Sprintf("added a PostToolUse hook on Bash to %s: %s", c.Path, c.New)
	case Replaced:
		return fmt.Sprintf("replaced %s with %s in %s", c.Old, c.New, c.Path)
	case Removed:
		return fmt.Sprintf("removed %s from %s", c.Old, c.Path)
	case Unchanged:
		if c.New == "" {
			return fmt.Sprintf("%s has no contextfill hook; nothing changed", c.Path)
		}
		return fmt.Sprintf("%s already runs %s; nothing changed", c.Path, c.New)
	}
	return fmt.Sprintf("%v %s", c.Kind, c.Path)
}

func (c Change) openCode() string {
	switch c.Kind {
	case Linked:
		return fmt.Sprintf("linked %s to %s; %s", c.Path, c.New, reload)
	case Replaced:
		return fmt.Sprintf("relinked %s from %s to %s; %s", c.Path, c.Old, c.New, reload)
	case Removed:
		return fmt.Sprintf("removed the link %s (to %s); run `opencode reload` to unload it", c.Path, c.Old)
	case Unchanged:
		if c.New == "" {
			return fmt.Sprintf("no plugin link at %s; nothing changed", c.Path)
		}
		return fmt.Sprintf("%s already links to %s; nothing changed", c.Path, c.New)
	case Kept:
		return fmt.Sprintf("%s is a copy, not a link, so it was kept; nothing changed. Delete it and run this again to link the installed plugin", c.Path)
	case Missing:
		return fmt.Sprintf("this contextfill has no OpenCode plugin beside it (looked for %s); install contextfill with Homebrew (brew install bensyverson/tap/contextfill) to get one; nothing changed", c.New)
	}
	return fmt.Sprintf("%v %s", c.Kind, c.Path)
}
