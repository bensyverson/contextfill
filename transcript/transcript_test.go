package transcript_test

import (
	"errors"
	"path/filepath"
	"testing"

	"github.com/bensyverson/contextfill/fill"
	"github.com/bensyverson/contextfill/transcript"
)

const sessionID = "5f3a9c21-0b7e-4d2a-9c61-2e8f4a7b1d30"

func TestFindLocatesTheSessionInAnyProjectDirectory(t *testing.T) {
	projects := t.TempDir()
	want := filepath.Join(projects, "-Users-someone-git-App", sessionID+".jsonl")
	writeLines(t, want, assistant("claude-opus-5-5[1m]", usage{input: 1}))
	writeLines(t, filepath.Join(projects, "-Users-someone-git-Other", "other-session.jsonl"), assistant("claude-opus-5-5", usage{input: 1}))

	got, err := transcript.Find(projects, sessionID)
	if err != nil {
		t.Fatalf("Find: %v", err)
	}
	if got != want {
		t.Errorf("Find = %q, want %q", got, want)
	}
}

func TestFindReportsAMissingSession(t *testing.T) {
	_, err := transcript.Find(t.TempDir(), sessionID)
	if !errors.Is(err, fill.ErrNoSession) {
		t.Errorf("Find error = %v, want ErrNoSession", err)
	}
}

func TestLatestSumsTheInputSideOfTheLastAssistantTurn(t *testing.T) {
	path := filepath.Join(t.TempDir(), sessionID+".jsonl")
	writeLines(t, path,
		entry(map[string]any{"type": "user", "message": map[string]any{"content": "hello"}}),
		assistant("claude-opus-5-5[1m]", usage{input: 5, cacheRead: 1000, cacheCreation: 200, output: 50}),
		entry(map[string]any{"type": "user", "message": map[string]any{"content": "more"}}),
		assistant("claude-opus-5-5[1m]", usage{input: 2, cacheRead: 201000, cacheCreation: 33513, output: 1148}),
		entry(map[string]any{"type": "attachment"}),
	)

	got, err := transcript.Latest(path)
	if err != nil {
		t.Fatalf("Latest: %v", err)
	}
	if got.Used != 234515 {
		t.Errorf("Used = %d, want 234515", got.Used)
	}
	if got.Window != 1_000_000 {
		t.Errorf("Window = %d, want 1000000", got.Window)
	}
	if got.Model != "claude-opus-5-5[1m]" {
		t.Errorf("Model = %q, want claude-opus-5-5[1m]", got.Model)
	}
	if fill := got.Fill(); fill < 0.2345 || fill > 0.2346 {
		t.Errorf("Fill = %f, want 0.2345", fill)
	}
}

func TestLatestSkipsTurnsWithNoUsageAndUnreadableLines(t *testing.T) {
	path := filepath.Join(t.TempDir(), sessionID+".jsonl")
	writeLines(t, path,
		assistant("claude-sonnet-5-5", usage{input: 10, cacheRead: 90}),
		"{not json",
		assistant("claude-sonnet-5-5", usage{}),
	)

	got, err := transcript.Latest(path)
	if err != nil {
		t.Fatalf("Latest: %v", err)
	}
	if got.Used != 100 {
		t.Errorf("Used = %d, want 100 (the zero-usage turn is skipped)", got.Used)
	}
}

func TestLatestSkipsSidechainTurns(t *testing.T) {
	path := filepath.Join(t.TempDir(), sessionID+".jsonl")
	side := entry(map[string]any{
		"type": "assistant", "isSidechain": true, "requestedModel": "claude-opus-5-5",
		"message": map[string]any{"usage": map[string]any{"input_tokens": 99999}},
	})
	writeLines(t, path, assistant("claude-opus-5-5", usage{input: 7}), side)

	got, err := transcript.Latest(path)
	if err != nil {
		t.Fatalf("Latest: %v", err)
	}
	if got.Used != 7 {
		t.Errorf("Used = %d, want 7", got.Used)
	}
}

