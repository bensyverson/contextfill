package install

import (
	"encoding/json"
	"errors"
	"reflect"
	"strings"
	"testing"
)

const command = "/opt/homebrew/bin/contextfill hook --max 40 --main-max 33"

// existing is a settings file in the shape Claude Code writes, with keys in
// an order a sorted encoder would not keep, another tool's PostToolUse hook,
// and a command whose && an HTML-escaping encoder would mangle.
const existing = `{
  "cleanupPeriodDays": 999,
  "permissions": {
    "allow": ["Bash(job:*)", "Read(//tmp/**)"]
  },
  "model": "opus",
  "hooks": {
    "SessionStart": [
      { "hooks": [{ "type": "command", "command": "job orient && echo <ok>" }] }
    ],
    "PostToolUse": [
      { "hooks": [{ "type": "command", "command": "/usr/local/bin/status", "async": true }] }
    ],
    "PreToolUse": [
      { "matcher": "EnterPlanMode", "hooks": [{ "type": "command", "command": "deny" }] }
    ]
  },
  "alwaysThinkingEnabled": true
}
`

// keys is an object's keys in file order.
func keys(t *testing.T, data []byte) []string {
	t.Helper()
	order, err := parseObject(data)
	if err != nil {
		t.Fatalf("parse %s: %v", data, err)
	}
	var names []string
	for _, m := range order {
		names = append(names, m.Key)
	}
	return names
}

// value decodes the object member at the path of keys.
func value(t *testing.T, data []byte, path ...string) any {
	t.Helper()
	var v any
	if err := json.Unmarshal(data, &v); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	for _, k := range path {
		v = v.(map[string]any)[k]
	}
	return v
}

// commands is every command in PostToolUse, by group, as "matcher: command".
func commands(t *testing.T, data []byte) []string {
	t.Helper()
	var s struct {
		Hooks struct {
			PostToolUse []struct {
				Matcher string `json:"matcher"`
				Hooks   []struct {
					Command string `json:"command"`
				} `json:"hooks"`
			} `json:"PostToolUse"`
		} `json:"hooks"`
	}
	if err := json.Unmarshal(data, &s); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	var out []string
	for _, g := range s.Hooks.PostToolUse {
		for _, h := range g.Hooks {
			out = append(out, g.Matcher+": "+h.Command)
		}
	}
	return out
}

func TestMergeKeepsEveryKeyAndItsOrder(t *testing.T) {
	out, changes, err := MergeClaude([]byte(existing), command)
	if err != nil {
		t.Fatal(err)
	}
	if got, want := keys(t, out), []string{"cleanupPeriodDays", "permissions", "model", "hooks", "alwaysThinkingEnabled"}; !reflect.DeepEqual(got, want) {
		t.Errorf("top-level keys = %v, want %v", got, want)
	}
	for _, path := range [][]string{{"cleanupPeriodDays"}, {"permissions"}, {"model"}, {"alwaysThinkingEnabled"}, {"hooks", "SessionStart"}, {"hooks", "PreToolUse"}} {
		if got, want := value(t, out, path...), value(t, []byte(existing), path...); !reflect.DeepEqual(got, want) {
			t.Errorf("%v = %v, want it kept as %v", path, got, want)
		}
	}
	if got, want := commands(t, out), []string{": /usr/local/bin/status", "Bash: " + command}; !reflect.DeepEqual(got, want) {
		t.Errorf("PostToolUse = %v, want %v", got, want)
	}
	if len(changes) != 1 || changes[0].Kind != Added || changes[0].New != command {
		t.Errorf("changes = %+v, want one Added", changes)
	}
}

func TestMergeKeepsTheHooksObjectsOrder(t *testing.T) {
	out, _, err := MergeClaude([]byte(existing), command)
	if err != nil {
		t.Fatal(err)
	}
	top, err := parseObject(out)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, m := range top {
		if m.Key == "hooks" {
			found = true
			if got, want := keys(t, m.Value), []string{"SessionStart", "PostToolUse", "PreToolUse"}; !reflect.DeepEqual(got, want) {
				t.Errorf("hooks keys = %v, want %v", got, want)
			}
		}
	}
	if !found {
		t.Errorf("no hooks key in %s", out)
	}
}

