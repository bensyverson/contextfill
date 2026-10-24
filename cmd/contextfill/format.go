package main

import (
	"fmt"
	"math"
	"strconv"

	"github.com/bensyverson/contextfill/transcript"
)

// percent renders a fill as a whole percentage: "23%".
func percent(r transcript.Reading) string {
	return fmt.Sprintf("%d%%", int(math.Round(r.Fill()*100)))
}

// tokens renders a token count compactly: 950, 20k, 234.5k, 1M, 1.2M.
func tokens(n int) string {
	switch {
	case n < 1000:
		return strconv.Itoa(n)
	case n < 1_000_000:
		return trimmed(float64(n)/1000) + "k"
	default:
		return trimmed(float64(n)/1_000_000) + "M"
	}
}

// trimmed renders one decimal place, dropping a trailing ".0".
func trimmed(f float64) string {
	return strconv.FormatFloat(math.Round(f*10)/10, 'f', -1, 64)
}

// fillOf renders "23% of 1M".
func fillOf(r transcript.Reading) string {
	return percent(r) + " of " + tokens(r.Window)
}

// short is the first eight characters of a session id, enough to tell
// sessions apart in a listing.
func short(id string) string {
	if len(id) > 8 {
		return id[:8]
	}
	return id
}
