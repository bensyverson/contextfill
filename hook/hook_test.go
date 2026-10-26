package hook_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/bensyverson/contextfill/hook"
)

const sessionID = "5f3a9c21-0b7e-4d2a-9c61-2e8f4a7b1d30"

func turn(model string, used int, sidechain bool) string {
	line, _ := json.Marshal(map[string]any{
		"type": "assistant", "isSidechain": sidechain, "requestedModel": model,
		"message": map[string]any{"usage": map[string]any{"input_tokens": used}},
	})
	return string(line) + "\n"
}

func write(t *testing.T, path, body string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

// input is a PostToolUse payload as Claude Code sends it.
func input(transcript, agentID, command string) string {
	fields := map[string]any{
		"session_id": sessionID, "transcript_path": transcript,
		"hook_event_name": "PostToolUse", "tool_name": "Bash",
		"tool_input": map[string]any{"command": command},
	}
	if agentID != "" {
		fields["agent_id"] = agentID
	}
	body, _ := json.Marshal(fields)
	return string(body)
}

// context decodes a response and returns its additionalContext.
func context(t *testing.T, out []byte) string {
	t.Helper()
	var r struct {
		HookSpecificOutput struct {
			HookEventName     string `json:"hookEventName"`
			AdditionalContext string `json:"additionalContext"`
		} `json:"hookSpecificOutput"`
	}
	if err := json.Unmarshal(out, &r); err != nil {
		t.Fatalf("response is not JSON: %v\n%s", err, out)
	}
	if r.HookSpecificOutput.HookEventName != "PostToolUse" {
		t.Errorf("hookEventName = %q, want PostToolUse", r.HookSpecificOutput.HookEventName)
	}
	return r.HookSpecificOutput.AdditionalContext
}

func TestAfterACommitItReportsTheMainThreadsFill(t *testing.T) {
	transcript := filepath.Join(t.TempDir(), sessionID+".jsonl")
	write(t, transcript, turn("claude-opus-5-5[1m]", 300_000, false))

	out, err := hook.Respond(strings.NewReader(input(transcript, "", "cd x && git commit -F m")), hook.Limits{})
	if err != nil {
		t.Fatalf("Respond: %v", err)
	}
	if got := context(t, out); got != "contextfill: 30% of 1M · 300k tokens in context after this commit." {
		t.Errorf("additionalContext = %q", got)
	}
}

func TestInASubagentItReportsTheAgentsOwnFill(t *testing.T) {
	dir := t.TempDir()
	transcript := filepath.Join(dir, sessionID+".jsonl")
	write(t, transcript, turn("claude-opus-5-5[1m]", 300_000, false))
	write(t, filepath.Join(dir, sessionID, "subagents", "agent-a7f0b3d9.jsonl"), turn("claude-sonnet-5-5", 90_000, true))

	out, err := hook.Respond(strings.NewReader(input(transcript, "a7f0b3d9", "git commit -F m")), hook.Limits{})
	if err != nil {
		t.Fatalf("Respond: %v", err)
	}
	if got := context(t, out); !strings.HasPrefix(got, "contextfill: 45% of 200k · 90k tokens") {
		t.Errorf("additionalContext = %q, want the agent's 45%%", got)
	}
}

func TestOverTheLimitItSaysSo(t *testing.T) {
	transcript := filepath.Join(t.TempDir(), sessionID+".jsonl")
	write(t, transcript, turn("claude-opus-5-5[1m]", 450_000, false))

	out, err := hook.Respond(strings.NewReader(input(transcript, "", "git commit -F m")), hook.Limits{Max: 40})
	if err != nil {
		t.Fatalf("Respond: %v", err)
	}
	if got := context(t, out); !strings.HasSuffix(got, "after this commit (over 40%).") {
		t.Errorf("additionalContext = %q, want it to say the fill is over 40%%", got)
	}
}

func TestTheMainThreadIsJudgedAgainstMainMax(t *testing.T) {
	transcript := filepath.Join(t.TempDir(), sessionID+".jsonl")
	write(t, transcript, turn("claude-opus-5-5[1m]", 300_000, false))

	out, err := hook.Respond(strings.NewReader(input(transcript, "", "git commit -F m")), hook.Limits{Max: 40, MainMax: 25})
	if err != nil {
		t.Fatalf("Respond: %v", err)
	}
	if got := context(t, out); !strings.Contains(got, "(over 25%)") {
		t.Errorf("additionalContext = %q, want the main thread's 30%% over its 25%% limit", got)
	}
}

func TestASubagentIsJudgedAgainstMaxEvenWithMainMax(t *testing.T) {
	dir := t.TempDir()
	transcript := filepath.Join(dir, sessionID+".jsonl")
	write(t, transcript, turn("claude-opus-5-5[1m]", 300_000, false))
	write(t, filepath.Join(dir, sessionID, "subagents", "agent-a7f0b3d9.jsonl"), turn("claude-sonnet-5-5", 60_000, true))

	payload := input(transcript, "a7f0b3d9", "git commit -F m")
	out, err := hook.Respond(strings.NewReader(payload), hook.Limits{Max: 40, MainMax: 25})
	if err != nil {
		t.Fatalf("Respond: %v", err)
	}
	if got := context(t, out); strings.Contains(got, "over") {
		t.Errorf("additionalContext = %q, want no note for the agent's 30%% under --max 40", got)
	}
	out, _ = hook.Respond(strings.NewReader(payload), hook.Limits{Max: 20, MainMax: 50})
	if got := context(t, out); !strings.Contains(got, "(over 20%)") {
		t.Errorf("additionalContext = %q, want the agent's 30%% over --max 20", got)
	}
}

func TestACommandWithoutACommitGetsNoResponse(t *testing.T) {
	transcript := filepath.Join(t.TempDir(), sessionID+".jsonl")
	write(t, transcript, turn("claude-opus-5-5", 1000, false))

	if out, _ := hook.Respond(strings.NewReader(input(transcript, "", "git commit -F m")), hook.Limits{}); len(out) == 0 {
		t.Fatal("a commit got no response, so silence below proves nothing")
	}
	out, err := hook.Respond(strings.NewReader(input(transcript, "", "git status && echo 'git commit'")), hook.Limits{})
	if err != nil || len(out) != 0 {
		t.Errorf("Respond = %q, %v; want no output", out, err)
	}
}

func TestAnUnreadableTranscriptIsAnError(t *testing.T) {
	_, err := hook.Respond(strings.NewReader(input("/no/such/transcript.jsonl", "", "git commit")), hook.Limits{})
	if err == nil {
		t.Error("Respond succeeded with no transcript")
	}
}
