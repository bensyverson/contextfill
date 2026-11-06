package install

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/bensyverson/contextfill/fill"
)

// newFileMode is a new settings file's mode: it can hold secrets in env.
const newFileMode fs.FileMode = 0o600

// InstallClaude merges contextfill's hook, running command, into the
// settings file at path, creating it if it's missing. It refuses, writing
// nothing, a file that isn't valid settings JSON.
func InstallClaude(path, command string) ([]Change, error) {
	return editSettings(path, func(data []byte) ([]byte, []Change, error) { return MergeClaude(data, command) })
}

// UninstallClaude takes contextfill's hook out of the settings file at path.
func UninstallClaude(path string) ([]Change, error) {
	return editSettings(path, RemoveClaude)
}

// editSettings applies edit to the settings file, writing only when it
// changed something. A symlinked settings file is written through, so the
// link survives.
func editSettings(path string, edit func([]byte) ([]byte, []Change, error)) ([]Change, error) {
	real, err := filepath.EvalSymlinks(path)
	if errors.Is(err, fs.ErrNotExist) {
		real = path
	} else if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(real)
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return nil, err
	}
	missing := err != nil
	out, changes, err := edit(data)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	changes = stamp(changes, fill.ClaudeCode, path)
	if len(changes) == 1 && changes[0].Kind == Unchanged {
		return changes, nil
	}
	mode := newFileMode
	if !missing {
		info, err := os.Stat(real)
		if err != nil {
			return nil, err
		}
		mode = info.Mode().Perm()
	}
	if err := writeAtomic(real, out, mode); err != nil {
		return nil, err
	}
	return changes, nil
}

// writeAtomic writes data to a temporary file beside path, then renames it
// over path, so a reader sees the old file or the new one, never half.
func writeAtomic(path string, data []byte, mode fs.FileMode) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(dir, "."+filepath.Base(path)+".*")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name())
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Chmod(mode); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmp.Name(), path)
}