func TestMergeDoesNotEscapeOtherCommands(t *testing.T) {
	out, _, err := MergeClaude([]byte(existing), command)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(out), "job orient && echo <ok>") {
		t.Errorf("another hook's command was rewritten:\n%s", out)
	}
}

func TestMergeTwiceChangesNothing(t *testing.T) {
	once, _, err := MergeClaude([]byte(existing), command)
	if err != nil {
		t.Fatal(err)
	}
	twice, changes, err := MergeClaude(once, command)
	if err != nil {
		t.Fatal(err)
	}
	if string(twice) != string(once) {
		t.Errorf("second merge changed the file:\n%s\nthen\n%s", once, twice)
	}
	if len(changes) != 1 || changes[0].Kind != Unchanged {
		t.Errorf("changes = %+v, want one Unchanged", changes)
	}
}

func TestMergeReplacesAStaleEntryInPlace(t *testing.T) {
	stale := `{"hooks": {"PostToolUse": [
  {"matcher": "Bash", "hooks": [{"type": "command", "command": "/Users/someone/go/bin/contextfill hook --max 40", "timeout": 10}]},
  {"hooks": [{"type": "command", "command": "/usr/local/bin/status"}]}
]}}`
	out, changes, err := MergeClaude([]byte(stale), command)
	if err != nil {
		t.Fatal(err)
	}
	if got, want := commands(t, out), []string{"Bash: " + command, ": /usr/local/bin/status"}; !reflect.DeepEqual(got, want) {
		t.Errorf("PostToolUse = %v, want %v", got, want)
	}
	group := value(t, out, "hooks", "PostToolUse").([]any)[0].(map[string]any)
	if timeout := group["hooks"].([]any)[0].(map[string]any)["timeout"]; timeout != 10.0 {
		t.Errorf("timeout = %v, want the entry's own 10 kept", timeout)
	}
	if len(changes) != 1 || changes[0].Kind != Replaced || changes[0].Old != "/Users/someone/go/bin/contextfill hook --max 40" {
		t.Errorf("changes = %+v, want one Replaced naming the old command", changes)
	}
}

func TestMergeReplacesAChangedMax(t *testing.T) {
	once, _, err := MergeClaude([]byte(existing), command)
	if err != nil {
		t.Fatal(err)
	}
	wider := "/opt/homebrew/bin/contextfill hook --max 50 --main-max 33"
	out, changes, err := MergeClaude(once, wider)
	if err != nil {
		t.Fatal(err)
	}
	if got, want := commands(t, out), []string{": /usr/local/bin/status", "Bash: " + wider}; !reflect.DeepEqual(got, want) {
		t.Errorf("PostToolUse = %v, want %v", got, want)
	}
	if len(changes) != 1 || changes[0].Kind != Replaced {
		t.Errorf("changes = %+v, want one Replaced", changes)
	}
}

func TestMergeLeavesOneEntryWhereThereWereTwo(t *testing.T) {
	two := `{"hooks": {"PostToolUse": [
  {"matcher": "Bash", "hooks": [{"type": "command", "command": "contextfill hook --max 40"}]},
  {"matcher": "Bash", "hooks": [{"type": "command", "command": "/Users/someone/go/bin/contextfill hook"}]}
]}}`
	out, changes, err := MergeClaude([]byte(two), command)
	if err != nil {
		t.Fatal(err)
	}
	if got, want := commands(t, out), []string{"Bash: " + command}; !reflect.DeepEqual(got, want) {
		t.Errorf("PostToolUse = %v, want %v", got, want)
	}
	if len(changes) != 2 || changes[0].Kind != Replaced || changes[1].Kind != Removed {
		t.Errorf("changes = %+v, want Replaced then Removed", changes)
	}
}

