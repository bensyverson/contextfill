package install

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestInstallClaudeCreatesAMissingFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), ".claude", "settings.json")
	changes, err := InstallClaude(path, command)
	if err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if got, want := commands(t, data), []string{"Bash: " + command}; !reflect.DeepEqual(got, want) {
		t.Errorf("PostToolUse = %v, want %v", got, want)
	}
	if len(changes) != 1 || changes[0].Kind != Added || changes[0].Path != path {
		t.Errorf("changes = %+v, want one Added naming %s", changes, path)
	}
}

func TestInstallClaudeRefusesInvalidJSONWithoutWriting(t *testing.T) {
	path := filepath.Join(t.TempDir(), "settings.json")
	broken := `{"model": "opus",`
	if err := os.WriteFile(path, []byte(broken), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := InstallClaude(path, command); !errors.Is(err, ErrInvalidSettings) {
		t.Fatalf("error = %v, want ErrInvalidSettings", err)
	}
	if data, _ := os.ReadFile(path); string(data) != broken {
		t.Errorf("file = %q, want it untouched", data)
	}
	entries, _ := os.ReadDir(filepath.Dir(path))
	if len(entries) != 1 {
		t.Errorf("directory holds %d entries, want no temp file left behind", len(entries))
	}
}

func TestInstallClaudeKeepsTheFilesMode(t *testing.T) {
	path := filepath.Join(t.TempDir(), "settings.json")
	if err := os.WriteFile(path, []byte(existing), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := InstallClaude(path, command); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Errorf("mode = %v, want 0600 kept", info.Mode().Perm())
	}
	data, _ := os.ReadFile(path)
	if got := commands(t, data); len(got) != 2 {
		t.Errorf("PostToolUse = %v, want the hook added", got)
	}
}

func TestInstallClaudeWritesThroughASymlink(t *testing.T) {
	dir := t.TempDir()
	real := filepath.Join(dir, "dotfiles", "settings.json")
	write(t, real, existing)
	link := filepath.Join(dir, ".claude", "settings.json")
	if err := os.MkdirAll(filepath.Dir(link), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(real, link); err != nil {
		t.Fatal(err)
	}
	if _, err := InstallClaude(link, command); err != nil {
		t.Fatal(err)
	}
	if target, err := os.Readlink(link); err != nil || target != real {
		t.Errorf("link = %q, %v; want it still pointing at %s", target, err, real)
	}
	data, _ := os.ReadFile(real)
	if got := commands(t, data); len(got) != 2 {
		t.Errorf("the link's target holds %v, want the hook added there", got)
	}
}

func TestInstallClaudeTwiceLeavesTheFileAlone(t *testing.T) {
	path := filepath.Join(t.TempDir(), "settings.json")
	write(t, path, existing)
	if _, err := InstallClaude(path, command); err != nil {
		t.Fatal(err)
	}
	before, _ := os.Stat(path)
	changes, err := InstallClaude(path, command)
	if err != nil {
		t.Fatal(err)
	}
	after, _ := os.Stat(path)
	if !os.SameFile(before, after) {
		t.Errorf("a second install rewrote the file")
	}
	if len(changes) != 1 || changes[0].Kind != Unchanged {
		t.Errorf("changes = %+v, want one Unchanged", changes)
	}
}

func TestUninstallClaudeWithNoFileChangesNothing(t *testing.T) {
	path := filepath.Join(t.TempDir(), "settings.json")
	changes, err := UninstallClaude(path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(path); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("uninstall created %s", path)
	}
	if len(changes) != 1 || changes[0].Kind != Unchanged {
		t.Errorf("changes = %+v, want one Unchanged", changes)
	}
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
