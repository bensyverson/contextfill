package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"path/filepath"
	"text/tabwriter"

	"github.com/bensyverson/contextfill/hook"
	"github.com/bensyverson/contextfill/transcript"
)

// sessionEnv is the variable Claude Code sets to the running session's id in
// every shell it starts.
const sessionEnv = "CLAUDE_CODE_SESSION_ID"

// report is the --json shape.
type report struct {
	Session string        `json:"session"`
	Model   string        `json:"model"`
	Used    int           `json:"used"`
	Window  int           `json:"window"`
	Fill    float64       `json:"fill"`
	Agents  []agentReport `json:"agents"`
}

type agentReport struct {
	ID          string  `json:"id"`
	Description string  `json:"description"`
	Model       string  `json:"model"`
	Finished    bool    `json:"finished"`
	Used        int     `json:"used"`
	Window      int     `json:"window"`
	Fill        float64 `json:"fill"`
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
	if o.hook {
		out, err := hook.Respond(stdin, hook.Limits{Max: o.maxPercent, MainMax: o.mainMaxPercent})
		if err != nil {
			// Not exit 2, which would block the tool call the hook follows.
			fmt.Fprintf(stderr, "contextfill hook: %v\n", err)
			return exitTarget
		}
		stdout.Write(out)
		return exitOK
	}

	projects, err := projectsDir(env)
	if err != nil {
		fmt.Fprintf(stderr, "contextfill: %v\n", err)
		return exitEnvironment
	}
	id := o.sessionID
	if id == "" {
		id = env(sessionEnv)
	}
	if id == "" {
		fmt.Fprintf(stderr, "contextfill: no session to read: %s is not set.\n"+
			"Run it from a shell inside Claude Code, or name one: contextfill <session-id>.\n"+
			"Transcripts are %s/<project>/<session-id>.jsonl.\n", sessionEnv, projects)
		return exitEnvironment
	}

	path, err := transcript.Find(projects, id)
	if err != nil {
		fmt.Fprintf(stderr, "contextfill: %v\nList the sessions there with: ls %s/*/\n", err, projects)
		return exitTarget
	}
	reading, err := transcript.Latest(path)
	if err != nil {
		fmt.Fprintf(stderr, "contextfill: %v\n", err)
		return exitTarget
	}
	var agents []transcript.Agent
	if o.agents || o.json {
		if agents, err = transcript.Agents(path); err != nil {
			fmt.Fprintf(stderr, "contextfill: %v\n", err)
			return exitTarget
		}
	}

	over := o.maxPercent > 0 && reading.Fill()*100 > o.maxPercent
	if o.json {
		writeJSON(stdout, id, reading, agents)
	} else {
		writeText(stdout, id, reading, agents, o)
		if over {
			fmt.Fprintf(stdout, "over the %s%% limit: write the handoff before starting more work\n", trimmed(o.maxPercent))
		}
	}
	if over {
		return exitOverMax
	}
	return exitOK
}

// projectsDir is where Claude Code keeps transcripts: $CLAUDE_CONFIG_DIR, or
// ~/.claude, then projects.
func projectsDir(env func(string) string) (string, error) {
	if config := env("CLAUDE_CONFIG_DIR"); config != "" {
		return filepath.Join(config, "projects"), nil
	}
	home := env("HOME")
	if home == "" {
		return "", errors.New("neither CLAUDE_CONFIG_DIR nor HOME is set, so there is nowhere to look for transcripts")
	}
	return filepath.Join(home, ".claude", "projects"), nil
}

func writeText(w io.Writer, id string, r transcript.Reading, agents []transcript.Agent, o options) {
	fmt.Fprintf(w, "%s · %s tokens · %s · session %s\n", r.Summary(), transcript.Tokens(r.Used), r.Model, short(id))
	if !o.agents {
		return
	}
	if len(agents) == 0 {
		fmt.Fprintln(w, "  no subagents")
		return
	}
	tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
	for _, a := range agents {
		fill := "no reading yet"
		if a.Reading.Window > 0 {
			fill = a.Reading.Summary()
		}
		status := "running"
		if a.Finished {
			status = "finished"
		}
		fmt.Fprintf(tw, "  %s\t%s\t%s\t%s\t%s\n", a.ID, fill, status, a.Model, a.Description)
	}
	tw.Flush()
}

func writeJSON(w io.Writer, id string, r transcript.Reading, agents []transcript.Agent) {
	out := report{Session: id, Model: r.Model, Used: r.Used, Window: r.Window, Fill: r.Fill(), Agents: []agentReport{}}
	for _, a := range agents {
		out.Agents = append(out.Agents, agentReport{
			ID: a.ID, Description: a.Description, Model: a.Model, Finished: a.Finished,
			Used: a.Reading.Used, Window: a.Reading.Window, Fill: a.Reading.Fill(),
		})
	}
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	enc.Encode(out)
}

const helpText = `contextfill — how full is this Claude Code session's context window?

Reads the session's transcript under ~/.claude/projects (or $CLAUDE_CONFIG_DIR)
and reports the tokens its latest turn carried, out of the model's window.
With no session id it reads the current one, from $CLAUDE_CODE_SESSION_ID.

Usage:
  contextfill [session-id] [--agents] [--max <percent>] [--json]

  contextfill                  23% of 1M · 234.5k tokens · claude-opus-5-5[1m] · session 5f3a9c21
  contextfill --agents         ...plus one row per subagent: id, fill, running or finished, model, task
  contextfill --max 40         exits 1 when the fill is over 40%, so a script can branch on it
  contextfill --json           the same reading, and every subagent, as JSON

As a Claude Code PostToolUse hook on Bash, it tells the model its fill after
every git commit (a subagent gets its own), and stays silent otherwise:

  contextfill hook --max 40    reads the hook payload on stdin; past 40% it says
                               to finish what's in flight and write the handoff
  contextfill hook --max 40 --main-max 25
                               the same, but the main thread's limit is 25%
                               (--main-max falls back to --max)

Exit codes: 0 ok · 1 over --max · 2 usage error · 4 transcript missing or
unreadable · 5 no session to read.
`
