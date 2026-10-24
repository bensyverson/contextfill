package transcript

import (
	"math"
	"strconv"
)

// Percent is the fill as a whole percentage, rounded.
func (r Reading) Percent() int {
	return int(math.Round(r.Fill() * 100))
}

// Summary renders the fill against the window: "23% of 1M".
func (r Reading) Summary() string {
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
