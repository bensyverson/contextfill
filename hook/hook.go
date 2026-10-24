// Package hook tells the model how full its context window is after a shell
// command that commits: in Claude Code as a PostToolUse hook, in OpenCode
// through the plugin in opencode/plugin.
package hook

import (
	"encoding/json"
	"fmt"
	"io"
	"strconv"

	"github.com/bensyverson/contextfill/fill"
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

// Limits are the fill percentages past which the hook advises a handoff. 0
// means no limit.
type Limits struct {
	// Max applies to subagents, and to the main thread when MainMax is 0.
	Max float64
	// MainMax applies to the main thread, the payload without an agent_id.
	MainMax float64
}

// For returns the limit for a subagent, or for the main thread.
func (l Limits) For(subagent bool) float64 {
	if !subagent && l.MainMax > 0 {
		return l.MainMax
	}
	return l.Max
}

// Respond reads a PostToolUse payload and, when its Bash command commits,
// returns the JSON that hands the model its context fill: the subagent's own
// when the payload names one, else the main thread's. Once the fill passes
// its limit, it adds handoff advice. Any other tool call gets no output.
func Respond(r io.Reader, limits Limits) ([]byte, error) {
	var in input
	if err := json.NewDecoder(r).Decode(&in); err != nil {
		return nil, fmt.Errorf("reading the hook payload: %w", err)
	}
	if in.ToolName != "Bash" || !Commits(in.ToolInput.Command) {
		return nil, nil
	}
	var (
		reading fill.Reading
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

	var out response
	out.HookSpecificOutput.HookEventName = "PostToolUse"
	out.HookSpecificOutput.AdditionalContext = message(reading, limits.For(in.AgentID != ""))
	return json.Marshal(out)
}

// message tells the model its fill after a commit, adding handoff advice once
// the fill passes maxPercent; 0 means no limit.
func message(reading fill.Reading, maxPercent float64) string {
	text := fmt.Sprintf("contextfill: %s · %s tokens in context after this commit.",
		reading.Summary(), fill.Tokens(reading.Used))
	if maxPercent > 0 && reading.Fill()*100 > maxPercent {
		text += fmt.Sprintf(" That is over the %s%% limit: finish what's in flight and write the handoff rather than starting new work.",
			strconv.FormatFloat(maxPercent, 'f', -1, 64))
	}
	return text
}
