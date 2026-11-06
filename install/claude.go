package install

import (
	"bytes"
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/bensyverson/contextfill/fill"
)

// Claude Code's settings: hooks.PostToolUse is a list of groups, each a
// matcher and the hooks it runs.
const (
	postToolUse = "PostToolUse"
	bashMatcher = "Bash"
	// hookTimeout is the seconds Claude Code waits for the hook.
	hookTimeout = 10
)

// group is one PostToolUse matcher group, the shape a new one is written in.
type group struct {
	Matcher string  `json:"matcher"`
	Hooks   []entry `json:"hooks"`
}

// entry is one hook in a group. Only Command is read from an existing one.
type entry struct {
	Type    string `json:"type"`
	Command string `json:"command"`
	Timeout int    `json:"timeout,omitempty"`
}

// settings is a parsed settings file: what is read is parsed, everything
// else is kept as written, in order.
type settings struct {
	top    object
	hooks  object
	groups []*groupView
}

// groupView is an existing group, parsed only as far as finding contextfill
// takes. obj is nil for a group that isn't an object; it is left alone.
type groupView struct {
	raw      json.RawMessage
	obj      object
	matcher  string
	entries  []json.RawMessage
	commands []string
	dirty    bool
}

// found is a contextfill entry: its group and its place in that group.
type found struct {
	group   *groupView
	index   int
	command string
}

func invalid(format string, args ...any) error {
	return fmt.Errorf("%w: %s", ErrInvalidSettings, fmt.Sprintf(format, args...))
}

// parseSettings reads a settings file; empty data is an empty file.
func parseSettings(data []byte) (*settings, error) {
	s := &settings{}
	if len(bytes.TrimSpace(data)) == 0 {
		return s, nil
	}
	top, err := parseObject(data)
	if err != nil {
		return nil, invalid("%v", err)
	}
	s.top = top
	raw, ok := top.get("hooks")
	if !ok {
		return s, nil
	}
	if s.hooks, err = parseObject(raw); err != nil {
		return nil, invalid("hooks is not an object")
	}
	raw, ok = s.hooks.get(postToolUse)
	if !ok {
		return s, nil
	}
	var groups []json.RawMessage
	if err := json.Unmarshal(raw, &groups); err != nil {
		return nil, invalid("hooks.%s is not a list", postToolUse)
	}
	for _, raw := range groups {
		s.groups = append(s.groups, parseGroup(raw))
	}
	return s, nil
}

func parseGroup(raw json.RawMessage) *groupView {
	g := &groupView{raw: raw}
	obj, err := parseObject(raw)
	if err != nil {
		return g
	}
	if m, ok := obj.get("matcher"); ok {
		json.Unmarshal(m, &g.matcher)
	}
	hooks, ok := obj.get("hooks")
	if !ok || json.Unmarshal(hooks, &g.entries) != nil {
		return g
	}
	g.obj = obj
	for _, e := range g.entries {
		var parsed entry
		json.Unmarshal(e, &parsed)
		g.commands = append(g.commands, parsed.Command)
	}
	return g
}

// contextfill lists every contextfill entry, in file order.
func (s *settings) contextfill() []found {
	var out []found
	for _, g := range s.groups {
		for i, c := range g.commands {
			if IsContextfill(c) {
				out = append(out, found{group: g, index: i, command: c})
			}
		}
	}
	return out
}

// drop removes the entries; a group left with no hooks goes too.
func (s *settings) drop(entries []found) {
	gone := map[*groupView]map[int]bool{}
	for _, f := range entries {
		if gone[f.group] == nil {
			gone[f.group] = map[int]bool{}
		}
		gone[f.group][f.index] = true
	}
	for g, indexes := range gone {
		var keep []json.RawMessage
		for i, e := range g.entries {
			if !indexes[i] {
				keep = append(keep, e)
			}
		}
		g.entries, g.dirty = keep, true
	}
}

// encode writes the settings back, every untouched value as it was.
func (s *settings) encode() ([]byte, error) {
	var groups []json.RawMessage
	for _, g := range s.groups {
		switch {
		case !g.dirty:
			groups = append(groups, g.raw)
		case len(g.entries) > 0:
			groups = append(groups, g.obj.set("hooks", encodeArray(g.entries)).encode())
		}
	}
	if len(groups) > 0 {
		s.hooks = s.hooks.set(postToolUse, encodeArray(groups))
	} else {
		s.hooks = s.hooks.without(postToolUse)
	}
	if len(s.hooks) > 0 {
		s.top = s.top.set("hooks", s.hooks.encode())
	} else {
		s.top = s.top.without("hooks")
	}
	return pretty(s.top)
}

