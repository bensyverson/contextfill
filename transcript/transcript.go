// Package transcript reads Claude Code session transcripts to report how full
// a session's context window is.
//
// A transcript is a JSONL file under the projects directory
// (~/.claude/projects/<project>/<session id>.jsonl). Each assistant turn
// records the tokens it was sent; the input side of the most recent turn is
// the context the session now carries.
package transcript

import (
	"errors"
	"fmt"
	"path/filepath"
)

var (
	// ErrNoTranscript means no transcript exists for the session id.
	ErrNoTranscript = errors.New("no transcript for this session")
	// ErrNoUsage means the transcript holds no assistant turn with token usage.
	ErrNoUsage = errors.New("no assistant turn with token usage yet")
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
		return "", fmt.Errorf("session %s under %s: %w", sessionID, projectsDir, ErrNoTranscript)
	}
	return matches[0], nil
}
