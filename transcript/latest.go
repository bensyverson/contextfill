package transcript

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
)

// entry is the part of a transcript line this package reads.
type entry struct {
	Type           string `json:"type"`
	IsSidechain    bool   `json:"isSidechain"`
	RequestedModel string `json:"requestedModel"`
	Message        struct {
		Model string `json:"model"`
		Usage struct {
			InputTokens              int `json:"input_tokens"`
			CacheReadInputTokens     int `json:"cache_read_input_tokens"`
			CacheCreationInputTokens int `json:"cache_creation_input_tokens"`
		} `json:"usage"`
	} `json:"message"`
}

// used is the context a turn was sent: fresh input plus what was read from
// and written to the prompt cache.
func (e entry) used() int {
	u := e.Message.Usage
	return u.InputTokens + u.CacheReadInputTokens + u.CacheCreationInputTokens
}

// Latest reads the session transcript at path and returns the reading of its
// most recent main-thread assistant turn, skipping any sidechain turns a
// subagent left in it. Lines that aren't JSON and turns with no usage are
// skipped; a transcript with no usable turn is ErrNoUsage.
func Latest(path string) (Reading, error) {
	return latest(path, false)
}

// latest is Latest, optionally counting sidechain turns: a subagent's own
// transcript marks every turn as a sidechain of its parent.
func latest(path string, sidechains bool) (Reading, error) {
	f, err := os.Open(path)
	if err != nil {
		return Reading{}, fmt.Errorf("opening transcript: %w", err)
	}
	defer f.Close()

	var latest *entry
	r := bufio.NewReader(f)
	for {
		line, readErr := r.ReadBytes('\n')
		if len(line) > 0 {
			var e entry
			if json.Unmarshal(line, &e) == nil && e.Type == "assistant" && (sidechains || !e.IsSidechain) && e.used() > 0 {
				latest = &e
			}
		}
		if errors.Is(readErr, io.EOF) {
			break
		}
		if readErr != nil {
			return Reading{}, fmt.Errorf("reading %s: %w", path, readErr)
		}
	}
	if latest == nil {
		return Reading{}, fmt.Errorf("%s: %w", path, ErrNoUsage)
	}
	model := latest.RequestedModel
	if model == "" {
		model = latest.Message.Model
	}
	return Reading{Model: model, Used: latest.used(), Window: WindowFor(model)}, nil
}
