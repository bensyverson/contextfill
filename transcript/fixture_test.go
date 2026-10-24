package transcript_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// usage is one assistant turn's token counts, as a transcript records them.
type usage struct {
	input, cacheRead, cacheCreation, output int
}

// assistant renders a main-thread assistant entry with the given usage.
func assistant(model string, u usage) string {
	return entry(map[string]any{
		"type":           "assistant",
		"isSidechain":    false,
		"requestedModel": model,
		"timestamp":      "2026-10-24T16:00:00.000Z",
		"message": map[string]any{
			"model": strings.TrimSuffix(model, "[1m]"),
			"usage": map[string]any{
				"input_tokens":                u.input,
				"cache_read_input_tokens":     u.cacheRead,
				"cache_creation_input_tokens": u.cacheCreation,
				"output_tokens":               u.output,
			},
		},
	})
}

func entry(fields map[string]any) string {
	line, err := json.Marshal(fields)
	if err != nil {
		panic(err)
	}
	return string(line)
}

// writeLines writes a JSONL file, creating its directory.
func writeLines(t *testing.T, path string, lines ...string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(strings.Join(lines, "\n")+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
}

// writeFile writes a file, creating its directory.
func writeFile(t *testing.T, path, body string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}
