// Package fill holds what every harness's reader shares: a reading of a
// context window, a subagent, the Source interface a reader implements, and
// which harness a shell is running in.
package fill

import (
	"math"
	"strconv"
)

// Reading is one measurement of a context window.
type Reading struct {
	// Model is the model the turn asked for, e.g. "claude-opus-5-5[1m]".
	Model string
	// Used is the number of tokens the context held at the latest turn.
	Used int
	// Window is the model's context window, in tokens; 0 when the harness
	// doesn't know it.
	Window int
}

// HasWindow reports whether the reading knows its model's window.
func (r Reading) HasWindow() bool { return r.Window > 0 }

// Fill is the fraction of the window in use, from 0 to 1. It is 0 for a
// reading with no window.
func (r Reading) Fill() float64 {
	if !r.HasWindow() {
		return 0
	}
	return float64(r.Used) / float64(r.Window)
}

// Percent is the fill as a whole percentage, rounded.
func (r Reading) Percent() int {
	return int(math.Round(r.Fill() * 100))
}

// Summary renders the fill against the window, "23% of 1M", or "window
// unknown".
func (r Reading) Summary() string {
	if !r.HasWindow() {
		return "window unknown"
	}
	return strconv.Itoa(r.Percent()) + "% of " + Tokens(r.Window)
}

// Tokens renders a token count compactly: 950, 20k, 234.5k, 1M, 1.2M.
func Tokens(n int) string {
	switch {
	case n < 1000:
		return strconv.Itoa(n)
	case n < 1_000_000:
		return oneDecimal(float64(n)/1000) + "k"
	default:
		return oneDecimal(float64(n)/1_000_000) + "M"
	}
}

// oneDecimal renders one decimal place, dropping a trailing ".0".
func oneDecimal(f float64) string {
	return strconv.FormatFloat(math.Round(f*10)/10, 'f', -1, 64)
}
