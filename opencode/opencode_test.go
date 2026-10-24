package opencode_test

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/bensyverson/contextfill/fill"
	"github.com/bensyverson/contextfill/opencode"
)

var _ fill.Source = (*opencode.Client)(nil)

const (
	mainID = "ses_synthetic0main"
	dir    = "/Users/someone/git/App"
)

func TestLatestSumsTheInputSideOfTheNewestStepWithAReading(t *testing.T) {
	f := &fake{
		sessions: map[string]session{mainID: {ID: mainID, Directory: dir, Steps: []string{
			step("prov", "old-model", 9, 9, 9),
			step("prov", "big-model", 3, 11_392, 128),
			failedStep(true),
			failedStep(false),
		}}},
		models: map[string][]model{dir: {{"prov", "big-model", 1_048_576}, {"other", "big-model", 8}}},
	}
	reading, err := opencode.NewClient(serve(t, f)).Latest(mainID)
	if err != nil {
		t.Fatalf("Latest: %v", err)
	}
	want := fill.Reading{Model: "prov/big-model", Used: 11_523, Window: 1_048_576}
	if reading != want {
		t.Errorf("Latest = %+v, want %+v", reading, want)
	}
	if len(f.modelQueries) != 1 || f.modelQueries[0] != dir {
		t.Errorf("model list queried for %q, want the session's directory %q", f.modelQueries, dir)
	}
}

func TestAModelTheServerDoesNotKnowHasNoWindow(t *testing.T) {
	f := &fake{sessions: map[string]session{mainID: {ID: mainID, Directory: dir, Steps: []string{step("local", "mystery", 10, 0, 0)}}}}
	reading, err := opencode.NewClient(serve(t, f)).Latest(mainID)
	if err != nil {
		t.Fatalf("Latest: %v", err)
	}
	if reading.Used != 10 || reading.HasWindow() {
		t.Errorf("Latest = %+v, want 10 tokens and no window", reading)
	}
}

func TestASessionWithNoReadingIsErrNoUsage(t *testing.T) {
	f := &fake{sessions: map[string]session{mainID: {ID: mainID, Directory: dir, Steps: []string{failedStep(true), failedStep(false), failedStep(true)}}}}
	if _, err := opencode.NewClient(serve(t, f)).Latest(mainID); !errors.Is(err, fill.ErrNoUsage) {
		t.Errorf("Latest error = %v, want ErrNoUsage", err)
	}
}

func TestAnUnknownSessionIsErrNoSession(t *testing.T) {
	f := &fake{sessions: map[string]session{}}
	if _, err := opencode.NewClient(serve(t, f)).Latest("ses_nope"); !errors.Is(err, fill.ErrNoSession) {
		t.Errorf("Latest error = %v, want ErrNoSession", err)
	}
}

func TestTokensWithoutTheirCacheCountsAreAnError(t *testing.T) {
	partial := `{"type":"assistant","finish":"stop","model":{"id":"m","providerID":"p"},"tokens":{"input":5,"output":1,"reasoning":0}}`
	f := &fake{sessions: map[string]session{mainID: {ID: mainID, Directory: dir, Steps: []string{partial}}}}
	_, err := opencode.NewClient(serve(t, f)).Latest(mainID)
	if err == nil || !strings.Contains(err.Error(), "cache") {
		t.Errorf("Latest error = %v, want one naming the missing cache counts", err)
	}
}

func TestAWrongPasswordNamesTheServiceFile(t *testing.T) {
	service := serve(t, &fake{})
	service.Password = "stale"
	_, err := opencode.NewClient(service).Latest(mainID)
	if err == nil || !strings.Contains(err.Error(), "service.json") {
		t.Errorf("Latest error = %v, want one naming service.json", err)
	}
}

func TestAServerThatIsDownIsErrNoServer(t *testing.T) {
	service := serve(t, &fake{})
	service.URL = "http://127.0.0.1:1"
	if _, err := opencode.NewClient(service).Latest(mainID); !errors.Is(err, opencode.ErrNoServer) {
		t.Errorf("Latest error = %v, want ErrNoServer", err)
	}
}

