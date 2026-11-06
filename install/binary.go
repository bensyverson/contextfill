package install

import (
	"path/filepath"
	"strconv"
	"strings"
)

// formula is contextfill's Homebrew formula, and its keg's name.
const formula = "contextfill"

// Hook is the command a harness runs after each tool call.
type Hook struct {
	// Program is the contextfill binary's full path.
	Program string
	// Max and MainMax are the --max and --main-max limits, in percent.
	Max, MainMax float64
}

// Command is the hook as a Claude Code command line.
func (h Hook) Command() string {
	return quote(h.Program) + " hook --max " + percent(h.Max) + " --main-max " + percent(h.MainMax)
}

func percent(f float64) string { return strconv.FormatFloat(f, 'f', -1, 64) }

// quote quotes a path that the shell would otherwise split.
func quote(path string) string {
	if strings.ContainsAny(path, " \t'\"$`\\") {
		return "'" + strings.ReplaceAll(path, "'", `'\''`) + "'"
	}
	return path
}

// StablePath is the path a hook should name for the contextfill at
// executable. Homebrew runs a binary from its versioned Cellar keg, which the
// next upgrade deletes, so for a Cellar binary it returns the prefix's bin
// link, or failing that the opt link, when that link resolves to this very
// binary. Anything else is returned as given.
func StablePath(executable string) string {
	real := resolve(executable)
	prefix, _, ok := cellarKeg(real)
	if !ok {
		return executable
	}
	for _, link := range []string{
		filepath.Join(prefix, "bin", formula),
		filepath.Join(prefix, "opt", formula, "bin", formula),
	} {
		if resolve(link) == real {
			return link
		}
	}
	return real
}

// resolve follows every symlink in path; a path that can't be resolved is
// returned as it is.
func resolve(path string) string {
	if real, err := filepath.EvalSymlinks(path); err == nil {
		return real
	}
	return path
}

// cellarKeg reports whether binary is <prefix>/Cellar/contextfill/<version>/bin/contextfill,
// and returns the prefix and the keg.
func cellarKeg(binary string) (prefix, keg string, ok bool) {
	bin := filepath.Dir(binary)
	keg = filepath.Dir(bin)
	rack := filepath.Dir(keg)
	cellar := filepath.Dir(rack)
	if filepath.Base(binary) != formula || filepath.Base(bin) != "bin" || filepath.Base(rack) != formula || filepath.Base(cellar) != "Cellar" {
		return "", "", false
	}
	return filepath.Dir(cellar), keg, true
}
