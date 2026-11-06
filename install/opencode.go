package install

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/bensyverson/contextfill/fill"
)

// pluginName is the plugin's file name, in the formula's pkgshare and in
// OpenCode's plugins directory alike.
const pluginName = "contextfill.js"

// pluginCandidates are where the plugin may be, best first. For a Homebrew
// install the first is under the opt link, which an upgrade repoints, so a
// link to it survives the upgrade; the formula installs it to pkgshare.
func pluginCandidates(executable string) []string {
	real := resolve(executable)
	if prefix, keg, ok := cellarKeg(real); ok {
		return []string{
			filepath.Join(prefix, "opt", formula, "share", formula, pluginName),
			filepath.Join(keg, "share", formula, pluginName),
		}
	}
	return []string{filepath.Join(filepath.Dir(filepath.Dir(real)), "share", formula, pluginName)}
}

// FindPlugin returns the OpenCode plugin installed with the contextfill at
// executable, or ErrNoPlugin.
func FindPlugin(executable string) (string, error) {
	for _, path := range pluginCandidates(executable) {
		if _, err := os.Stat(path); err == nil {
			return path, nil
		}
	}
	return "", ErrNoPlugin
}

// InstallOpenCode links the plugin installed with the contextfill at
// executable to link. Without a plugin it changes nothing and says so in a
// Missing change, not an error: an older or a go-installed contextfill has
// none, and that isn't a failure of the Claude Code side or of the caller.
func InstallOpenCode(link, executable string) ([]Change, error) {
	plugin, err := FindPlugin(executable)
	if errors.Is(err, ErrNoPlugin) {
		return []Change{{Harness: fill.OpenCode, Kind: Missing, Path: link, New: pluginCandidates(executable)[0]}}, nil
	}
	return LinkPlugin(link, plugin)
}

// LinkPlugin makes link a symlink to plugin. A link elsewhere is repointed;
// a regular file there is a copy the person made, to change its limits, and
// is kept.
func LinkPlugin(link, plugin string) ([]Change, error) {
	info, err := os.Lstat(link)
	switch {
	case errors.Is(err, fs.ErrNotExist):
		if err := os.MkdirAll(filepath.Dir(link), 0o755); err != nil {
			return nil, err
		}
		if err := os.Symlink(plugin, link); err != nil {
			return nil, err
		}
		return openCodeChange(Linked, link, "", plugin), nil
	case err != nil:
		return nil, err
	case info.Mode()&fs.ModeSymlink == 0:
		return openCodeChange(Kept, link, "", plugin), nil
	}
	old, err := os.Readlink(link)
	if err != nil {
		return nil, err
	}
	if old == plugin {
		return openCodeChange(Unchanged, link, old, plugin), nil
	}
	// A new link renamed over the old one, so there's never a moment with
	// no plugin.
	tmp := filepath.Join(filepath.Dir(link), "."+pluginName+".new")
	os.Remove(tmp)
	if err := os.Symlink(plugin, tmp); err != nil {
		return nil, err
	}
	if err := os.Rename(tmp, link); err != nil {
		os.Remove(tmp)
		return nil, err
	}
	return openCodeChange(Replaced, link, old, plugin), nil
}

// UnlinkPlugin removes link if it is a symlink; a copy is kept.
func UnlinkPlugin(link string) ([]Change, error) {
	info, err := os.Lstat(link)
	switch {
	case errors.Is(err, fs.ErrNotExist):
		return openCodeChange(Unchanged, link, "", ""), nil
	case err != nil:
		return nil, err
	case info.Mode()&fs.ModeSymlink == 0:
		return openCodeChange(Kept, link, "", ""), nil
	}
	old, err := os.Readlink(link)
	if err != nil {
		return nil, err
	}
	if err := os.Remove(link); err != nil {
		return nil, err
	}
	return openCodeChange(Removed, link, old, ""), nil
}

func openCodeChange(k Kind, link, old, plugin string) []Change {
	return []Change{{Harness: fill.OpenCode, Kind: k, Path: link, Old: old, New: plugin}}
}
