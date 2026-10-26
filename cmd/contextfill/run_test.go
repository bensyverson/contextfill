package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

const sessionID = "5f3a9c21-0b7e-4d2a-9c61-2e8f4a7b1d30"

// home lays out a Claude config dir holding one session transcript whose
// latest turn used the given tokens, and returns the dir.
func home(t *testing.T, model string, used int) string {
	t.Helper()
	config := t.TempDir()
	path := filepath.Join(config, "projects", "-Users-someone-git-App", sessionID+".jsonl")
	write(t, path, turn(model, used))
	return config
}

func turn(model string, used int) string {
	line, _ := json.Marshal(map[string]any{
		"type": "assistant", "requestedModel": model,
		"message": map[string]any{"usage": map[string]any{"input_tokens": 2, "cache_read_input_tokens": used - 2}},
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

// invoke runs the command with an environment of only the given variables.
func invoke(env map[string]string, args ...string) (code int, stdout, stderr string) {
	return invokeWithInput(env, "", args...)
}

// invokeWithInput is invoke with the given standard input.
func invokeWithInput(env map[string]string, stdin string, args ...string) (code int, stdout, stderr string) {
	var out, errOut bytes.Buffer
	code = run(args, func(name string) string { return env[name] }, strings.NewReader(stdin), &out, &errOut)
	return code, out.String(), errOut.String()
}

func TestReportsTheCurrentSessionFromItsEnvironment(t *testing.T) {
	config := home(t, "claude-opus-5-5[1m]", 234515)
	code, out, errOut := invoke(map[string]string{"CLAUDE_CONFIG_DIR": config, "CLAUDE_CODE_SESSION_ID": sessionID})

	if code != 0 {
		t.Fatalf("exit %d, stderr %q", code, errOut)
	}
	want := "23% of 1M · 234.5k tokens · claude-opus-5-5[1m] · session 5f3a9c21\n"
	if out != want {
		t.Errorf("stdout = %q, want %q", out, want)
	}
}

func TestASessionIDArgumentWinsOverTheEnvironment(t *testing.T) {
	config := home(t, "claude-sonnet-5-5", 150_000)
	code, out, _ := invoke(map[string]string{"CLAUDE_CONFIG_DIR": config, "CLAUDE_CODE_SESSION_ID": "some-other-session"}, sessionID)

	if code != 0 || !strings.HasPrefix(out, "75% of 200k · 150k tokens") {
		t.Errorf("exit %d, stdout %q", code, out)
	}
}

func TestFindsTheConfigDirUnderHomeByDefault(t *testing.T) {
	homeDir := t.TempDir()
	write(t, filepath.Join(homeDir, ".claude", "projects", "p", sessionID+".jsonl"), turn("claude-opus-5-5", 20_000))
	code, out, _ := invoke(map[string]string{"HOME": homeDir, "CLAUDE_CODE_SESSION_ID": sessionID})

	if code != 0 || !strings.HasPrefix(out, "10% of 200k · 20k tokens") {
		t.Errorf("exit %d, stdout %q", code, out)
	}
}

func TestMaxExitsOneWhenTheFillIsOver(t *testing.T) {
	config := home(t, "claude-opus-5-5[1m]", 450_000)
	env := map[string]string{"CLAUDE_CONFIG_DIR": config, "CLAUDE_CODE_SESSION_ID": sessionID}

	if code, out, _ := invoke(env, "--max", "40"); code != 1 || !strings.Contains(out, "over the 40% limit") {
		t.Errorf("over: exit %d, stdout %q", code, out)
	}
	if code, out, _ := invoke(env, "--max", "50"); code != 0 || strings.Contains(out, "limit") {
		t.Errorf("under: exit %d, stdout %q", code, out)
	}
}

func TestAgentsListsEachSubagent(t *testing.T) {
	config := home(t, "claude-opus-5-5[1m]", 100_000)
	subagents := filepath.Join(config, "projects", "-Users-someone-git-App", sessionID, "subagents")
	write(t, filepath.Join(subagents, "agent-a2c41e7b.jsonl"), turn("claude-opus-5-5[1m]", 412_000))
	write(t, filepath.Join(subagents, "agent-a2c41e7b.meta.json"), `{"description":"Add the export menu","model":"opus","finished":false}`)
	write(t, filepath.Join(subagents, "agent-b2.jsonl"), turn("claude-sonnet-5-5", 60_000))
	write(t, filepath.Join(subagents, "agent-b2.meta.json"), `{"description":"Docs index","model":"sonnet","finished":true}`)

	code, out, errOut := invoke(map[string]string{"CLAUDE_CONFIG_DIR": config, "CLAUDE_CODE_SESSION_ID": sessionID}, "--agents")
	if code != 0 {
		t.Fatalf("exit %d, stderr %q", code, errOut)
	}
	want := "10% of 1M · 100k tokens · claude-opus-5-5[1m] · session 5f3a9c21\n" +
		"  a2c41e7b  41% of 1M    running   opus    Add the export menu\n" +
		"  b2        30% of 200k  finished  sonnet  Docs index\n"
	if out != want {
		t.Errorf("stdout =\n%s\nwant\n%s", out, want)
	}
}

func TestAgentsSaysSoWhenThereAreNone(t *testing.T) {
	config := home(t, "claude-opus-5-5", 1000)
	_, out, _ := invoke(map[string]string{"CLAUDE_CONFIG_DIR": config, "CLAUDE_CODE_SESSION_ID": sessionID}, "--agents")
	if !strings.HasSuffix(out, "  no subagents\n") {
		t.Errorf("stdout = %q, want a closing no-subagents line", out)
	}
}

func TestJSONCarriesTheReadingAndAgents(t *testing.T) {
	config := home(t, "claude-opus-5-5[1m]", 250_000)
	code, out, _ := invoke(map[string]string{"CLAUDE_CONFIG_DIR": config, "CLAUDE_CODE_SESSION_ID": sessionID}, "--json", "--agents")
	if code != 0 {
		t.Fatalf("exit %d", code)
	}
	var got report
	if err := json.Unmarshal([]byte(out), &got); err != nil {
		t.Fatalf("stdout is not JSON: %v\n%s", err, out)
	}
	if got.Session != sessionID || got.Used != 250_000 || got.Window != 1_000_000 || got.Fill == nil || *got.Fill != 0.25 || got.Agents == nil {
		t.Errorf("report = %+v", got)
	}
}

func TestWithoutASessionItExitsFiveAndSaysHowToGiveOne(t *testing.T) {
	code, out, errOut := invoke(map[string]string{"HOME": t.TempDir()})
	if code != 5 || out != "" {
		t.Errorf("exit %d, stdout %q", code, out)
	}
	if !strings.Contains(errOut, "CLAUDE_CODE_SESSION_ID") || !strings.Contains(errOut, "contextfill <session-id>") {
		t.Errorf("stderr %q does not say how to name a session", errOut)
	}
}

func TestAnUnknownSessionExitsFourAndNamesWhereItLooked(t *testing.T) {
	config := t.TempDir()
	code, _, errOut := invoke(map[string]string{"CLAUDE_CONFIG_DIR": config}, "no-such-session")
	if code != 4 || !strings.Contains(errOut, filepath.Join(config, "projects")) {
		t.Errorf("exit %d, stderr %q", code, errOut)
	}
}

func TestUsageErrorsExitTwo(t *testing.T) {
	env := map[string]string{"CLAUDE_CODE_SESSION_ID": sessionID}
	for _, args := range [][]string{{"--max"}, {"--max", "lots"}, {"--frobnicate"}, {"a", "b"}, {"--main-max", "25"}, {"hook", "--main-max", "0"}, {"--harness", "cursor"}, {"--harness"}, {"hook", "--harness", "cursor"}} {
		if code, _, errOut := invoke(env, args...); code != 2 || !strings.Contains(errOut, "contextfill --help") {
			t.Errorf("%v: exit %d, stderr %q", args, code, errOut)
		}
	}
}

func TestHelpNeedsNoSession(t *testing.T) {
	code, out, _ := invoke(map[string]string{}, "--help")
	if code != 0 || !strings.Contains(out, "contextfill --agents") || !strings.Contains(out, "--max") {
		t.Errorf("exit %d, stdout %q", code, out)
	}
}

func TestHookAnswersACommitWithTheFill(t *testing.T) {
	config := home(t, "claude-opus-5-5[1m]", 450_000)
	transcript := filepath.Join(config, "projects", "-Users-someone-git-App", sessionID+".jsonl")
	payload, _ := json.Marshal(map[string]any{
		"transcript_path": transcript, "tool_name": "Bash",
		"tool_input": map[string]any{"command": "git add -A && git commit -F m"},
	})

	code, out, errOut := invokeWithInput(map[string]string{}, string(payload), "hook", "--max", "40")
	if code != 0 {
		t.Fatalf("exit %d, stderr %q", code, errOut)
	}
	if !strings.Contains(out, `"additionalContext":"contextfill: 45% of 1M`) || !strings.Contains(out, "over the 40% limit") {
		t.Errorf("stdout = %q", out)
	}
}

func TestHookJudgesTheMainThreadAgainstMainMax(t *testing.T) {
	config := home(t, "claude-opus-5-5[1m]", 300_000)
	transcript := filepath.Join(config, "projects", "-Users-someone-git-App", sessionID+".jsonl")
	payload, _ := json.Marshal(map[string]any{
		"transcript_path": transcript, "tool_name": "Bash",
		"tool_input": map[string]any{"command": "git commit -F m"},
	})

	code, out, errOut := invokeWithInput(nil, string(payload), "hook", "--max", "40", "--main-max=25")
	if code != 0 || !strings.Contains(out, "over the 25% limit") {
		t.Errorf("exit %d, stdout %q, stderr %q; want the main thread over --main-max 25", code, out, errOut)
	}
}

func TestHelpDocumentsMainMax(t *testing.T) {
	if _, out, _ := invoke(nil, "--help"); !strings.Contains(out, "--main-max") {
		t.Errorf("--help doesn't mention --main-max:\n%s", out)
	}
}

func TestVersionNeedsNoSession(t *testing.T) {
	code, out, _ := invoke(map[string]string{}, "--version")
	if code != 0 || !regexp.MustCompile(`^contextfill \d+\.\d+\.\d+\n$`).MatchString(out) {
		t.Errorf("exit %d, stdout %q", code, out)
	}
}

func TestHelpDocumentsVersion(t *testing.T) {
	if _, out, _ := invoke(nil, "--help"); !strings.Contains(out, "--version") {
		t.Errorf("--help doesn't mention --version:\n%s", out)
	}
}

func TestHelpDocumentsOpenCode(t *testing.T) {
	if _, out, _ := invoke(nil, "--help"); !strings.Contains(out, "--harness") || !strings.Contains(out, "OpenCode") {
		t.Errorf("--help doesn't cover OpenCode and --harness:\n%s", out)
	}
}

func TestJSONNamesTheHarness(t *testing.T) {
	config := home(t, "claude-opus-5-5[1m]", 234515)
	_, out, _ := invoke(map[string]string{"CLAUDE_CONFIG_DIR": config, "CLAUDE_CODE_SESSION_ID": sessionID}, "--json")
	if !strings.Contains(out, `"harness": "claude"`) {
		t.Errorf("--json = %s, want harness claude", out)
	}
}

func TestHookIsSilentForOtherCommands(t *testing.T) {
	config := home(t, "claude-opus-5-5", 1000)
	transcript := filepath.Join(config, "projects", "-Users-someone-git-App", sessionID+".jsonl")
	commit, _ := json.Marshal(map[string]any{"transcript_path": transcript, "tool_name": "Bash", "tool_input": map[string]any{"command": "git commit"}})
	status, _ := json.Marshal(map[string]any{"transcript_path": transcript, "tool_name": "Bash", "tool_input": map[string]any{"command": "git status"}})

	if _, out, _ := invokeWithInput(nil, string(commit), "hook"); out == "" {
		t.Fatal("a commit got no answer, so silence below proves nothing")
	}
	if code, out, _ := invokeWithInput(nil, string(status), "hook"); code != 0 || out != "" {
		t.Errorf("exit %d, stdout %q; want silence", code, out)
	}
}

func TestHookReportsABrokenPayloadWithoutBlocking(t *testing.T) {
	code, out, errOut := invokeWithInput(nil, "not json", "hook")
	if code == 0 || code == 2 || out != "" || !strings.Contains(errOut, "contextfill hook") {
		t.Errorf("exit %d, stdout %q, stderr %q; want a non-blocking error naming the hook", code, out, errOut)
	}
}
