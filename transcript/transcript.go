// Package transcript reads Claude Code session transcripts to report how full
// a session's context window is.
//
// A transcript is a JSONL file under the projects directory
// (~/.claude/projects/<project>/<session id>.jsonl). Each assistant turn
// records the tokens it was sent; the input side of the most recent turn is
// the context the session now carries.
package transcript

import (
	"fmt"
	"path/filepath"

	"github.com/bensyverson/contextfill/fill"
)

// Find returns the path of the session's transcript, searching every project
// directory under projectsDir, so the caller's working directory doesn't
// matter.
func Find(projectsDir, sessionID string) (string, error) {
	matches, err := filepath.Glob(filepath.Join(projectsDir, "*", sessionID+".jsonl"))
	if err != nil {
		return "", fmt.Errorf("searching %s: %w", projectsDir, err)
	}
	if len(matches) == 0 {
		return "", fmt.Errorf("session %s under %s: %w", sessionID, projectsDir, fill.ErrNoSession)
	}
	return matches[0], nil
}

// Projects is a fill.Source over the transcripts under a projects directory,
// ~/.claude/projects by default.
type Projects struct {
	Dir string
}

// Latest returns the latest reading of the session's main thread.
func (p Projects) Latest(sessionID string) (fill.Reading, error) {
	path, err := Find(p.Dir, sessionID)
	if err != nil {
		return fill.Reading{}, err
	}
	return Latest(path)
}

// Agents lists the session's subagents.
func (p Projects) Agents(sessionID string) ([]fill.Agent, error) {
	path, err := Find(p.Dir, sessionID)
	if err != nil {
		return nil, err
	}
	return Agents(path)
}