// MergeClaude returns the settings with one PostToolUse hook on Bash running
// command, and what changed. An older contextfill entry is rewritten in
// place, keeping its other fields; any other is removed. When nothing needs
// to change it returns data as it was.
func MergeClaude(data []byte, command string) ([]byte, []Change, error) {
	s, err := parseSettings(data)
	if err != nil {
		return nil, nil, err
	}
	existing := s.contextfill()
	if len(existing) == 1 && existing[0].group.matcher == bashMatcher && existing[0].command == command {
		return data, []Change{{Kind: Unchanged, New: command}}, nil
	}

	var changes []Change
	keep := -1
	for i, f := range existing {
		if f.group.matcher == bashMatcher && f.group.obj != nil {
			keep = i
			break
		}
	}
	if keep >= 0 {
		f := existing[keep]
		obj, err := parseObject(f.group.entries[f.index])
		if err != nil {
			return nil, nil, invalid("a hook in hooks.%s is not an object", postToolUse)
		}
		f.group.entries[f.index] = obj.set("command", encodeValue(command)).encode()
		f.group.dirty = true
		if f.command != command {
			changes = append(changes, Change{Kind: Replaced, Old: f.command, New: command})
		}
		existing = append(existing[:keep:keep], existing[keep+1:]...)
	}
	if keep < 0 && len(existing) > 0 {
		changes = append(changes, Change{Kind: Replaced, Old: existing[0].command, New: command})
		existing[0].command = ""
	}
	for _, f := range existing {
		if f.command != "" {
			changes = append(changes, Change{Kind: Removed, Old: f.command})
		}
	}
	s.drop(existing)
	if keep < 0 {
		g := group{Matcher: bashMatcher, Hooks: []entry{{Type: "command", Command: command, Timeout: hookTimeout}}}
		s.groups = append(s.groups, &groupView{raw: encodeValue(g)})
		if len(changes) == 0 {
			changes = append(changes, Change{Kind: Added, New: command})
		}
	}
	out, err := s.encode()
	return out, changes, err
}

// RemoveClaude returns the settings without contextfill's hook, and what
// changed. With no contextfill entry it returns data as it was.
func RemoveClaude(data []byte) ([]byte, []Change, error) {
	s, err := parseSettings(data)
	if err != nil {
		return nil, nil, err
	}
	existing := s.contextfill()
	if len(existing) == 0 {
		return data, []Change{{Kind: Unchanged}}, nil
	}
	var changes []Change
	for _, f := range existing {
		changes = append(changes, Change{Kind: Removed, Old: f.command})
	}
	s.drop(existing)
	out, err := s.encode()
	return out, changes, err
}

// IsContextfill reports whether a hook command runs a contextfill binary,
// wherever it lives: its program, after any VAR=value assignments, is named
// contextfill.
func IsContextfill(command string) bool {
	rest := strings.TrimSpace(command)
	for rest != "" {
		var word string
		if q := rest[0]; q == '"' || q == '\'' {
			end := strings.IndexByte(rest[1:], q)
			if end < 0 {
				return false
			}
			word, rest = rest[1:end+1], rest[end+2:]
		} else {
			word, rest, _ = strings.Cut(rest, " ")
		}
		rest = strings.TrimSpace(rest)
		if isAssignment(word) {
			continue
		}
		return filepath.Base(word) == "contextfill"
	}
	return false
}

// isAssignment reports whether a shell word is NAME=value.
func isAssignment(word string) bool {
	name, _, ok := strings.Cut(word, "=")
	if !ok || name == "" {
		return false
	}
	for i, r := range name {
		if !(r == '_' || r >= 'A' && r <= 'Z' || r >= 'a' && r <= 'z' || i > 0 && r >= '0' && r <= '9') {
			return false
		}
	}
	return true
}

// stamp sets the harness and path on each change.
func stamp(changes []Change, h fill.Harness, path string) []Change {
	for i := range changes {
		changes[i].Harness, changes[i].Path = h, path
	}
	return changes
}