func TestLatestReportsATranscriptWithNoAssistantTurn(t *testing.T) {
	path := filepath.Join(t.TempDir(), sessionID+".jsonl")
	writeLines(t, path, entry(map[string]any{"type": "user"}))

	_, err := transcript.Latest(path)
	if !errors.Is(err, fill.ErrNoUsage) {
		t.Errorf("Latest error = %v, want ErrNoUsage", err)
	}
}

func TestWindowForModel(t *testing.T) {
	cases := map[string]int{
		// Claude Code 2.1.311 stopped adding [1m] to models whose window is 1M by default.
		"claude-opus-5-5":                   1_000_000,
		"claude-opus-5-5[1m]":               1_000_000,
		"claude-opus-5":                     1_000_000,
		"claude-opus-4-7":                   1_000_000,
		"claude-opus-4-6":                   200_000,
		"claude-opus-4-6[1m]":               1_000_000,
		"claude-opus-4-5-20251101":          200_000,
		"claude-sonnet-5-5":                 1_000_000,
		"claude-sonnet-5":                   1_000_000,
		"claude-sonnet-4-6":                 200_000,
		"claude-sonnet-4-6[1m]":             1_000_000,
		"claude-haiku-5-5":                  1_000_000,
		"claude-haiku-4-5-20251001":         200_000,
		"claude-fable-5":                    1_000_000,
		"claude-fable-5-1":                  1_000_000,
		"claude-opus-6":                     1_000_000,
		"us.anthropic.claude-opus-5-5-v1:0": 1_000_000,
		"some-gateway-alias":                200_000,
		"some-gateway-alias[1m]":            1_000_000,
		"":                                  200_000,
	}
	for model, want := range cases {
		if got := transcript.WindowFor(model); got != want {
			t.Errorf("WindowFor(%q) = %d, want %d", model, got, want)
		}
	}
}

func TestWindowForHonorsClaudeCodesOptOutOf1M(t *testing.T) {
	t.Setenv("CLAUDE_CODE_DISABLE_1M_CONTEXT", "1")
	for _, model := range []string{"claude-opus-5-5", "claude-opus-5-5[1m]", "claude-sonnet-4-6[1m]", "claude-fable-5-1"} {
		if got := transcript.WindowFor(model); got != 200_000 {
			t.Errorf("WindowFor(%q) = %d with 1M disabled, want 200000", model, got)
		}
	}
}

func TestLatestReadsABareOpus55As1M(t *testing.T) {
	path := filepath.Join(t.TempDir(), sessionID+".jsonl")
	writeLines(t, path, assistant("claude-opus-5-5", usage{input: 2, cacheRead: 126_600}))

	got, err := transcript.Latest(path)
	if err != nil {
		t.Fatalf("Latest: %v", err)
	}
	if got.Window != 1_000_000 {
		t.Errorf("Window = %d, want 1000000", got.Window)
	}
}

var _ fill.Source = transcript.Projects{}

func TestProjectsReadsASessionByID(t *testing.T) {
	projects := t.TempDir()
	session := filepath.Join(projects, "-Users-someone-git-App", sessionID+".jsonl")
	writeLines(t, session, assistant("claude-opus-5-5[1m]", usage{input: 5, cacheRead: 300_000}))
	writeLines(t, filepath.Join(projects, "-Users-someone-git-App", sessionID, "subagents", "agent-a1.jsonl"),
		assistant("claude-sonnet-5-5", usage{input: 50_000}))

	source := transcript.Projects{Dir: projects}
	reading, err := source.Latest(sessionID)
	if err != nil {
		t.Fatalf("Latest: %v", err)
	}
	if reading.Used != 300_005 || reading.Window != 1_000_000 {
		t.Errorf("Latest = %+v", reading)
	}
	agents, err := source.Agents(sessionID)
	if err != nil || len(agents) != 1 || agents[0].ID != "a1" {
		t.Errorf("Agents = %+v, %v", agents, err)
	}
	if _, err := source.Latest("no-such-session"); !errors.Is(err, fill.ErrNoSession) {
		t.Errorf("Latest of a missing session: %v, want ErrNoSession", err)
	}
}
