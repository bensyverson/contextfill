package main

import (
	"fmt"
	"strconv"
	"strings"
)

// options is a parsed command line.
type options struct {
	help bool
	// hook means answer a Claude Code PostToolUse hook from stdin.
	hook      bool
	agents    bool
	json      bool
	sessionID string
	// maxPercent is the --max limit; 0 means no limit.
	maxPercent float64
}

// parse reads the command line. Flags and the session id may come in any
// order.
func parse(args []string) (options, error) {
	var o options
	for i := 0; i < len(args); i++ {
		arg := args[i]
		switch {
		case arg == "-h" || arg == "--help":
			o.help = true
		case arg == "--agents":
			o.agents = true
		case arg == "--json":
			o.json = true
		case arg == "--max" || strings.HasPrefix(arg, "--max="):
			value, ok := strings.CutPrefix(arg, "--max=")
			if !ok {
				if i+1 >= len(args) {
					return o, fmt.Errorf("--max needs a percentage, e.g. --max 40")
				}
				i++
				value = args[i]
			}
			percent, err := strconv.ParseFloat(strings.TrimSuffix(value, "%"), 64)
			if err != nil || percent <= 0 || percent > 100 {
				return o, fmt.Errorf("--max takes a percentage from 1 to 100, not %q", value)
			}
			o.maxPercent = percent
		case arg == "hook" && !o.hook && o.sessionID == "":
			o.hook = true
		case strings.HasPrefix(arg, "-"):
			return o, fmt.Errorf("unknown flag %s", arg)
		case o.sessionID == "" && !o.hook:
			o.sessionID = arg
		case o.hook:
			return o, fmt.Errorf("hook reads its session from the payload on stdin, not %s", arg)
		default:
			return o, fmt.Errorf("one session id at a time; got %s and %s", o.sessionID, arg)
		}
	}
	return o, nil
}
