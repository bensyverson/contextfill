package opencode_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"github.com/bensyverson/contextfill/opencode"
)

const password = "synthetic-password"

// session is one session as the fake server holds it. Steps are its
// assistant messages, oldest first, as raw JSON.
type session struct {
	ID, ParentID, Title, Agent, Directory string
	// Provider and Model are the model the session runs.
	Provider, Model string
	Updated, Idle   int64
	Steps           []string
}

// step renders an assistant message with the given token counts.
func step(provider, model string, input, cacheRead, cacheWrite int) string {
	return mustJSON(map[string]any{
		"type": "assistant", "finish": "stop",
		"model":  map[string]any{"id": model, "providerID": provider, "variant": "default"},
		"tokens": map[string]any{"input": input, "output": 40, "reasoning": 0, "cache": map[string]any{"read": cacheRead, "write": cacheWrite}},
	})
}

// failedStep is an assistant message that errored before the model answered.
func failedStep(withTokens bool) string {
	m := map[string]any{"type": "assistant", "finish": "error", "error": map[string]any{"name": "APIError"}}
	if withTokens {
		m["tokens"] = map[string]any{"input": 0, "output": 0, "reasoning": 0, "cache": map[string]any{"read": 0, "write": 0}}
	}
	return mustJSON(m)
}

func mustJSON(v any) string {
	b, err := json.Marshal(v)
	if err != nil {
		panic(err)
	}
	return string(b)
}

// model is one entry of the server's model list.
type model struct {
	Provider, ID string
	Context      int
}

// fake is an OpenCode server holding the given sessions and models. It
// pages message lists two at a time, so paging is exercised.
type fake struct {
	sessions map[string]session
	models   map[string][]model // by directory
	// modelQueries records the location[directory] of each model list call.
	modelQueries []string
}

func serve(t *testing.T, f *fake) opencode.Service {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if user, pass, ok := r.BasicAuth(); !ok || user != "opencode" || pass != password {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		f.handle(t, w, r)
	}))
	t.Cleanup(srv.Close)
	return opencode.Service{URL: srv.URL, Password: password}
}

func (f *fake) handle(t *testing.T, w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	parts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
	switch {
	case r.URL.Path == "/api/model":
		dir := q.Get("location[directory]")
		f.modelQueries = append(f.modelQueries, dir)
		var data []map[string]any
		for _, m := range f.models[dir] {
			data = append(data, map[string]any{"id": m.ID, "modelID": m.ID, "providerID": m.Provider,
				"limit": map[string]any{"context": m.Context, "output": 8192}})
		}
		write(w, map[string]any{"location": map[string]any{"directory": dir}, "data": data})
	case r.URL.Path == "/api/session":
		var data []map[string]any
		for _, s := range f.sessions {
			if s.ParentID == q.Get("parentID") && s.ParentID != "" {
				data = append(data, s.wire())
			}
		}
		if q.Get("cursor") != "" {
			data = nil
		}
		write(w, map[string]any{"data": data, "cursor": map[string]any{"next": "more"}})
	case len(parts) >= 3 && parts[0] == "api" && parts[1] == "session":
		s, ok := f.sessions[parts[2]]
		if !ok {
			w.WriteHeader(http.StatusNotFound)
			write(w, map[string]any{"_tag": "SessionNotFoundError", "sessionID": parts[2], "message": "Session not found: " + parts[2]})
			return
		}
		if len(parts) == 3 {
			write(w, map[string]any{"data": s.wire()})
			return
		}
		// Like the real server, a cursor carries its own order and refuses
		// another.
		if q.Get("cursor") != "" && q.Has("order") {
			w.WriteHeader(http.StatusBadRequest)
			write(w, map[string]any{"_tag": "InvalidCursorError", "message": "Cursor cannot be combined with order"})
			return
		}
		if parts[3] != "message" || q.Get("type") != "assistant" || (q.Get("cursor") == "" && q.Get("order") != "desc") {
			t.Errorf("unexpected request %s", r.URL)
		}
		start, _ := strconv.Atoi(q.Get("cursor"))
		var page []json.RawMessage
		for i := len(s.Steps) - 1 - start; i >= 0 && len(page) < 2; i-- {
			page = append(page, json.RawMessage(s.Steps[i]))
		}
		// Like the real server, a cursor comes back even on the last page.
		write(w, map[string]any{"data": page, "cursor": map[string]any{"next": strconv.Itoa(start + 2)}})
	default:
		t.Errorf("unexpected request %s", r.URL)
		w.WriteHeader(http.StatusNotFound)
	}
}

func (s session) wire() map[string]any {
	m := map[string]any{"id": s.ID, "title": s.Title, "agent": s.Agent,
		"model":    map[string]any{"id": s.Model, "providerID": s.Provider, "variant": "default"},
		"location": map[string]any{"directory": s.Directory},
		"time":     map[string]any{"created": 1, "updated": s.Updated}}
	if s.ParentID != "" {
		m["parentID"] = s.ParentID
	}
	if s.Idle != 0 {
		m["time"].(map[string]any)["idle"] = s.Idle
	}
	return m
}

func write(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(v)
}
