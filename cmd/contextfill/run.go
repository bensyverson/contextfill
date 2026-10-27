package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"text/tabwriter"

	"github.com/bensyverson/contextfill/fill"
	"github.com/bensyverson/contextfill/hook"
	"github.com/bensyverson/contextfill/opencode"
)

// report is the --json shape. Fill is null when the window is unknown.
type report struct {
	Harness fill.Harness  `json:"harness"`
	Session string        `json:"session"`
	Model   string        `json:"model"`
	Used    int           `json:"used"`
	Window  int           `json:"window"`
	Fill    *float64      `json:"fill"`
	Agents  []agentReport `json:"agents"`
}

type agentReport struct {
	ID          string   `json:"id"`
	Description string   `json:"description"`
	Model       string   `json:"model"`
	Finished    bool     `json:"finished"`
	Used        int      `json:"used"`
	Window      int      `json:"window"`
	Fill        *float64 `json:"fill"`
}

// fillOf is a reading's fill for --json: null when its window is unknown.
func fillOf(r fill.Reading) *float64 {
	if !r.HasWindow() {
		return nil
	}
	f := r.Fill()
	return &f
}

// run is the whole command, with its environment passed in so tests can drive
// it. It returns the exit code.
func run(args []string, env func(string) string, stdin io.Reader, stdout, stderr io.Writer) int {
	o, err := parse(args)
	if err != nil {
		fmt.Fprintf(stderr, "contextfill: %v\nRun contextfill --help for usage.\n", err)
		return exitUsage
	}
	if o.help {
		fmt.Fprint(stdout, helpText)
		return exitOK
	}
	if o.version {
		fmt.Fprintf(stdout, "contextfill %s\n", version)
		return exitOK
	}
	if o.hook {
		return runHook(o, env, stdin, stdout, stderr)
	}

	harness := o.harness
	if harness == "" {
		harness = fill.Detect(env)
	}
	t, err := open(harness, env)
	if err != nil {
		fmt.Fprintf(stderr, "contextfill: %v\n", err)
		return exitEnvironment
	}
	id := o.sessionID
	if id == "" {
		id = env(harness.SessionEnv())
	}
	if id == "" {
		fmt.Fprintf(stderr, "contextfill: no session to read: %s is not set.\n"+
			"Run it from a shell inside %s, or name one: contextfill <session-id>.\n%s\n",
			harness.SessionEnv(), harnessName(harness), t.where)
		return exitEnvironment
	}

	reading, err := t.source.Latest(id)
	if err != nil {
		return readFailed(stderr, t, err)
	}
	var agents []fill.Agent
	if o.agents || o.json {
		if agents, err = t.source.Agents(id); err != nil {
			return readFailed(stderr, t, err)
		}
	}

	if o.json {
		writeJSON(stdout, harness, id, reading, agents)
	} else {
		writeText(stdout, display(harness, id), reading, agents, o)
	}
	if o.maxPercent > 0 && !reading.HasWindow() {
		fmt.Fprintf(stderr, "contextfill: can't judge --max: %s's window is unknown to %s.\n", reading.Model, harnessName(harness))
		return exitTarget
	}
	if o.maxPercent > 0 && reading.Fill()*100 > o.maxPercent {
		if !o.json {
			fmt.Fprintf(stdout, "over %s%%\n", trimmed(o.maxPercent))
		}
		return exitOverMax
	}
	return exitOK
}

// runHook answers a commit hook: Claude Code's PostToolUse payload by
// default, or the OpenCode plugin's with --harness opencode. Its errors never
// exit 2, which would make Claude Code block the tool call the hook follows.
func runHook(o options, env func(string) string, stdin io.Reader, stdout, stderr io.Writer) int {
	limits := hook.Limits{Max: o.maxPercent, MainMax: o.mainMaxPercent}
	if o.harness != fill.OpenCode {
		out, err := hook.Respond(stdin, limits)
		if err != nil {
			fmt.Fprintf(stderr, "contextfill hook: %v\n", err)
			return exitTarget
		}
		stdout.Write(out)
		return exitOK
	}
	client, _, err := openCode(env)
	if err != nil {
		fmt.Fprintf(stderr, "contextfill hook: %v\n", err)
		return exitEnvironment
	}
	text, err := hook.RespondOpenCode(stdin, limits, client)
	if err != nil {
		fmt.Fprintf(stderr, "contextfill hook: %v\n", err)
		return exitTarget
	}
	if text != "" {
		fmt.Fprintln(stdout, text)
	}
	return exitOK
}

