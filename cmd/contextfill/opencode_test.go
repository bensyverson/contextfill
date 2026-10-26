package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/bensyverson/contextfill/fill"
)

const openCodeSession = "ses_synthetic0main"

// openCodeState runs a fake OpenCode server whose one session's latest step
// used the given tokens of a model with the given window (0: unknown), and
// returns an XDG_STATE_HOME whose service.json points at it.
func openCodeState(t *testing.T, used, window int) string {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if _, pass, _ := r.BasicAuth(); pass != "pw" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		var answer any
		switch r.URL.Path {
		case "/api/session/" + openCodeSession:
			answer = map[string]any{"data": map[string]any{"id": openCodeSession, "location": map[string]any{"directory": "/w"}}}
		case "/api/session/" + openCodeSession + "/message":
			var data []any
			if r.URL.Query().Get("cursor") == "" {
				data = []any{map[string]any{"type": "assistant", "model": map[string]any{"id": "m", "providerID": "p"},
					"tokens": map[string]any{"input": 3, "output": 9, "cache": map[string]any{"read": used - 3, "write": 0}}}}
			}
			answer = map[string]any{"data": data, "cursor": map[string]any{"next": "1"}}
		case "/api/model":
			var data []any
			if window > 0 {
				data = []any{map[string]any{"id": "m", "providerID": "p", "limit": map[string]any{"context": window}}}
			}
			answer = map[string]any{"data": data}
		case "/api/session":
			answer = map[string]any{"data": []any{}}
		default:
			w.WriteHeader(http.StatusNotFound)
			answer = map[string]any{"_tag": "SessionNotFoundError"}
		}
		json.NewEncoder(w).Encode(answer)
	}))
	t.Cleanup(srv.Close)
	state := t.TempDir()
	body, _ := json.Marshal(map[string]any{"url": srv.URL, "password": "pw", "pid": 1})
	write(t, filepath.Join(state, "opencode", "service.json"), string(body))
	return state
}

func TestInsideOpenCodeItReadsTheOpenCodeSession(t *testing.T) {
	env := map[string]string{
		"XDG_STATE_HOME": openCodeState(t, 262_144, 1_048_576),
		"OPENCODE":       "1", "OPENCODE_SESSION_ID": openCodeSession,
		// Inherited from the Claude Code shell OpenCode was started in.
		"CLAUDECODE": "1", "CLAUDE_CODE_SESSION_ID": sessionID,
	}
	code, out, errOut := invoke(env)
	if code != 0 {
		t.Fatalf("exit %d, stderr %q", code, errOut)
	}
	if out != "25% of 1M · 262.1k tokens · p/m · session "+openCodeSession+"\n" {
		t.Errorf("stdout = %q", out)
	}
}

func TestHarnessFlagOverridesTheEnvironment(t *testing.T) {
	env := map[string]string{"XDG_STATE_HOME": openCodeState(t, 1000, 200_000), "CLAUDECODE": "1"}
	code, out, errOut := invoke(env, "--harness", "opencode", openCodeSession)
	if code != 0 || !strings.HasPrefix(out, "1% of 200k") {
		t.Errorf("exit %d, stdout %q, stderr %q", code, out, errOut)
	}
}

func TestAnUnknownWindowSaysSoAndCannotBeJudged(t *testing.T) {
	env := map[string]string{"XDG_STATE_HOME": openCodeState(t, 11_523, 0), "OPENCODE": "1", "OPENCODE_SESSION_ID": openCodeSession}
	if code, out, _ := invoke(env); code != 0 || !strings.HasPrefix(out, "window unknown · 11.5k tokens · p/m") {
		t.Errorf("exit %d, stdout %q", code, out)
	}
	if code, _, errOut := invoke(env, "--max", "40"); code != 4 || !strings.Contains(errOut, "window is unknown") {
		t.Errorf("--max: exit %d, stderr %q; want 4 and why", code, errOut)
	}
	_, out, _ := invoke(env, "--json")
	var r struct {
		Harness string   `json:"harness"`
		Window  int      `json:"window"`
		Fill    *float64 `json:"fill"`
	}
	if err := json.Unmarshal([]byte(out), &r); err != nil || r.Harness != "opencode" || r.Window != 0 || r.Fill != nil {
		t.Errorf("--json = %s (%v); want harness opencode, window 0 and fill null", out, err)
	}
}

