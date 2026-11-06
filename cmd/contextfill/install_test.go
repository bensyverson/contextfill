package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// homebrew lays out a Homebrew prefix holding contextfill, with the OpenCode
// plugin when withPlugin is set, makes its Cellar binary the running one, and
// returns the stable path a hook should name.
func homebrew(t *testing.T, withPlugin bool) string {
	t.Helper()
	prefix, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	keg := filepath.Join(prefix, "Cellar", "contextfill", "0.2.0")
	binary := filepath.Join(keg, "bin", "contextfill")
	write(t, binary, "#!/bin/sh\n")
	if withPlugin {
		write(t, filepath.Join(keg, "share", "contextfill", "contextfill.js"), "export default {}\n")
	}
	for link, target := range map[string]string{
		filepath.Join(prefix, "bin", "contextfill"): filepath.Join("..", "Cellar", "contextfill", "0.2.0", "bin", "contextfill"),
		filepath.Join(prefix, "opt", "contextfill"): filepath.Join("..", "Cellar", "contextfill", "0.2.0"),
	} {
		if err := os.MkdirAll(filepath.Dir(link), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.Symlink(target, link); err != nil {
			t.Fatal(err)
		}
	}
	saved := executable
	executable = func() (string, error) { return binary, nil }
	t.Cleanup(func() { executable = saved })
	return filepath.Join(prefix, "bin", "contextfill")
}

// apps makes a temporary HOME holding the config dirs of the named apps.
func apps(t *testing.T, claude, opencode bool) string {
	t.Helper()
	home := t.TempDir()
	if claude {
		if err := os.MkdirAll(filepath.Join(home, ".claude"), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	if opencode {
		if err := os.MkdirAll(filepath.Join(home, ".config", "opencode"), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	return home
}

// hookCommands is every PostToolUse command in the settings file.
func hookCommands(t *testing.T, home string) []string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(home, ".claude", "settings.json"))
	if err != nil {
		return nil
	}
	var s struct {
		Hooks struct {
			PostToolUse []struct {
				Hooks []struct {
					Command string `json:"command"`
				} `json:"hooks"`
			} `json:"PostToolUse"`
		} `json:"hooks"`
	}
	if err := json.Unmarshal(data, &s); err != nil {
		t.Fatal(err)
	}
	var out []string
	for _, g := range s.Hooks.PostToolUse {
		for _, h := range g.Hooks {
			out = append(out, h.Command)
		}
	}
	return out
}

func pluginLink(home string) string {
	return filepath.Join(home, ".config", "opencode", "plugins", "contextfill.js")
}

func TestHookInstallForClaudeCodeNamesTheStablePath(t *testing.T) {
	bin := homebrew(t, true)
	home := apps(t, true, false)
	code, out, errOut := invoke(map[string]string{"HOME": home}, "hook", "install", "--harness", "claude-code")
	if code != 0 {
		t.Fatalf("exit %d, stderr %q", code, errOut)
	}
	want := bin + " hook --max 40 --main-max 33"
	if got := hookCommands(t, home); len(got) != 1 || got[0] != want {
		t.Errorf("hooks = %v, want [%s]", got, want)
	}
	if lines := strings.Split(strings.TrimSpace(out), "\n"); len(lines) != 1 || !strings.Contains(out, want) {
		t.Errorf("stdout = %q, want one line naming the command", out)
	}
}

func TestHookInstallTwiceSaysNothingChanged(t *testing.T) {
	homebrew(t, true)
	home := apps(t, true, false)
	invoke(map[string]string{"HOME": home}, "hook", "install", "--harness", "claude-code")
	code, out, _ := invoke(map[string]string{"HOME": home}, "hook", "install", "--harness", "claude-code")
	if code != 0 || !strings.Contains(out, "nothing changed") {
		t.Errorf("exit %d, stdout %q; want it to say nothing changed", code, out)
	}
	if got := hookCommands(t, home); len(got) != 1 {
		t.Errorf("hooks = %v, want one", got)
	}
}

func TestHookInstallPassesTheLimitsThrough(t *testing.T) {
	bin := homebrew(t, true)
	home := apps(t, true, false)
	code, _, errOut := invoke(map[string]string{"HOME": home}, "hook", "install", "--harness", "claude-code", "--max", "50", "--main-max=30")
	if code != 0 {
		t.Fatalf("exit %d, stderr %q", code, errOut)
	}
	if got, want := hookCommands(t, home), bin+" hook --max 50 --main-max 30"; len(got) != 1 || got[0] != want {
		t.Errorf("hooks = %v, want [%s]", got, want)
	}
}

func TestHookInstallReplacesAStaleCopy(t *testing.T) {
	bin := homebrew(t, true)
	home := apps(t, true, false)
	write(t, filepath.Join(home, ".claude", "settings.json"),
		`{"model": "opus", "hooks": {"PostToolUse": [{"matcher": "Bash", "hooks": [{"type": "command", "command": "`+home+`/go/bin/contextfill hook --max 40", "timeout": 10}]}]}}`)
	code, out, errOut := invoke(map[string]string{"HOME": home}, "hook", "install", "--harness", "claude-code")
	if code != 0 {
		t.Fatalf("exit %d, stderr %q", code, errOut)
	}
	if got, want := hookCommands(t, home), bin+" hook --max 40 --main-max 33"; len(got) != 1 || got[0] != want {
		t.Errorf("hooks = %v, want [%s]", got, want)
	}
	if !strings.Contains(out, home+"/go/bin/contextfill") {
		t.Errorf("stdout = %q, want it to name what it replaced", out)
	}
}

func TestHookInstallRefusesABrokenSettingsFile(t *testing.T) {
	homebrew(t, true)
	home := apps(t, true, false)
	path := filepath.Join(home, ".claude", "settings.json")
	write(t, path, `{"model": `)
	code, _, errOut := invoke(map[string]string{"HOME": home}, "hook", "install", "--harness", "claude-code")
	if code != exitTarget || !strings.Contains(errOut, path) {
		t.Errorf("exit %d, stderr %q; want 4 naming %s", code, errOut, path)
	}
	if data, _ := os.ReadFile(path); string(data) != `{"model": ` {
		t.Errorf("file = %q, want it untouched", data)
	}
}

func TestHookInstallHonorsClaudeConfigDir(t *testing.T) {
	homebrew(t, true)
	home := apps(t, false, false)
	config := t.TempDir()
	code, _, errOut := invoke(map[string]string{"HOME": home, "CLAUDE_CONFIG_DIR": config}, "hook", "install", "--harness", "claude")
	if code != 0 {
		t.Fatalf("exit %d, stderr %q", code, errOut)
	}
	if _, err := os.Stat(filepath.Join(config, "settings.json")); err != nil {
		t.Errorf("no settings.json under CLAUDE_CONFIG_DIR: %v", err)
	}
}

func TestHookInstallForOpenCodeLinksThePlugin(t *testing.T) {
	homebrew(t, true)
	home := apps(t, false, true)
	code, out, errOut := invoke(map[string]string{"HOME": home}, "hook", "install", "--harness", "opencode")
	if code != 0 {
		t.Fatalf("exit %d, stderr %q", code, errOut)
	}
	target, err := os.Readlink(pluginLink(home))
	if err != nil || !strings.HasSuffix(target, "/opt/contextfill/share/contextfill/contextfill.js") {
		t.Errorf("link -> %q, %v; want the opt path", target, err)
	}
	if !strings.Contains(out, "opencode reload") {
		t.Errorf("stdout = %q, want it to say how to load the plugin", out)
	}
	if hookCommands(t, home) != nil {
		t.Errorf("--harness opencode touched Claude Code's settings")
	}
}

func TestHookInstallForOpenCodeWithoutAPluginExitsZero(t *testing.T) {
	homebrew(t, false)
	home := apps(t, false, true)
	code, out, errOut := invoke(map[string]string{"HOME": home}, "hook", "install", "--harness", "opencode")
	if code != 0 {
		t.Fatalf("exit %d, stderr %q; bootstrap reads non-zero as a failure", code, errOut)
	}
	if !strings.Contains(out, "plugin") || !strings.Contains(out, "nothing changed") {
		t.Errorf("stdout = %q, want a plain line saying the plugin is missing", out)
	}
	if _, err := os.Lstat(pluginLink(home)); err == nil {
		t.Errorf("linked a plugin that isn't there")
	}
}

func TestHookInstallWithoutAHarnessDoesEachPresentApp(t *testing.T) {
	homebrew(t, true)
	home := apps(t, true, true)
	code, out, errOut := invoke(map[string]string{"HOME": home}, "hook", "install")
	if code != 0 {
		t.Fatalf("exit %d, stderr %q", code, errOut)
	}
	if len(hookCommands(t, home)) != 1 {
		t.Errorf("Claude Code got no hook")
	}
	if _, err := os.Readlink(pluginLink(home)); err != nil {
		t.Errorf("OpenCode got no plugin: %v", err)
	}
	if lines := strings.Split(strings.TrimSpace(out), "\n"); len(lines) != 2 {
		t.Errorf("stdout = %q, want one line per app", out)
	}
}

func TestHookInstallWithoutAHarnessSkipsAnAbsentApp(t *testing.T) {
	homebrew(t, true)
	home := apps(t, true, false)
	code, _, errOut := invoke(map[string]string{"HOME": home}, "hook", "install")
	if code != 0 {
		t.Fatalf("exit %d, stderr %q", code, errOut)
	}
	if _, err := os.Stat(filepath.Join(home, ".config", "opencode")); err == nil {
		t.Errorf("made OpenCode's config dir though OpenCode isn't here")
	}
}

func TestHookInstallWithNoAppExitsFive(t *testing.T) {
	homebrew(t, true)
	home := apps(t, false, false)
	code, _, errOut := invoke(map[string]string{"HOME": home}, "hook", "install")
	if code != exitEnvironment || !strings.Contains(errOut, "--harness") {
		t.Errorf("exit %d, stderr %q; want 5 pointing at --harness", code, errOut)
	}
}

func TestHookUninstallRemovesOnlyItsOwn(t *testing.T) {
	homebrew(t, true)
	home := apps(t, true, true)
	write(t, filepath.Join(home, ".claude", "settings.json"),
		`{"hooks": {"PostToolUse": [{"hooks": [{"type": "command", "command": "/usr/local/bin/status"}]}]}}`)
	invoke(map[string]string{"HOME": home}, "hook", "install")
	code, out, errOut := invoke(map[string]string{"HOME": home}, "hook", "uninstall")
	if code != 0 {
		t.Fatalf("exit %d, stderr %q", code, errOut)
	}
	if got := hookCommands(t, home); len(got) != 1 || got[0] != "/usr/local/bin/status" {
		t.Errorf("hooks = %v, want only the other tool's", got)
	}
	if _, err := os.Lstat(pluginLink(home)); err == nil {
		t.Errorf("the plugin link is still there")
	}
	if lines := strings.Split(strings.TrimSpace(out), "\n"); len(lines) != 2 {
		t.Errorf("stdout = %q, want one line per app", out)
	}
}

func TestHookInstallUsageErrorsExitTwo(t *testing.T) {
	for _, args := range [][]string{
		{"hook", "install", "--agents"},
		{"hook", "install", "--json"},
		{"hook", "install", "extra"},
		{"hook", "install", "uninstall"},
		{"hook", "install", "--harness", "cursor"},
		{"hook", "install", "--max", "0"},
	} {
		if code, _, _ := invoke(map[string]string{"HOME": t.TempDir()}, args...); code != exitUsage {
			t.Errorf("%v exited %d, want 2", args, code)
		}
	}
}

func TestHookInstallHelpNeedsNothing(t *testing.T) {
	code, out, _ := invoke(nil, "hook", "install", "--help")
	if code != 0 || !strings.Contains(out, "hook install") || !strings.Contains(out, "--harness") {
		t.Errorf("exit %d, stdout %q", code, out)
	}
}

func TestHelpDocumentsHookInstall(t *testing.T) {
	_, out, _ := invoke(nil, "--help")
	if !strings.Contains(out, "contextfill hook install") || !strings.Contains(out, "contextfill hook uninstall") {
		t.Errorf("--help doesn't mention hook install and uninstall")
	}
}

// The hook every existing settings.json runs: flags, then a payload on stdin.
func TestPlainHookWithFlagsStillReadsItsPayload(t *testing.T) {
	payload := `{"transcript_path": "/nowhere.jsonl", "tool_name": "Bash", "tool_input": {"command": "git status"}}`
	code, out, errOut := invokeWithInput(nil, payload, "hook", "--max", "40", "--main-max", "33")
	if code != 0 || out != "" {
		t.Errorf("exit %d, stdout %q, stderr %q; want a silent 0 for a non-commit", code, out, errOut)
	}
}
