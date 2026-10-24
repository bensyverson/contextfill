package transcript

import "strings"

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
