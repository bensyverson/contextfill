package opencode

import (
	"errors"
	"fmt"
	"net/url"
	"slices"
	"strconv"
	"strings"

	"github.com/bensyverson/contextfill/fill"
)

// stepsPerPage is how many assistant steps to fetch at a time. The newest
// step almost always has a reading; a few failed ones in a row are rare.
const stepsPerPage = 5

// Latest returns the session's latest reading: the input side of its newest
// assistant step that has one, input + cache read + cache write. Steps that
// errored or were aborted carry no tokens and are skipped. A model the
// server doesn't know gives a reading with no window.
func (c *Client) Latest(sessionID string) (fill.Reading, error) {
	s, err := c.session(sessionID)
	if err != nil {
		return fill.Reading{}, err
	}
	return c.latest(s)
}

func (c *Client) latest(s sessionInfo) (fill.Reading, error) {
	path := "/api/session/" + url.PathEscape(s.ID) + "/message"
	query := url.Values{"type": {"assistant"}, "order": {"desc"}, "limit": {strconv.Itoa(stepsPerPage)}}
	for {
		var p page[message]
		if err := c.get(path, query, &p); err != nil {
			return fill.Reading{}, err
		}
		if len(p.Data) == 0 {
			return fill.Reading{}, fmt.Errorf("session %s: %w", s.ID, fill.ErrNoUsage)
		}
		for _, m := range p.Data {
			used, err := m.used()
			if err != nil {
				return fill.Reading{}, fmt.Errorf("session %s: %w", s.ID, err)
			}
			if used == 0 {
				continue
			}
			window, err := c.window(s.Location.Directory, m.Model)
			if err != nil {
				return fill.Reading{}, err
			}
			return fill.Reading{Model: m.Model.name(), Used: used, Window: window}, nil
		}
		// The cursor carries the order, and the server refuses both.
		query.Del("order")
		query.Set("cursor", p.Cursor.Next)
	}
}

// used is the step's input side, 0 for a step with no tokens.
func (m message) used() (int, error) {
	t := m.Tokens
	if t == nil {
		return 0, nil
	}
	if t.Input == nil || t.Cache == nil || t.Cache.Read == nil || t.Cache.Write == nil {
		return 0, errors.New("an assistant step has tokens without input or cache read and write counts; OpenCode's message shape has changed")
	}
	return *t.Input + *t.Cache.Read + *t.Cache.Write, nil
}

// Agents lists the session's subagents, the sessions whose parent it is, in
// id order. One is finished once it has gone idle since its last update.
func (c *Client) Agents(sessionID string) ([]fill.Agent, error) {
	if _, err := c.session(sessionID); err != nil {
		return nil, err
	}
	var children []sessionInfo
	query := url.Values{"parentID": {sessionID}}
	for {
		var p page[sessionInfo]
		if err := c.get("/api/session", query, &p); err != nil {
			return nil, err
		}
		if len(p.Data) == 0 {
			break
		}
		children = append(children, p.Data...)
		query.Set("cursor", p.Cursor.Next)
	}
	slices.SortFunc(children, func(a, b sessionInfo) int { return strings.Compare(a.ID, b.ID) })

	agents := make([]fill.Agent, 0, len(children))
	for _, child := range children {
		agent := fill.Agent{
			ID:          child.ID,
			Description: child.Title,
			Model:       child.Model.name(),
			Finished:    child.Time.Idle != 0 && child.Time.Idle >= child.Time.Updated,
		}
		reading, err := c.latest(child)
		switch {
		case err == nil:
			agent.Reading, agent.Model = reading, reading.Model
		case !errors.Is(err, fill.ErrNoUsage):
			return nil, err
		}
		agents = append(agents, agent)
	}
	return agents, nil
}

func (c *Client) session(id string) (sessionInfo, error) {
	var answer struct {
		Data sessionInfo `json:"data"`
	}
	if err := c.get("/api/session/"+url.PathEscape(id), nil, &answer); err != nil {
		if errors.Is(err, fill.ErrNoSession) {
			return sessionInfo{}, fmt.Errorf("session %s: %w", id, err)
		}
		return sessionInfo{}, err
	}
	return answer.Data, nil
}

// window returns the model's context window as the server knows it for a
// directory, whose config may add providers; 0 if it doesn't know the model.
func (c *Client) window(directory string, m modelRef) (int, error) {
	windows, ok := c.windows[directory]
	if !ok {
		var answer struct {
			Data []modelInfo `json:"data"`
		}
		if err := c.get("/api/model", url.Values{"location[directory]": {directory}}, &answer); err != nil {
			return 0, err
		}
		windows = map[modelKey]int{}
		for _, info := range answer.Data {
			windows[modelKey{info.ProviderID, info.ID}] = info.Limit.Context
		}
		c.windows[directory] = windows
	}
	return windows[modelKey{m.ProviderID, m.ID}], nil
}
