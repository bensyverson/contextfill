package install

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

// cellar lays out a Homebrew prefix holding contextfill at the version, with
// the bin and opt links Homebrew makes, and returns the prefix and the
// Cellar binary.
func cellar(t *testing.T, version string, plugin bool) (prefix, binary string) {
	t.Helper()
	prefix, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	keg := filepath.Join(prefix, "Cellar", "contextfill", version)
	binary = filepath.Join(keg, "bin", "contextfill")
	write(t, binary, "#!/bin/sh\n")
	if plugin {
		write(t, filepath.Join(keg, "share", "contextfill", "contextfill.js"), "export default {}\n")
	}
	symlink(t, filepath.Join("..", "Cellar", "contextfill", version, "bin", "contextfill"), filepath.Join(prefix, "bin", "contextfill"))
	symlink(t, filepath.Join("..", "Cellar", "contextfill", version), filepath.Join(prefix, "opt", "contextfill"))
	return prefix, binary
}

func symlink(t *testing.T, target, link string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(link), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(target, link); err != nil {
		t.Fatal(err)
	}
}

func TestStablePathPrefersHomebrewsBinLink(t *testing.T) {
	prefix, binary := cellar(t, "0.2.0", true)
	if got, want := StablePath(binary), filepath.Join(prefix, "bin", "contextfill"); got != want {
		t.Errorf("StablePath(%s) = %s, want %s", binary, got, want)
	}
}

func TestStablePathFallsBackToOptWhenBinIsSomeoneElses(t *testing.T) {
	prefix, binary := cellar(t, "0.2.0", true)
	bin := filepath.Join(prefix, "bin", "contextfill")
	os.Remove(bin)
	write(t, bin, "another contextfill\n")
	if got, want := StablePath(binary), filepath.Join(prefix, "opt", "contextfill", "bin", "contextfill"); got != want {
		t.Errorf("StablePath(%s) = %s, want %s", binary, got, want)
	}
}

func TestStablePathFollowsALinkIntoTheCellar(t *testing.T) {
	prefix, _ := cellar(t, "0.2.0", true)
	bin := filepath.Join(prefix, "bin", "contextfill")
	if got := StablePath(bin); got != bin {
		t.Errorf("StablePath(%s) = %s, want it kept", bin, got)
	}
}

func TestStablePathKeepsABinaryOutsideHomebrew(t *testing.T) {
	binary := filepath.Join(t.TempDir(), "go", "bin", "contextfill")
	write(t, binary, "#!/bin/sh\n")
	if got := StablePath(binary); got != binary {
		t.Errorf("StablePath(%s) = %s, want it kept", binary, got)
	}
}

func TestFindPluginPrefersTheOptPath(t *testing.T) {
	prefix, binary := cellar(t, "0.2.0", true)
	got, err := FindPlugin(binary)
	if err != nil {
		t.Fatal(err)
	}
	if want := filepath.Join(prefix, "opt", "contextfill", "share", "contextfill", "contextfill.js"); got != want {
		t.Errorf("FindPlugin = %s, want %s, which survives an upgrade", got, want)
	}
}

func TestFindPluginBesideABinaryOutsideHomebrew(t *testing.T) {
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	binary := filepath.Join(root, "bin", "contextfill")
	write(t, binary, "#!/bin/sh\n")
	plugin := filepath.Join(root, "share", "contextfill", "contextfill.js")
	write(t, plugin, "export default {}\n")
	if got, err := FindPlugin(binary); err != nil || got != plugin {
		t.Errorf("FindPlugin = %s, %v; want %s", got, err, plugin)
	}
}

func TestFindPluginSaysWhenThereIsNone(t *testing.T) {
	_, binary := cellar(t, "0.1.3", false)
	if _, err := FindPlugin(binary); !errors.Is(err, ErrNoPlugin) {
		t.Errorf("error = %v, want ErrNoPlugin", err)
	}
}

func TestLinkPluginLinksThenChangesNothing(t *testing.T) {
	_, binary := cellar(t, "0.2.0", true)
	plugin, _ := FindPlugin(binary)
	link := filepath.Join(t.TempDir(), "opencode", "plugins", "contextfill.js")

	changes, err := LinkPlugin(link, plugin)
	if err != nil {
		t.Fatal(err)
	}
	if target, _ := os.Readlink(link); target != plugin {
		t.Errorf("link -> %q, want %s", target, plugin)
	}
	if len(changes) != 1 || changes[0].Kind != Linked || changes[0].Path != link {
		t.Errorf("changes = %+v, want one Linked", changes)
	}

	changes, err = LinkPlugin(link, plugin)
	if err != nil {
		t.Fatal(err)
	}
	if len(changes) != 1 || changes[0].Kind != Unchanged {
		t.Errorf("second link: changes = %+v, want one Unchanged", changes)
	}
}

func TestLinkPluginReplacesALinkToAnotherCopy(t *testing.T) {
	_, binary := cellar(t, "0.2.0", true)
	plugin, _ := FindPlugin(binary)
	dir := t.TempDir()
	old := filepath.Join(dir, "clone", "opencode", "plugin", "contextfill.js")
	write(t, old, "old\n")
	link := filepath.Join(dir, "plugins", "contextfill.js")
	symlink(t, old, link)

	changes, err := LinkPlugin(link, plugin)
	if err != nil {
		t.Fatal(err)
	}
	if target, _ := os.Readlink(link); target != plugin {
		t.Errorf("link -> %q, want %s", target, plugin)
	}
	if len(changes) != 1 || changes[0].Kind != Replaced || changes[0].Old != old {
		t.Errorf("changes = %+v, want one Replaced naming %s", changes, old)
	}
}

func TestLinkPluginKeepsACopyThePersonMade(t *testing.T) {
	_, binary := cellar(t, "0.2.0", true)
	plugin, _ := FindPlugin(binary)
	link := filepath.Join(t.TempDir(), "plugins", "contextfill.js")
	write(t, link, "// my limits\n")

	changes, err := LinkPlugin(link, plugin)
	if err != nil {
		t.Fatal(err)
	}
	if data, _ := os.ReadFile(link); string(data) != "// my limits\n" {
		t.Errorf("the copy was overwritten: %q", data)
	}
	if len(changes) != 1 || changes[0].Kind != Kept {
		t.Errorf("changes = %+v, want one Kept", changes)
	}
}

func TestUnlinkPluginRemovesOnlyALink(t *testing.T) {
	_, binary := cellar(t, "0.2.0", true)
	plugin, _ := FindPlugin(binary)
	dir := t.TempDir()
	link := filepath.Join(dir, "plugins", "contextfill.js")
	symlink(t, plugin, link)

	changes, err := UnlinkPlugin(link)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Lstat(link); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("link still there: %v", err)
	}
	if len(changes) != 1 || changes[0].Kind != Removed {
		t.Errorf("changes = %+v, want one Removed", changes)
	}

	copied := filepath.Join(dir, "other", "contextfill.js")
	write(t, copied, "// my limits\n")
	if changes, err := UnlinkPlugin(copied); err != nil || len(changes) != 1 || changes[0].Kind != Kept {
		t.Errorf("unlinking a copy: changes = %+v, %v; want one Kept", changes, err)
	}
	if changes, err := UnlinkPlugin(filepath.Join(dir, "none.js")); err != nil || len(changes) != 1 || changes[0].Kind != Unchanged {
		t.Errorf("unlinking nothing: changes = %+v, %v; want one Unchanged", changes, err)
	}
}
