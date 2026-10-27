package transcript

import (
	"os"
	"strconv"
	"strings"
)

const (
	defaultWindow = 200_000
	millionWindow = 1_000_000
	millionSuffix = "[1m]"

	// disable1MEnv is Claude Code's switch that holds every model to 200k.
	disable1MEnv = "CLAUDE_CODE_DISABLE_1M_CONTEXT"
)

// version is a model's major and minor version, as in "claude-opus-4-7".
type version struct{ major, minor int }

func (v version) atLeast(o version) bool {
	return v.major > o.major || v.major == o.major && v.minor >= o.minor
}

// nativeMillion is, per family, the first version Claude Code runs with a 1M
// window by default, with no "[1m]" suffix: see "Extended context" in
// https://code.claude.com/docs/en/model-config.
var nativeMillion = map[string]version{
	"fable":  {5, 0},
	"sonnet": {5, 0},
	"haiku":  {5, 5},
	"opus":   {4, 7},
}

// WindowFor returns the context window Claude Code gives a requested model
// name. Opus 4.7 and later, Sonnet 5 and later, Haiku 5.5 and the Fable
// models run at a million tokens by default; any other model does only when
// requested with the "[1m]" suffix, and otherwise gets the standard 200k.
// Claude Code's CLAUDE_CODE_DISABLE_1M_CONTEXT=1 holds every model to 200k.
func WindowFor(model string) int {
	if os.Getenv(disable1MEnv) == "1" {
		return defaultWindow
	}
	if strings.HasSuffix(model, millionSuffix) {
		return millionWindow
	}
	family, v, ok := parseModel(model)
	if first, known := nativeMillion[family]; ok && known && v.atLeast(first) {
		return millionWindow
	}
	return defaultWindow
}

// parseModel reads the family and version from a model ID such as
// "claude-opus-4-5-20251101" or "us.anthropic.claude-opus-5-5-v1:0". A part
// as long as a date isn't a minor version.
func parseModel(model string) (family string, v version, ok bool) {
	_, id, found := strings.Cut(model, "claude-")
	if !found {
		return "", version{}, false
	}
	parts := strings.Split(id, "-")
	if len(parts) < 2 {
		return "", version{}, false
	}
	major, err := strconv.Atoi(parts[1])
	if err != nil {
		return "", version{}, false
	}
	v.major = major
	if len(parts) > 2 && len(parts[2]) <= 2 {
		if minor, err := strconv.Atoi(parts[2]); err == nil {
			v.minor = minor
		}
	}
	return parts[0], v, true
}
