package transcript

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// Agent is one subagent of a session.
type Agent struct {
	// ID is the agent's id, from its transcript's file name (agent-<id>.jsonl).
	ID string
	// Description is the short task description the agent was launched with.
	Description string
	// Model is the model alias it was launched with, e.g. "opus".
	Model string
	// Finished reports whether the agent has completed.
	Finished bool
	// Reading is its latest context reading; zero if it hasn't answered yet.
	Reading Reading
}

// meta is the part of an agent's .meta.json this package reads.
type meta struct {
	Description string `json:"description"`
	Model       string `json:"model"`
	Finished    bool   `json:"finished"`
}

// Agents lists the subagents of the session whose transcript is at
// sessionPath, in id order. They live beside it, in
// <session id>/subagents/agent-<id>.jsonl with an agent-<id>.meta.json.
func Agents(sessionPath string) ([]Agent, error) {
	dir := subagentsDir(sessionPath)
	paths, err := filepath.Glob(filepath.Join(dir, "agent-*.jsonl"))
	if err != nil {
		return nil, fmt.Errorf("searching %s: %w", dir, err)
	}
	sort.Strings(paths)
	agents := make([]Agent, 0, len(paths))
	for _, path := range paths {
		base := strings.TrimSuffix(path, ".jsonl")
		agent := Agent{ID: strings.TrimPrefix(filepath.Base(base), "agent-")}
		if body, err := os.ReadFile(base + ".meta.json"); err == nil {
			var m meta
			if err := json.Unmarshal(body, &m); err != nil {
				return nil, fmt.Errorf("reading %s.meta.json: %w", base, err)
			}
			agent.Description, agent.Model, agent.Finished = m.Description, m.Model, m.Finished
		}
		reading, err := latest(path, true)
		switch {
		case err == nil:
			agent.Reading = reading
		case !errors.Is(err, ErrNoUsage):
			return nil, err
		}
		agents = append(agents, agent)
	}
	return agents, nil
}

// LatestOfAgent returns the latest reading of one subagent of the session
// whose transcript is at sessionPath.
func LatestOfAgent(sessionPath, agentID string) (Reading, error) {
	return latest(agentPath(sessionPath, agentID), true)
}

// agentPath is where a subagent's transcript lives:
// <session id>/subagents/agent-<id>.jsonl beside the session's own.
func agentPath(sessionPath, agentID string) string {
	return filepath.Join(subagentsDir(sessionPath), "agent-"+agentID+".jsonl")
}

func subagentsDir(sessionPath string) string {
	return filepath.Join(strings.TrimSuffix(sessionPath, ".jsonl"), "subagents")
}
