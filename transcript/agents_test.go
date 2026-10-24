package transcript_test

import (
	"path/filepath"
	"testing"

	"github.com/bensyverson/contextfill/transcript"
)

func TestAgentsListsEachSubagentWithItsFill(t *testing.T) {
	dir := t.TempDir()
	session := filepath.Join(dir, sessionID+".jsonl")
	writeLines(t, session, assistant("claude-opus-5-5[1m]", usage{input: 1}))
	subagents := filepath.Join(dir, sessionID, "subagents")
	writeLines(t, filepath.Join(subagents, "agent-a1.jsonl"), assistant("claude-opus-5-5[1m]", usage{input: 400_000}))
	writeFile(t, filepath.Join(subagents, "agent-a1.meta.json"), `{"description":"Add the export menu","model":"opus","finished":true}`)
	writeLines(t, filepath.Join(subagents, "agent-b2.jsonl"), assistant("claude-sonnet-5-5", usage{input: 50_000}))
	writeFile(t, filepath.Join(subagents, "agent-b2.meta.json"), `{"description":"Docs index","model":"sonnet","finished":false}`)

	agents, err := transcript.Agents(session)
	if err != nil {
		t.Fatalf("Agents: %v", err)
	}
	if len(agents) != 2 {
		t.Fatalf("Agents returned %d, want 2: %+v", len(agents), agents)
	}
	a, b := agents[0], agents[1]
	if a.ID != "a1" || a.Description != "Add the export menu" || !a.Finished || a.Reading.Used != 400_000 || a.Reading.Window != 1_000_000 {
		t.Errorf("first agent = %+v", a)
	}
	if b.ID != "b2" || b.Description != "Docs index" || b.Finished || b.Reading.Used != 50_000 || b.Reading.Window != 200_000 {
		t.Errorf("second agent = %+v", b)
	}
}

func TestAgentsKeepsAnAgentThatHasNotAnsweredYet(t *testing.T) {
	dir := t.TempDir()
	session := filepath.Join(dir, sessionID+".jsonl")
	writeLines(t, session, assistant("claude-opus-5-5", usage{input: 1}))
	subagents := filepath.Join(dir, sessionID, "subagents")
	writeLines(t, filepath.Join(subagents, "agent-c3.jsonl"), entry(map[string]any{"type": "user"}))
	writeFile(t, filepath.Join(subagents, "agent-c3.meta.json"), `{"description":"Just started","model":"opus"}`)

	agents, err := transcript.Agents(session)
	if err != nil {
		t.Fatalf("Agents: %v", err)
	}
	if len(agents) != 1 || agents[0].Reading.Used != 0 || agents[0].Description != "Just started" {
		t.Errorf("Agents = %+v, want one agent with no reading yet", agents)
	}
}

// A subagent's own transcript marks every turn as a sidechain of its parent.
func TestAgentsReadsSidechainTurnsInAnAgentsOwnTranscript(t *testing.T) {
	dir := t.TempDir()
	session := filepath.Join(dir, sessionID+".jsonl")
	writeLines(t, session, assistant("claude-opus-5-5", usage{input: 1}))
	subagents := filepath.Join(dir, sessionID, "subagents")
	side := entry(map[string]any{
		"type": "assistant", "isSidechain": true, "requestedModel": "claude-opus-5-5[1m]",
		"message": map[string]any{"usage": map[string]any{"input_tokens": 3, "cache_read_input_tokens": 199_997}},
	})
	writeLines(t, filepath.Join(subagents, "agent-d4.jsonl"), side)

	agents, err := transcript.Agents(session)
	if err != nil {
		t.Fatalf("Agents: %v", err)
	}
	if len(agents) != 1 || agents[0].Reading.Used != 200_000 || agents[0].Reading.Window != 1_000_000 {
		t.Errorf("Agents = %+v, want one agent at 200k of 1M", agents)
	}
}