func TestOpenCodeWithNoServerSaysHowToStartIt(t *testing.T) {
	env := map[string]string{"XDG_STATE_HOME": t.TempDir(), "OPENCODE": "1", "OPENCODE_SESSION_ID": openCodeSession}
	if code, _, errOut := invoke(env); code != 5 || !strings.Contains(errOut, "opencode service start") {
		t.Errorf("exit %d, stderr %q; want 5 and the remedy", code, errOut)
	}
}

func TestOpenCodeWithNoSessionNamesItsVariable(t *testing.T) {
	env := map[string]string{"XDG_STATE_HOME": openCodeState(t, 1000, 200_000), "OPENCODE": "1"}
	if code, _, errOut := invoke(env); code != 5 || !strings.Contains(errOut, "OPENCODE_SESSION_ID") {
		t.Errorf("exit %d, stderr %q", code, errOut)
	}
}

func TestAnUnknownOpenCodeSessionSaysHowToListThem(t *testing.T) {
	env := map[string]string{"XDG_STATE_HOME": openCodeState(t, 1000, 200_000), "OPENCODE": "1"}
	if code, _, errOut := invoke(env, "ses_nope"); code != 4 || !strings.Contains(errOut, "opencode session list") {
		t.Errorf("exit %d, stderr %q", code, errOut)
	}
}

func TestAnAgentWithAReadingButNoWindowIsNotWaiting(t *testing.T) {
	var out strings.Builder
	agents := []fill.Agent{{ID: "ses_child", Model: "p/m", Reading: fill.Reading{Model: "p/m", Used: 5000}}}
	writeText(&out, openCodeSession, fill.Reading{Model: "p/m", Used: 9000}, agents, options{agents: true})
	if !strings.Contains(out.String(), "ses_child  window unknown") {
		t.Errorf("listing = %q, want the agent's reading shown as window unknown", out.String())
	}
}

func TestHookForOpenCodeAnswersACommitInPlainText(t *testing.T) {
	env := map[string]string{"XDG_STATE_HOME": openCodeState(t, 262_144, 1_048_576)}
	commit, _ := json.Marshal(map[string]any{"tool": "shell", "sessionID": openCodeSession, "input": map[string]any{"command": "git commit -F m"}})
	code, out, errOut := invokeWithInput(env, string(commit), "hook", "--harness", "opencode", "--max", "40", "--main-max", "20")
	if code != 0 || out != "contextfill: 25% of 1M · 262.1k tokens in context after this commit (over 20%).\n" {
		t.Errorf("exit %d, stdout %q, stderr %q", code, out, errOut)
	}
	status, _ := json.Marshal(map[string]any{"tool": "shell", "sessionID": openCodeSession, "input": map[string]any{"command": "git status"}})
	if code, out, _ := invokeWithInput(env, string(status), "hook", "--harness", "opencode"); code != 0 || out != "" {
		t.Errorf("non-commit: exit %d, stdout %q; want silence", code, out)
	}
}

func TestHookForOpenCodeWithNoServerDoesNotBlock(t *testing.T) {
	env := map[string]string{"XDG_STATE_HOME": t.TempDir()}
	code, out, errOut := invokeWithInput(env, `{"tool":"shell","sessionID":"ses_x","input":{"command":"git commit"}}`, "hook", "--harness", "opencode")
	if code == 0 || code == 2 || out != "" || !strings.Contains(errOut, "contextfill hook") {
		t.Errorf("exit %d, stdout %q, stderr %q; want a non-blocking error naming the hook", code, out, errOut)
	}
}
