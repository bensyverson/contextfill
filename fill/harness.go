package fill

import "fmt"

// Harness is the agent harness a session runs in.
type Harness string

const (
	ClaudeCode Harness = "claude"
	OpenCode   Harness = "opencode"
)

// Detect returns the harness whose shell this is. OpenCode wins when both
// are marked: an OpenCode started from a Claude Code shell inherits Claude
// Code's variables, so OpenCode's marker is the innermost. With neither, it
// is Claude Code, whose missing session variable then explains itself.
func Detect(env func(string) string) Harness {
	if env("OPENCODE") == "1" {
		return OpenCode
	}
	return ClaudeCode
}

// ParseHarness reads a --harness value.
func ParseHarness(name string) (Harness, error) {
	switch h := Harness(name); h {
	case ClaudeCode, OpenCode:
		return h, nil
	}
	return "", fmt.Errorf("--harness is %s or %s, not %q", ClaudeCode, OpenCode, name)
}

// SessionEnv is the variable the harness sets to the running session's id in
// every shell it starts.
func (h Harness) SessionEnv() string {
	if h == OpenCode {
		return "OPENCODE_SESSION_ID"
	}
	return "CLAUDE_CODE_SESSION_ID"
}
