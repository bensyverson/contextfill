package main

import (
	"math"
	"strconv"
)

// trimmed renders one decimal place, dropping a trailing ".0".
func trimmed(f float64) string {
	return strconv.FormatFloat(math.Round(f*10)/10, 'f', -1, 64)
}

// short is the first eight characters of a session id, enough to tell
// sessions apart in a listing.
func short(id string) string {
	if len(id) > 8 {
		return id[:8]
	}
	return id
}
