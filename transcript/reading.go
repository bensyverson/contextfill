package transcript

import "strings"

// Reading is one measurement of a context window.
type Reading struct {
	// Model is the model the turn asked for, e.g. "claude-opus-5-5[1m]".
	Model string
	// Used is the number of tokens the context held at the latest turn.
	Used int
	// Window is the model's context window, in tokens.
	Window int
}

// Fill is the fraction of the window in use, from 0 to 1. It is 0 for a
// reading with no window.
func (r Reading) Fill() float64 {
	if r.Window <= 0 {
		return 0
	}
	return float64(r.Used) / float64(r.Window)
}

const (
	defaultWindow = 200_000
	millionWindow = 1_000_000
	millionSuffix = "[1m]"
)

// WindowFor returns the context window of a requested model name: a million
// tokens for one Claude Code requests with the "[1m]" suffix, otherwise the
// standard 200k.
func WindowFor(model string) int {
	if strings.HasSuffix(model, millionSuffix) {
		return millionWindow
	}
	return defaultWindow
}
