package hook

import (
	"encoding/json"
	"fmt"
	"io"

	"github.com/bensyverson/contextfill/fill"
)

// openCodeShell is the name OpenCode gives its shell tool.
const openCodeShell = "shell"

// Sessions is what answering OpenCode needs of its reader.
type Sessions interface {
	Latest(sessionID string) (fill.Reading, error)
	// IsSubagent reports whether the session is another session's child.
	IsSubagent(sessionID string) (bool, error)
}

// openCodeInput is what the OpenCode plugin pipes in: the finished tool
// call's own fields, under OpenCode's names.
type openCodeInput struct {
	Tool      string `json:"tool"`
	SessionID string `json:"sessionID"`
	Input     struct {
		Command string `json:"command"`
	} `json:"input"`
}

// RespondOpenCode reads a finished OpenCode tool call and, when it is a shell
// command that commits, returns the text that tells the model its session's
// fill. A subagent's session is judged against limits.Max, the main
// session's against MainMax. Any other tool call gets "".
func RespondOpenCode(r io.Reader, limits Limits, sessions Sessions) (string, error) {
	var in openCodeInput
	if err := json.NewDecoder(r).Decode(&in); err != nil {
		return "", fmt.Errorf("reading the plugin's payload: %w", err)
	}
	if in.Tool != openCodeShell || !Commits(in.Input.Command) {
		return "", nil
	}
	reading, err := sessions.Latest(in.SessionID)
	if err != nil {
		return "", err
	}
	subagent, err := sessions.IsSubagent(in.SessionID)
	if err != nil {
		return "", err
	}
	return message(reading, limits.For(subagent)), nil
}