func TestMergeMovesAnEntryOffAnotherMatcher(t *testing.T) {
	wrong := `{"hooks": {"PostToolUse": [
  {"hooks": [{"type": "command", "command": "/usr/local/bin/status"}, {"type": "command", "command": "contextfill hook"}]}
]}}`
	out, _, err := MergeClaude([]byte(wrong), command)
	if err != nil {
		t.Fatal(err)
	}
	if got, want := commands(t, out), []string{": /usr/local/bin/status", "Bash: " + command}; !reflect.DeepEqual(got, want) {
		t.Errorf("PostToolUse = %v, want %v", got, want)
	}
}

func TestMergeIntoNothingStartsAFile(t *testing.T) {
	out, changes, err := MergeClaude(nil, command)
	if err != nil {
		t.Fatal(err)
	}
	if got, want := commands(t, out), []string{"Bash: " + command}; !reflect.DeepEqual(got, want) {
		t.Errorf("PostToolUse = %v, want %v", got, want)
	}
	group := value(t, out, "hooks", "PostToolUse").([]any)[0].(map[string]any)
	if timeout := group["hooks"].([]any)[0].(map[string]any)["timeout"]; timeout != 10.0 {
		t.Errorf("timeout = %v, want 10", timeout)
	}
	if len(changes) != 1 || changes[0].Kind != Added {
		t.Errorf("changes = %+v, want one Added", changes)
	}
}

func TestMergeRefusesWhatIsNotASettingsObject(t *testing.T) {
	for _, bad := range []string{`{"model": "opus",}`, `[]`, `{"hooks": []}`, `{"hooks": {"PostToolUse": {}}}`, `{} {}`} {
		if _, _, err := MergeClaude([]byte(bad), command); !errors.Is(err, ErrInvalidSettings) {
			t.Errorf("MergeClaude(%s) error = %v, want ErrInvalidSettings", bad, err)
		}
	}
}

func TestRemoveTakesOnlyContextfillsEntry(t *testing.T) {
	once, _, err := MergeClaude([]byte(existing), command)
	if err != nil {
		t.Fatal(err)
	}
	out, changes, err := RemoveClaude(once)
	if err != nil {
		t.Fatal(err)
	}
	if got, want := commands(t, out), []string{": /usr/local/bin/status"}; !reflect.DeepEqual(got, want) {
		t.Errorf("PostToolUse = %v, want %v", got, want)
	}
	if got, want := keys(t, out), keys(t, []byte(existing)); !reflect.DeepEqual(got, want) {
		t.Errorf("keys = %v, want %v", got, want)
	}
	if len(changes) != 1 || changes[0].Kind != Removed || changes[0].Old != command {
		t.Errorf("changes = %+v, want one Removed", changes)
	}
}

func TestRemoveWithNothingToRemoveChangesNothing(t *testing.T) {
	out, changes, err := RemoveClaude([]byte(existing))
	if err != nil {
		t.Fatal(err)
	}
	if string(out) != existing {
		t.Errorf("RemoveClaude rewrote a file it had nothing to take from")
	}
	if len(changes) != 1 || changes[0].Kind != Unchanged {
		t.Errorf("changes = %+v, want one Unchanged", changes)
	}
}

func TestIsContextfill(t *testing.T) {
	for command, want := range map[string]bool{
		"/opt/homebrew/bin/contextfill hook --max 40": true,
		"contextfill hook":                       true,
		"/Users/someone/go/bin/contextfill":      true,
		`"/Users/some one/bin/contextfill" hook`: true,
		"CONTEXTFILL_X=1 contextfill hook":       true,
		"echo contextfill":                       false,
		"/usr/local/bin/contextfill-beta hook":   false,
		"/usr/local/bin/status":                  false,
		"":                                       false,
	} {
		if got := IsContextfill(command); got != want {
			t.Errorf("IsContextfill(%q) = %v, want %v", command, got, want)
		}
	}
}
