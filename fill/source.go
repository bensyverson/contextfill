package fill

import "errors"

var (
	// ErrNoSession means the harness has no session with that id.
	ErrNoSession = errors.New("no such session")
	// ErrNoUsage means the session holds no assistant turn with token usage.
	ErrNoUsage = errors.New("no assistant turn with token usage yet")
)

// Source reads one harness's sessions.
type Source interface {
	// Latest returns the session's latest reading.
	Latest(sessionID string) (Reading, error)
	// Agents lists the session's subagents.
	Agents(sessionID string) ([]Agent, error)
}

// Agent is one subagent of a session.
type Agent struct {
	// ID is the agent's id within its harness.
	ID string
	// Description is the short task the agent was launched with.
	Description string
	// Model is the model it runs, as its harness names it, e.g. "opus".
	Model string
	// Finished reports whether the agent has completed.
	Finished bool
	// Reading is its latest context reading; zero if it hasn't answered yet.
	Reading Reading
}
