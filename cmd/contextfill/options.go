package main

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/bensyverson/contextfill/fill"
)

// options is a parsed command line.
type options struct {
	help    bool
	version bool
	// hook means answer a Claude Code PostToolUse hook from stdin.
	hook bool
	// action is hook's install or uninstall verb; "" means answer a payload.
	action    action
	agents    bool
	json      bool
	sessionID string
	// maxPercent is the --max limit; 0 means no limit.
	maxPercent float64
	// mainMaxPercent is the hook's --main-max limit for the main thread; 0
	// means it falls back to maxPercent.
	mainMaxPercent float64
	// harness is the --harness override; "" means detect it.
	harness fill.Harness
}

// action is a verb under hook that sets the hook up rather than answering it.
type action string

const (
	actionInstall   action = "install"
	actionUninstall action = "uninstall"
)

// parse reads the command line. Flags and the session id may come in any
// order.
func parse(args []string) (options, error) {
	var o options
	for i := 0; i < len(args); i++ {
		arg := args[i]
		switch {
		case arg == "-h" || arg == "--help":
			o.help = true
		case arg == "--version":
			o.version = true
		case arg == "--agents":
			o.agents = true
		case arg == "--json":
			o.json = true
		case arg == "--max" || strings.HasPrefix(arg, "--max="):
			percent, err := percentFlag("--max", args, &i)
			if err != nil {
				return o, err
			}
			o.maxPercent = percent
		case arg == "--main-max" || strings.HasPrefix(arg, "--main-max="):
			percent, err := percentFlag("--main-max", args, &i)
			if err != nil {
				return o, err
			}
			o.mainMaxPercent = percent
		case arg == "--harness" || strings.HasPrefix(arg, "--harness="):
			value, ok := strings.CutPrefix(arg, "--harness=")
			if !ok {
				if i+1 >= len(args) {
					return o, fmt.Errorf("--harness needs a name: %s or %s", fill.ClaudeCode, fill.OpenCode)
				}
				i++
				value = args[i]
			}
			h, err := fill.ParseHarness(value)
			if err != nil {
				return o, err
			}
			o.harness = h
		case arg == "hook" && !o.hook && o.sessionID == "":
			o.hook = true
		case o.hook && o.action == "" && (action(arg) == actionInstall || action(arg) == actionUninstall):
			o.action = action(arg)
		case strings.HasPrefix(arg, "-"):
			return o, fmt.Errorf("unknown flag %s", arg)
		case o.sessionID == "" && !o.hook:
			o.sessionID = arg
		case o.hook && o.action != "":
			return o, fmt.Errorf("hook %s takes no arguments, only flags; got %s", o.action, arg)
		case o.hook:
			return o, fmt.Errorf("hook reads its session from the payload on stdin, not %s; to set the hook up, run contextfill hook install", arg)
		default:
			return o, fmt.Errorf("one session id at a time; got %s and %s", o.sessionID, arg)
		}
	}
	if o.action != "" && (o.agents || o.json) {
		return o, fmt.Errorf("hook %s takes --harness, --max and --main-max, not --agents or --json", o.action)
	}
	if o.mainMaxPercent > 0 && !o.hook {
		return o, fmt.Errorf("--main-max applies only to contextfill hook; a bare read is always the main thread, so use --max")
	}
	return o, nil
}

// percentFlag reads the percentage of the flag at args[*i], given as
// "name=value" or as the next argument, which it consumes.
func percentFlag(name string, args []string, i *int) (float64, error) {
	value, ok := strings.CutPrefix(args[*i], name+"=")
	if !ok {
		if *i+1 >= len(args) {
			return 0, fmt.Errorf("%s needs a percentage, e.g. %s 40", name, name)
		}
		*i++
		value = args[*i]
	}
	percent, err := strconv.ParseFloat(strings.TrimSuffix(value, "%"), 64)
	if err != nil || percent <= 0 || percent > 100 {
		return 0, fmt.Errorf("%s takes a percentage from 1 to 100, not %q", name, value)
	}
	return percent, nil
}
