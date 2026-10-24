package opencode

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"syscall"
	"time"

	"github.com/bensyverson/contextfill/fill"
)

// timeout bounds each request; the server is local, so a slow answer means
// it's stuck.
const timeout = 5 * time.Second

// Client is a fill.Source over a running OpenCode server.
type Client struct {
	service Service
	http    *http.Client
	// windows caches each directory's model windows for the client's life.
	windows map[string]map[modelKey]int
}

// NewClient returns a client for the server the service describes.
func NewClient(s Service) *Client {
	return &Client{service: s, http: &http.Client{Timeout: timeout}, windows: map[string]map[modelKey]int{}}
}

// The wire shapes below are the parts of the server's answers this package
// reads. Every answer wraps its payload in "data".

type sessionInfo struct {
	ID       string    `json:"id"`
	ParentID string    `json:"parentID"`
	Title    string    `json:"title"`
	Model    modelRef  `json:"model"`
	Location location  `json:"location"`
	Time     timeStamp `json:"time"`
}

type location struct {
	Directory string `json:"directory"`
}

type timeStamp struct {
	Updated int64 `json:"updated"`
	Idle    int64 `json:"idle"`
}

type modelRef struct {
	ID         string `json:"id"`
	ProviderID string `json:"providerID"`
}

// name is how OpenCode's own -m flag names a model: "provider/model".
func (m modelRef) name() string { return m.ProviderID + "/" + m.ID }

type modelKey struct{ provider, id string }

// message is an assistant step. Tokens is nil on a step that errored before
// the model answered; a present Tokens missing a count is a shape this
// package doesn't know.
type message struct {
	Model  modelRef `json:"model"`
	Tokens *struct {
		Input *int `json:"input"`
		Cache *struct {
			Read  *int `json:"read"`
			Write *int `json:"write"`
		} `json:"cache"`
	} `json:"tokens"`
}

type modelInfo struct {
	ID         string `json:"id"`
	ProviderID string `json:"providerID"`
	Limit      struct {
		Context int `json:"context"`
	} `json:"limit"`
}

// page is a list answer. Its cursor comes back even on the last page, so a
// list ends at an empty page.
type page[T any] struct {
	Data   []T `json:"data"`
	Cursor struct {
		Next string `json:"next"`
	} `json:"cursor"`
}

// get fetches path with the query and decodes the answer into out.
func (c *Client) get(path string, query url.Values, out any) error {
	u := c.service.URL + path
	if len(query) > 0 {
		u += "?" + query.Encode()
	}
	req, err := http.NewRequest(http.MethodGet, u, nil)
	if err != nil {
		return err
	}
	req.SetBasicAuth("opencode", c.service.Password)
	resp, err := c.http.Do(req)
	if errors.Is(err, syscall.ECONNREFUSED) {
		return fmt.Errorf("%w at %s; start it with: opencode service start", ErrNoServer, c.service.URL)
	}
	if err != nil {
		return fmt.Errorf("asking OpenCode's server: %w", err)
	}
	defer resp.Body.Close()
	switch resp.StatusCode {
	case http.StatusOK:
	case http.StatusUnauthorized:
		return fmt.Errorf("OpenCode's server refused the password in service.json; restart it with: opencode service restart")
	case http.StatusNotFound:
		var body struct {
			Tag string `json:"_tag"`
		}
		if json.NewDecoder(resp.Body).Decode(&body) == nil && body.Tag == "SessionNotFoundError" {
			return fill.ErrNoSession
		}
		return fmt.Errorf("OpenCode's server has no %s", path)
	default:
		return fmt.Errorf("OpenCode's server answered %s for %s", resp.Status, path)
	}
	if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
		return fmt.Errorf("reading OpenCode's answer for %s: %w", path, err)
	}
	return nil
}
