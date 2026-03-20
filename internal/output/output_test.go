package output

import (
	"encoding/json"
	"io"
	"os"
	"strings"
	"testing"

	"github.com/sheeppattern/zk/internal/model"
)

// captureStdout redirects os.Stdout to a pipe, runs fn, and returns
// everything written to stdout as a string.
func captureStdout(t *testing.T, fn func()) string {
	t.Helper()

	origStdout := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe() error: %v", err)
	}
	os.Stdout = w

	fn()

	w.Close()
	os.Stdout = origStdout

	out, err := io.ReadAll(r)
	if err != nil {
		t.Fatalf("ReadAll() error: %v", err)
	}
	return string(out)
}

func TestNewFormatter(t *testing.T) {
	f := NewFormatter("json")
	if f.Format != "json" {
		t.Fatalf("Format = %q; want %q", f.Format, "json")
	}

	f2 := NewFormatter("yaml")
	if f2.Format != "yaml" {
		t.Fatalf("Format = %q; want %q", f2.Format, "yaml")
	}
}

func TestFormatterJSON(t *testing.T) {
	f := NewFormatter("json")

	type sample struct {
		Name  string `json:"name"`
		Value int    `json:"value"`
	}
	data := sample{Name: "test", Value: 42}

	out := captureStdout(t, func() {
		if err := f.PrintJSON(data); err != nil {
			t.Fatalf("PrintJSON() error: %v", err)
		}
	})

	// Verify it's valid JSON.
	var parsed sample
	if err := json.Unmarshal([]byte(strings.TrimSpace(out)), &parsed); err != nil {
		t.Fatalf("output is not valid JSON: %v\noutput: %s", err, out)
	}
	if parsed.Name != "test" || parsed.Value != 42 {
		t.Fatalf("parsed = %+v; want {Name:test Value:42}", parsed)
	}
}

func TestFormatterYAML(t *testing.T) {
	f := NewFormatter("yaml")

	type sample struct {
		Name  string `yaml:"name"`
		Value int    `yaml:"value"`
	}
	data := sample{Name: "hello", Value: 7}

	out := captureStdout(t, func() {
		if err := f.PrintYAML(data); err != nil {
			t.Fatalf("PrintYAML() error: %v", err)
		}
	})

	if !strings.Contains(out, "name: hello") {
		t.Fatalf("YAML output missing 'name: hello'; got: %s", out)
	}
	if !strings.Contains(out, "value: 7") {
		t.Fatalf("YAML output missing 'value: 7'; got: %s", out)
	}
}

func TestFormatterPrintNote(t *testing.T) {
	f := NewFormatter("json")
	note := model.NewNote("Test Note", "Some content", []string{"go", "test"})

	out := captureStdout(t, func() {
		if err := f.PrintNote(note); err != nil {
			t.Fatalf("PrintNote() error: %v", err)
		}
	})

	// The JSON output should include content (via noteView).
	if !strings.Contains(out, "Some content") {
		t.Fatalf("JSON note output missing content; got: %s", out)
	}
	if !strings.Contains(out, "Test Note") {
		t.Fatalf("JSON note output missing title; got: %s", out)
	}
}
