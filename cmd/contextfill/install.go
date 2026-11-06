package main

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/bensyverson/contextfill/fill"
	"github.com/bensyverson/contextfill/install"
)

// The limits hook install writes when none are given: the recommended ones.
const (
	defaultMax     = 40
	defaultMainMax = 33
)

// executable is the running binary's path; tests point it at a fake keg.
var executable = os.Executable

// runInstall sets contextfill's hook up in, or takes it out of, the harness
// named by --harness, or each one present. It prints one line per change.
func runInstall(o options, env func(string) string, stdout, stderr io.Writer) int {
	exe, err := executable()
	if err != nil {
		fmt.Fprintf(stderr, "contextfill: can't find this binary's path: %v\n", err)
		return exitEnvironment
	}
	hook := install.Hook{Program: install.StablePath(exe), Max: o.maxPercent, MainMax: o.mainMaxPercent}
	if hook.Max == 0 {
		hook.Max = defaultMax
	}
	if hook.MainMax == 0 {
		hook.MainMax = defaultMainMax
	}

	claude, err := install.ClaudePlace(env)
	if err != nil {
		fmt.Fprintf(stderr, "contextfill: %v\n", err)
		return exitEnvironment
	}
	openCode, err := install.OpenCodePlace(env)
	if err != nil {
		fmt.Fprintf(stderr, "contextfill: %v\n", err)
		return exitEnvironment
	}
	harnesses := []fill.Harness{o.harness}
	if o.harness == "" {
		harnesses = nil
		if claude.Present {
			harnesses = append(harnesses, fill.ClaudeCode)
		}
		if openCode.Present {
			harnesses = append(harnesses, fill.OpenCode)
		}
	}
	if len(harnesses) == 0 {
		fmt.Fprintf(stderr, "contextfill: found neither Claude Code's settings folder (%s) nor OpenCode's (%s).\n"+
			"Name the app: contextfill hook %s --harness claude-code, or --harness opencode.\n",
			filepath.Dir(claude.Path), filepath.Dir(filepath.Dir(openCode.Path)), o.action)
		return exitEnvironment
	}

	code := exitOK
	for _, h := range harnesses {
		var changes []install.Change
		switch {
		case h == fill.OpenCode && o.action == actionInstall:
			changes, err = install.InstallOpenCode(openCode.Path, exe)
		case h == fill.OpenCode:
			changes, err = install.UnlinkPlugin(openCode.Path)
		case o.action == actionInstall:
			changes, err = install.InstallClaude(claude.Path, hook.Command())
		default:
			changes, err = install.UninstallClaude(claude.Path)
		}
		for _, c := range changes {
			fmt.Fprintln(stdout, c)
		}
		if err != nil {
			fmt.Fprintf(stderr, "contextfill: %v\n", err)
			if errors.Is(err, install.ErrInvalidSettings) {
				fmt.Fprintf(stderr, "Nothing was written. Fix the file, or move it aside, then run contextfill hook %s again.\n", o.action)
			}
			code = exitTarget
		}
	}
	return code
}

const hookInstallHelp = `contextfill hook install — set up the commit hook in Claude Code or OpenCode

Usage:
  contextfill hook install [--harness claude-code|opencode] [--max <percent>] [--main-max <percent>]
  contextfill hook uninstall [--harness claude-code|opencode]

  contextfill hook install --harness claude-code
      adds a PostToolUse hook on Bash to ~/.claude/settings.json (or
      $CLAUDE_CONFIG_DIR/settings.json) that runs this contextfill by its full
      path: hook --max 40 --main-max 33. Every other setting is kept, in
      order. An older contextfill entry, wherever its binary lived, is
      replaced; run it twice and the second run changes nothing. A settings
      file that isn't valid JSON is refused and left as it was (exit 4).
  contextfill hook install --harness opencode
      links the OpenCode plugin installed with contextfill into
      ~/.config/opencode/plugins (or $XDG_CONFIG_HOME/opencode/plugins). A
      copy you made there is kept. A contextfill installed without the plugin
      says so and exits 0.

With no --harness it sets up each app whose folder exists (~/.claude,
~/.config/opencode), and exits 5 if neither does. --max and --main-max default
to 40 and 33. It prints one line per change, or says nothing changed.
uninstall takes out only contextfill's hook entry and plugin link.

Under Homebrew it names /opt/homebrew/bin/contextfill (the prefix's bin link)
rather than the versioned Cellar path, which the next upgrade deletes.

Exit codes: 0 done, or nothing to do · 2 usage error · 4 a file couldn't be
read or written, or the settings file isn't valid JSON · 5 neither app found.
`
