// Package hook answers Claude Code's PostToolUse hook: after a Bash call that
// commits, it tells the model how full its context window is.
package hook

import (
	"encoding/json"
	"fmt"
	"io"
	"strconv"

	"github.com/bensyverson/contextfill/transcript"
)

// input is the part of a PostToolUse payload this package reads. A
// subagent's payload carries its parent's transcript_path plus its own
// agent_id.
type input struct {
	TranscriptPath string `json:"transcript_path"`
	AgentID        string `json:"agent_id"`
	ToolName       string `json:"tool_name"`
	ToolInput      struct {
		Command string `json:"command"`
	} `json:"tool_input"`
}

// response is the hook's JSON answer; additionalContext reaches the model.
type response struct {
	HookSpecificOutput struct {
		HookEventName     string `json:"hookEventName"`
		AdditionalContext string `json:"additionalContext"`
	} `json:"hookSpecificOutput"`
}

// Respond reads a PostToolUse payload and, when its Bash command commits,
// returns the JSON that hands the model its context fill: the subagent's own
// when the payload names one, else the main thread's. A maxPercent above 0
// adds handoff advice once the fill passes it. Any other tool call gets no
// output.
func Respond(r io.Reader, maxPercent float64) ([]byte, error) {
	var in input
	if err := json.NewDecoder(r).Decode(&in); err != nil {
		return nil, fmt.Errorf("reading the hook payload: %w", err)
	}
	if in.ToolName != "Bash" || !Commits(in.ToolInput.Command) {
		return nil, nil
	}
	var (
		reading transcript.Reading
		err     error
	)
	if in.AgentID != "" {
		reading, err = transcript.LatestOfAgent(in.TranscriptPath, in.AgentID)
	} else {
		reading, err = transcript.Latest(in.TranscriptPath)
	}
	if err != nil {
		return nil, err
	}

	text := fmt.Sprintf("contextfill: %s · %s tokens in context after this commit.",
		reading.Summary(), transcript.Tokens(reading.Used))
	if maxPercent > 0 && reading.Fill()*100 > maxPercent {
		text += fmt.Sprintf(" That is over the %s%% limit: finish what's in flight and write the handoff rather than starting new work.",
			strconv.FormatFloat(maxPercent, 'f', -1, 64))
	}
	var out response
	out.HookSpecificOutput.HookEventName = "PostToolUse"
	out.HookSpecificOutput.AdditionalContext = text
	return json.Marshal(out)
}
