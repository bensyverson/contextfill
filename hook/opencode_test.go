package hook_test

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/bensyverson/contextfill/fill"
	"github.com/bensyverson/contextfill/hook"
)

// sessions is a stub OpenCode reader: each session's reading, and which are
// subagents.
type sessions struct {
	readings  map[string]fill.Reading
	subagents map[string]bool
}

func (s sessions) Latest(id string) (fill.Reading, error) {
	r, ok := s.readings[id]
	if !ok {
		return fill.Reading{}, fill.ErrNoSession
	}
	return r, nil
}

func (s sessions) IsSubagent(id string) (bool, error) {
	if _, ok := s.readings[id]; !ok {
		return false, fill.ErrNoSession
	}
	return s.subagents[id], nil
}

var openCode = sessions{
	readings: map[string]fill.Reading{
		"ses_main":  {Model: "p/big", Used: 300_000, Window: 1_000_000},
		"ses_child": {Model: "p/big", Used: 300_000, Window: 1_000_000},
		"ses_local": {Model: "local/m", Used: 11_523},
	},
	subagents: map[string]bool{"ses_child": true},
}

// event is the payload the OpenCode plugin pipes in: the tool call's own
// field names.
func event(tool, sessionID, command string) string {
	body, _ := json.Marshal(map[string]any{"tool": tool, "sessionID": sessionID, "input": map[string]any{"command": command}})
	return string(body)
}

func TestAfterAnOpenCodeCommitItReportsTheFill(t *testing.T) {
	got, err := hook.RespondOpenCode(strings.NewReader(event("shell", "ses_main", "git add -A && git commit -F m")), hook.Limits{}, openCode)
	if err != nil {
		t.Fatalf("RespondOpenCode: %v", err)
	}
	if got != "contextfill: 30% of 1M · 300k tokens in context after this commit." {
		t.Errorf("RespondOpenCode = %q", got)
	}
}

func TestOpenCodesMainSessionIsJudgedAgainstMainMax(t *testing.T) {
	got, _ := hook.RespondOpenCode(strings.NewReader(event("shell", "ses_main", "git commit")), hook.Limits{Max: 40, MainMax: 25}, openCode)
	if !strings.Contains(got, "over the 25% limit") {
		t.Errorf("RespondOpenCode = %q, want the main session over 25%%", got)
	}
}

func TestAnOpenCodeSubagentIsJudgedAgainstMax(t *testing.T) {
	got, _ := hook.RespondOpenCode(strings.NewReader(event("shell", "ses_child", "git commit")), hook.Limits{Max: 40, MainMax: 25}, openCode)
	if got == "" || strings.Contains(got, "limit") {
		t.Errorf("RespondOpenCode = %q, want the subagent's 30%% under --max 40", got)
	}
}

func TestAnOpenCodeModelWithNoWindowGetsNoAdvice(t *testing.T) {
	got, _ := hook.RespondOpenCode(strings.NewReader(event("shell", "ses_local", "git commit")), hook.Limits{Max: 1}, openCode)
	if got != "contextfill: window unknown · 11.5k tokens in context after this commit." {
		t.Errorf("RespondOpenCode = %q", got)
	}
}

func TestOtherOpenCodeToolCallsGetNoResponse(t *testing.T) {
	if got, _ := hook.RespondOpenCode(strings.NewReader(event("shell", "ses_main", "git commit")), hook.Limits{}, openCode); got == "" {
		t.Fatal("a commit got no response, so silence below proves nothing")
	}
	for _, payload := range []string{event("shell", "ses_main", "git status"), event("read", "ses_main", "git commit")} {
		if got, err := hook.RespondOpenCode(strings.NewReader(payload), hook.Limits{}, openCode); got != "" || err != nil {
			t.Errorf("RespondOpenCode(%s) = %q, %v; want nothing", payload, got, err)
		}
	}
}

func TestABrokenOpenCodePayloadIsAnError(t *testing.T) {
	if _, err := hook.RespondOpenCode(strings.NewReader("not json"), hook.Limits{}, openCode); err == nil {
		t.Error("RespondOpenCode accepted a payload that isn't JSON")
	}
	if _, err := hook.RespondOpenCode(strings.NewReader(event("shell", "ses_gone", "git commit")), hook.Limits{}, openCode); err == nil {
		t.Error("RespondOpenCode succeeded for a session that doesn't exist")
	}
}
