package fill_test

import (
	"testing"

	"github.com/bensyverson/contextfill/fill"
)

func TestSummaryRendersTheFillAgainstTheWindow(t *testing.T) {
	r := fill.Reading{Model: "claude-opus-5-5[1m]", Used: 234_515, Window: 1_000_000}
	if got := r.Summary(); got != "23% of 1M" {
		t.Errorf("Summary = %q, want 23%% of 1M", got)
	}
	if !r.HasWindow() {
		t.Error("HasWindow = false for a 1M window")
	}
}

func TestAReadingWithNoWindowSaysSo(t *testing.T) {
	r := fill.Reading{Model: "local/some-model", Used: 11_523}
	if r.HasWindow() {
		t.Error("HasWindow = true with no window")
	}
	if got := r.Summary(); got != "window unknown" {
		t.Errorf("Summary = %q, want window unknown", got)
	}
	if r.Fill() != 0 {
		t.Errorf("Fill = %v, want 0", r.Fill())
	}
}

func TestTokensRendersCountsCompactly(t *testing.T) {
	for n, want := range map[int]string{950: "950", 20_000: "20k", 234_515: "234.5k", 1_000_000: "1M", 1_048_576: "1M", 1_250_000: "1.3M"} {
		if got := fill.Tokens(n); got != want {
			t.Errorf("Tokens(%d) = %q, want %q", n, got, want)
		}
	}
}

func TestDetectPrefersOpenCodeOverAnInheritedClaudeCodeSession(t *testing.T) {
	for _, c := range []struct {
		env  map[string]string
		want fill.Harness
	}{
		{map[string]string{"OPENCODE": "1", "CLAUDE_CODE_SESSION_ID": "5f3a9c21", "CLAUDECODE": "1"}, fill.OpenCode},
		{map[string]string{"OPENCODE": "1"}, fill.OpenCode},
		{map[string]string{"CLAUDECODE": "1"}, fill.ClaudeCode},
		{map[string]string{"CLAUDE_CODE_SESSION_ID": "5f3a9c21"}, fill.ClaudeCode},
		{map[string]string{}, fill.ClaudeCode},
	} {
		if got := fill.Detect(func(name string) string { return c.env[name] }); got != c.want {
			t.Errorf("Detect(%v) = %q, want %q", c.env, got, c.want)
		}
	}
}

func TestEachHarnessNamesItsSessionVariable(t *testing.T) {
	if got := fill.ClaudeCode.SessionEnv(); got != "CLAUDE_CODE_SESSION_ID" {
		t.Errorf("ClaudeCode.SessionEnv = %q", got)
	}
	if got := fill.OpenCode.SessionEnv(); got != "OPENCODE_SESSION_ID" {
		t.Errorf("OpenCode.SessionEnv = %q", got)
	}
}

func TestParseHarnessAcceptsEachName(t *testing.T) {
	for name, want := range map[string]fill.Harness{"claude": fill.ClaudeCode, "opencode": fill.OpenCode} {
		if got, err := fill.ParseHarness(name); err != nil || got != want {
			t.Errorf("ParseHarness(%q) = %q, %v; want %q", name, got, err, want)
		}
	}
	if _, err := fill.ParseHarness("cursor"); err == nil {
		t.Error("ParseHarness(cursor) succeeded")
	}
}