// readFailed reports an error reading the session and returns its exit code:
// a harness that isn't running is the environment, anything else the target.
func readFailed(stderr io.Writer, t target, err error) int {
	fmt.Fprintf(stderr, "contextfill: %v\n", err)
	switch {
	case errors.Is(err, opencode.ErrNoServer):
		return exitEnvironment
	case errors.Is(err, fill.ErrNoSession):
		fmt.Fprintf(stderr, "List the sessions with: %s\n", t.list)
	}
	return exitTarget
}

func writeText(w io.Writer, id string, r fill.Reading, agents []fill.Agent, o options) {
	fmt.Fprintf(w, "%s · %s tokens · %s · session %s\n", r.Summary(), fill.Tokens(r.Used), r.Model, id)
	if !o.agents {
		return
	}
	if len(agents) == 0 {
		fmt.Fprintln(w, "  no subagents")
		return
	}
	tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
	for _, a := range agents {
		reading := "no reading yet"
		if a.Reading.Used > 0 {
			reading = a.Reading.Summary()
		}
		status := "running"
		if a.Finished {
			status = "finished"
		}
		fmt.Fprintf(tw, "  %s\t%s\t%s\t%s\t%s\n", a.ID, reading, status, a.Model, a.Description)
	}
	tw.Flush()
}

func writeJSON(w io.Writer, h fill.Harness, id string, r fill.Reading, agents []fill.Agent) {
	out := report{Harness: h, Session: id, Model: r.Model, Used: r.Used, Window: r.Window, Fill: fillOf(r), Agents: []agentReport{}}
	for _, a := range agents {
		out.Agents = append(out.Agents, agentReport{
			ID: a.ID, Description: a.Description, Model: a.Model, Finished: a.Finished,
			Used: a.Reading.Used, Window: a.Reading.Window, Fill: fillOf(a.Reading),
		})
	}
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	enc.Encode(out)
}

const helpText = `contextfill — how full is this agent session's context window?

Reports the tokens a Claude Code or OpenCode session's latest turn carried,
out of its model's window. With no session id it reads the current one: inside
OpenCode (OPENCODE=1) from $OPENCODE_SESSION_ID, otherwise from Claude Code's
$CLAUDE_CODE_SESSION_ID.

Usage:
  contextfill [session-id] [--agents] [--max <percent>] [--json] [--harness <name>]

  contextfill                  23% of 1M · 234.5k tokens · claude-opus-5-5 · session 5f3a9c21
  contextfill --agents         ...plus one row per subagent: id, fill, running or finished, model, task
  contextfill --max 40         exits 1 when the fill is over 40%, so a script can branch on it
  contextfill --json           the same reading, and every subagent, as JSON
  contextfill --harness opencode ses_…
                               reads that harness's session, whatever shell this is
  contextfill --version        prints contextfill's own version

Claude Code: reads the transcript under ~/.claude/projects (or
$CLAUDE_CONFIG_DIR). The window is the one Claude Code gives the model: 1M for
Opus 4.7 and later, Sonnet 5 and later, Haiku 5.5, the Fable models, or any
model requested with [1m]; otherwise 200k, as is every model when
CLAUDE_CODE_DISABLE_1M_CONTEXT=1. OpenCode: asks its running server, found through
~/.local/state/opencode/service.json (or $XDG_STATE_HOME), the way OpenCode's
own clients do. A model OpenCode doesn't know the window of reads "window
unknown", and --max can't judge it.

As a Claude Code PostToolUse hook on Bash, or through its OpenCode plugin, it
tells the model its fill after every git commit (a subagent gets its own), and
stays silent otherwise:

  contextfill hook --max 40    reads the hook payload on stdin; past 40% it adds
                               that the fill is over 40%
  contextfill hook --max 40 --main-max 25
                               the same, but the main thread's limit is 25%
                               (--main-max falls back to --max)
  contextfill hook --harness opencode --max 40 --main-max 25
                               answers OpenCode's plugin (opencode/plugin) in
                               plain text, for the shell tool's commits

Exit codes: 0 ok · 1 over --max · 2 usage error · 4 session missing or
unreadable, or --max against an unknown window · 5 no session to read, or
OpenCode's server isn't running.
`