func TestAgentsListsEachChildSessionWithItsFill(t *testing.T) {
	f := &fake{
		sessions: map[string]session{
			mainID:        {ID: mainID, Directory: dir, Steps: []string{step("prov", "big-model", 1, 1, 1)}},
			"ses_child_a": {ID: "ses_child_a", ParentID: mainID, Title: "Find the export menu", Agent: "explore", Directory: dir, Updated: 100, Idle: 101, Steps: []string{step("prov", "big-model", 2, 52_000, 0)}},
			"ses_child_b": {ID: "ses_child_b", ParentID: mainID, Title: "Docs index", Agent: "general", Provider: "prov", Model: "small-model", Directory: dir, Updated: 200, Idle: 150},
			"ses_other":   {ID: "ses_other", ParentID: "ses_someone_else", Directory: dir},
		},
		models: map[string][]model{dir: {{"prov", "big-model", 1_048_576}}},
	}
	agents, err := opencode.NewClient(serve(t, f)).Agents(mainID)
	if err != nil {
		t.Fatalf("Agents: %v", err)
	}
	if len(agents) != 2 {
		t.Fatalf("Agents returned %d, want 2: %+v", len(agents), agents)
	}
	a, b := agents[0], agents[1]
	if a.ID != "ses_child_a" || a.Description != "Find the export menu" || a.Model != "prov/big-model" || !a.Finished || a.Reading.Used != 52_002 || a.Reading.Window != 1_048_576 {
		t.Errorf("first agent = %+v", a)
	}
	if b.ID != "ses_child_b" || b.Model != "prov/small-model" || b.Finished || b.Reading != (fill.Reading{}) {
		t.Errorf("second agent = %+v, want running with no reading yet", b)
	}
}

func TestLoadServiceReadsTheBackgroundServersAddress(t *testing.T) {
	path := filepath.Join(t.TempDir(), "service.json")
	if err := os.WriteFile(path, []byte(`{"id":"x","version":"2.1.4","url":"http://127.0.0.1:49374","pid":1,"password":"pw"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	service, err := opencode.LoadService(path)
	if err != nil || service != (opencode.Service{URL: "http://127.0.0.1:49374", Password: "pw"}) {
		t.Errorf("LoadService = %+v, %v", service, err)
	}
	if _, err := opencode.LoadService(filepath.Join(t.TempDir(), "service.json")); !errors.Is(err, opencode.ErrNoServer) {
		t.Errorf("LoadService of a missing file: %v, want ErrNoServer", err)
	}
}

func TestServicePathFollowsXDGStateHome(t *testing.T) {
	env := map[string]string{"HOME": "/Users/someone"}
	get := func(name string) string { return env[name] }
	if got, _ := opencode.ServicePath(get); got != "/Users/someone/.local/state/opencode/service.json" {
		t.Errorf("ServicePath = %q", got)
	}
	env["XDG_STATE_HOME"] = "/state"
	if got, _ := opencode.ServicePath(get); got != "/state/opencode/service.json" {
		t.Errorf("ServicePath with XDG_STATE_HOME = %q", got)
	}
	if _, err := opencode.ServicePath(func(string) string { return "" }); err == nil {
		t.Error("ServicePath succeeded with neither HOME nor XDG_STATE_HOME")
	}
}

func TestIsSubagentIsTrueForAChildSession(t *testing.T) {
	f := &fake{sessions: map[string]session{
		mainID:      {ID: mainID, Directory: dir},
		"ses_child": {ID: "ses_child", ParentID: mainID, Directory: dir},
	}}
	c := opencode.NewClient(serve(t, f))
	if sub, err := c.IsSubagent("ses_child"); err != nil || !sub {
		t.Errorf("IsSubagent(child) = %v, %v; want true", sub, err)
	}
	if sub, err := c.IsSubagent(mainID); err != nil || sub {
		t.Errorf("IsSubagent(main) = %v, %v; want false", sub, err)
	}
	if _, err := c.IsSubagent("ses_nope"); !errors.Is(err, fill.ErrNoSession) {
		t.Errorf("IsSubagent(unknown) error = %v, want ErrNoSession", err)
	}
}
