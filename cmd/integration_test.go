package cmd_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

var zkBinary string

func TestMain(m *testing.M) {
	tmp, err := os.MkdirTemp("", "zk-test-bin-*")
	if err != nil {
		panic("failed to create temp dir for binary: " + err.Error())
	}

	binName := "zk"
	if runtime.GOOS == "windows" {
		binName = "zk.exe"
	}
	zkBinary = filepath.Join(tmp, binName)

	// Build the binary from the project root (one level up from cmd/).
	projectRoot := filepath.Join("..", ".")
	cmd := exec.Command("go", "build", "-o", zkBinary, ".")
	cmd.Dir = projectRoot
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		panic("failed to build zk: " + err.Error())
	}

	code := m.Run()
	os.RemoveAll(tmp)
	os.Exit(code)
}

// runZK executes the zk binary with ZKMEMORY_PATH pointing to storeDir.
func runZK(t *testing.T, storeDir string, args ...string) (string, string, error) {
	t.Helper()
	cmd := exec.Command(zkBinary, args...)
	cmd.Env = append(os.Environ(), "ZKMEMORY_PATH="+storeDir)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	return stdout.String(), stderr.String(), err
}

// mustRunZK calls runZK and fails the test if the command exits with an error.
func mustRunZK(t *testing.T, storeDir string, args ...string) string {
	t.Helper()
	stdout, stderr, err := runZK(t, storeDir, args...)
	if err != nil {
		t.Fatalf("zk %s failed: %v\nstdout: %s\nstderr: %s", strings.Join(args, " "), err, stdout, stderr)
	}
	return stdout
}

// parseJSON unmarshals stdout JSON into the given target.
func parseJSON(t *testing.T, data string, target interface{}) {
	t.Helper()
	if err := json.Unmarshal([]byte(data), target); err != nil {
		t.Fatalf("failed to parse JSON: %v\nraw: %s", err, data)
	}
}

// initStore runs "zk init" on a fresh temp dir and returns the store path.
func initStore(t *testing.T) string {
	t.Helper()
	storeDir := t.TempDir()
	mustRunZK(t, storeDir, "init")
	return storeDir
}

// createProject creates a project and returns its ID.
func createProject(t *testing.T, storeDir, name, description string) string {
	t.Helper()
	args := []string{"project", "create", name}
	if description != "" {
		args = append(args, "--description", description)
	}
	stdout := mustRunZK(t, storeDir, args...)
	var proj map[string]interface{}
	parseJSON(t, stdout, &proj)
	id, ok := proj["id"].(string)
	if !ok || id == "" {
		t.Fatalf("project create did not return an id; output: %s", stdout)
	}
	return id
}

// createNote creates a note and returns its ID.
func createNote(t *testing.T, storeDir, projectID, title, content string, tags []string) string {
	t.Helper()
	args := []string{"note", "create", "--title", title, "--project", projectID}
	if content != "" {
		args = append(args, "--content", content)
	}
	if len(tags) > 0 {
		args = append(args, "--tags", strings.Join(tags, ","))
	}
	stdout := mustRunZK(t, storeDir, args...)
	var note map[string]interface{}
	parseJSON(t, stdout, &note)
	id, ok := note["id"].(string)
	if !ok || id == "" {
		t.Fatalf("note create did not return an id; output: %s", stdout)
	}
	return id
}

// ---------------------------------------------------------------------------
// Tests
// ---------------------------------------------------------------------------

func TestCLIInit(t *testing.T) {
	storeDir := t.TempDir()
	_, stderr, err := runZK(t, storeDir, "init")
	if err != nil {
		t.Fatalf("init failed: %v\nstderr: %s", err, stderr)
	}

	// Verify expected directories and files exist.
	for _, rel := range []string{
		"config.yaml",
		"projects",
		filepath.Join("global", "notes"),
	} {
		path := filepath.Join(storeDir, rel)
		if _, err := os.Stat(path); os.IsNotExist(err) {
			t.Errorf("expected %s to exist after init", rel)
		}
	}
}

func TestCLIProjectCRUD(t *testing.T) {
	storeDir := initStore(t)

	// Create
	projID := createProject(t, storeDir, "test-proj", "a test project")
	if !strings.HasPrefix(projID, "P-") {
		t.Fatalf("project ID should start with P-, got %s", projID)
	}

	// List: expect 1 project
	stdout := mustRunZK(t, storeDir, "project", "list")
	var projects []map[string]interface{}
	parseJSON(t, stdout, &projects)
	if len(projects) != 1 {
		t.Fatalf("expected 1 project, got %d", len(projects))
	}

	// Get: verify name and note_count
	stdout = mustRunZK(t, storeDir, "project", "get", projID)
	var detail map[string]interface{}
	parseJSON(t, stdout, &detail)
	if detail["name"] != "test-proj" {
		t.Errorf("expected name 'test-proj', got %v", detail["name"])
	}
	if nc, ok := detail["note_count"].(float64); !ok || int(nc) != 0 {
		t.Errorf("expected note_count=0, got %v", detail["note_count"])
	}

	// Delete
	mustRunZK(t, storeDir, "project", "delete", projID)

	// List: expect 0 projects
	stdout = mustRunZK(t, storeDir, "project", "list")
	parseJSON(t, stdout, &projects)
	if len(projects) != 0 {
		t.Fatalf("expected 0 projects after delete, got %d", len(projects))
	}
}

func TestCLINoteCRUD(t *testing.T) {
	storeDir := initStore(t)
	projID := createProject(t, storeDir, "note-proj", "")

	// Create note
	noteID := createNote(t, storeDir, projID, "Test Title", "Test Content", []string{"alpha", "beta"})
	if !strings.HasPrefix(noteID, "N-") {
		t.Fatalf("note ID should start with N-, got %s", noteID)
	}

	// Get note: verify fields
	stdout := mustRunZK(t, storeDir, "note", "get", noteID, "--project", projID)
	var note map[string]interface{}
	parseJSON(t, stdout, &note)
	if note["title"] != "Test Title" {
		t.Errorf("expected title 'Test Title', got %v", note["title"])
	}
	if note["content"] != "Test Content" {
		t.Errorf("expected content 'Test Content', got %v", note["content"])
	}
	tags, ok := note["tags"].([]interface{})
	if !ok || len(tags) != 2 {
		t.Errorf("expected 2 tags, got %v", note["tags"])
	}

	// Update note title
	stdout = mustRunZK(t, storeDir, "note", "update", noteID, "--title", "New Title", "--project", projID)
	parseJSON(t, stdout, &note)
	if note["title"] != "New Title" {
		t.Errorf("expected updated title 'New Title', got %v", note["title"])
	}

	// List notes: expect 1
	stdout = mustRunZK(t, storeDir, "note", "list", "--project", projID)
	var notes []map[string]interface{}
	parseJSON(t, stdout, &notes)
	if len(notes) != 1 {
		t.Fatalf("expected 1 note, got %d", len(notes))
	}

	// Delete note with --force
	mustRunZK(t, storeDir, "note", "delete", noteID, "--force", "--project", projID)

	// List notes: expect 0
	stdout = mustRunZK(t, storeDir, "note", "list", "--project", projID)
	parseJSON(t, stdout, &notes)
	if len(notes) != 0 {
		t.Fatalf("expected 0 notes after delete, got %d", len(notes))
	}
}

func TestCLILinkWorkflow(t *testing.T) {
	storeDir := initStore(t)
	projID := createProject(t, storeDir, "link-proj", "")

	n1 := createNote(t, storeDir, projID, "Note One", "Content one", nil)
	n2 := createNote(t, storeDir, projID, "Note Two", "Content two", nil)

	// Add link
	mustRunZK(t, storeDir, "link", "add", n1, n2, "--type", "supports", "--weight", "0.8", "--project", projID)

	// Add duplicate link: should succeed (skips silently), stderr mentions "already exists"
	_, stderr, err := runZK(t, storeDir, "link", "add", n1, n2, "--type", "supports", "--weight", "0.8", "--project", projID)
	if err != nil {
		t.Fatalf("duplicate link add should not fail, got: %v\nstderr: %s", err, stderr)
	}
	if !strings.Contains(stderr, "already exists") {
		t.Errorf("expected stderr to mention 'already exists', got: %s", stderr)
	}

	// List links for n1
	stdout := mustRunZK(t, storeDir, "link", "list", n1, "--project", projID)
	var linkResult struct {
		Outgoing []map[string]interface{} `json:"outgoing"`
		Incoming []map[string]interface{} `json:"incoming"`
	}
	parseJSON(t, stdout, &linkResult)
	if len(linkResult.Outgoing) != 1 {
		t.Fatalf("expected 1 outgoing link, got %d", len(linkResult.Outgoing))
	}
	if linkResult.Outgoing[0]["target_id"] != n2 {
		t.Errorf("expected outgoing target %s, got %v", n2, linkResult.Outgoing[0]["target_id"])
	}
	if linkResult.Outgoing[0]["relation_type"] != "supports" {
		t.Errorf("expected relation_type 'supports', got %v", linkResult.Outgoing[0]["relation_type"])
	}

	// n2 should have an incoming link from n1 (bidirectional)
	stdout = mustRunZK(t, storeDir, "link", "list", n2, "--project", projID)
	var linkResult2 struct {
		Outgoing []map[string]interface{} `json:"outgoing"`
		Incoming []map[string]interface{} `json:"incoming"`
	}
	parseJSON(t, stdout, &linkResult2)
	// n2 has an outgoing link back to n1 (reverse link added by bidirectional logic)
	if len(linkResult2.Outgoing) != 1 {
		t.Errorf("expected 1 outgoing (reverse) link on n2, got %d", len(linkResult2.Outgoing))
	}
}

func TestCLISearch(t *testing.T) {
	storeDir := initStore(t)
	projID := createProject(t, storeDir, "search-proj", "")

	createNote(t, storeDir, projID, "Redis Caching", "Redis is a fast in-memory store", []string{"database", "cache"})
	createNote(t, storeDir, projID, "PostgreSQL Guide", "PostgreSQL is a relational database", []string{"database", "sql"})
	createNote(t, storeDir, projID, "Go Concurrency", "Goroutines and channels", []string{"golang", "concurrency"})

	// Search for "redis": should match 1 note
	stdout := mustRunZK(t, storeDir, "search", "redis", "--project", projID)
	var results []map[string]interface{}
	parseJSON(t, stdout, &results)
	if len(results) != 1 {
		t.Fatalf("expected 1 search result for 'redis', got %d", len(results))
	}
	if results[0]["title"] != "Redis Caching" {
		t.Errorf("expected title 'Redis Caching', got %v", results[0]["title"])
	}

	// Search for "database": should match 2 notes (Redis and PostgreSQL both have "database" tag)
	stdout = mustRunZK(t, storeDir, "search", "database", "--project", projID)
	parseJSON(t, stdout, &results)
	if len(results) != 2 {
		t.Fatalf("expected 2 search results for 'database', got %d", len(results))
	}

	// Search with --tags filter: "database" query filtered by tag "sql" should return only PostgreSQL
	stdout = mustRunZK(t, storeDir, "search", "database", "--tags", "sql", "--project", projID)
	parseJSON(t, stdout, &results)
	if len(results) != 1 {
		t.Fatalf("expected 1 search result for 'database' with tag 'sql', got %d", len(results))
	}
	if results[0]["title"] != "PostgreSQL Guide" {
		t.Errorf("expected title 'PostgreSQL Guide', got %v", results[0]["title"])
	}

	// Search with no matches
	stdout = mustRunZK(t, storeDir, "search", "nonexistent-term", "--project", projID)
	parseJSON(t, stdout, &results)
	if len(results) != 0 {
		t.Errorf("expected 0 results for non-matching query, got %d", len(results))
	}
}

func TestCLIDiagnose(t *testing.T) {
	storeDir := initStore(t)
	projID := createProject(t, storeDir, "diag-proj", "")

	n1 := createNote(t, storeDir, projID, "Diag Note 1", "Content 1", nil)
	n2 := createNote(t, storeDir, projID, "Diag Note 2", "Content 2", nil)

	// Link them so they are not orphans
	mustRunZK(t, storeDir, "link", "add", n1, n2, "--type", "related", "--weight", "0.5", "--project", projID)

	// Run diagnose
	stdout := mustRunZK(t, storeDir, "diagnose", "--project", projID)
	var report struct {
		TotalNotes int `json:"total_notes"`
		TotalLinks int `json:"total_links"`
		Errors     []interface{} `json:"errors"`
		Warnings   []interface{} `json:"warnings"`
		Summary    struct {
			ErrorCount   int    `json:"error_count"`
			WarningCount int    `json:"warning_count"`
			HealthScore  string `json:"health_score"`
		} `json:"summary"`
	}
	parseJSON(t, stdout, &report)

	if report.TotalNotes != 2 {
		t.Errorf("expected 2 total_notes, got %d", report.TotalNotes)
	}
	// Bidirectional links: 2 links per add (forward + reverse)
	if report.TotalLinks != 2 {
		t.Errorf("expected 2 total_links, got %d", report.TotalLinks)
	}
	if report.Summary.HealthScore != "healthy" {
		t.Errorf("expected health_score 'healthy', got %q", report.Summary.HealthScore)
	}
	if report.Summary.ErrorCount != 0 {
		t.Errorf("expected 0 errors, got %d", report.Summary.ErrorCount)
	}
}

func TestCLINoteDeleteBacklinkWarning(t *testing.T) {
	storeDir := initStore(t)
	projID := createProject(t, storeDir, "backlink-proj", "")

	n1 := createNote(t, storeDir, projID, "Source", "source content", nil)
	n2 := createNote(t, storeDir, projID, "Target", "target content", nil)

	mustRunZK(t, storeDir, "link", "add", n1, n2, "--type", "supports", "--weight", "0.5", "--project", projID)

	// Delete n2 without --force: should fail due to backlinks from n1
	_, stderr, err := runZK(t, storeDir, "note", "delete", n2, "--project", projID)
	if err == nil {
		t.Fatal("expected delete without --force to fail when backlinks exist")
	}
	if !strings.Contains(stderr, "backlink") {
		t.Errorf("expected stderr to mention backlinks, got: %s", stderr)
	}

	// Delete n2 with --force: should succeed
	mustRunZK(t, storeDir, "note", "delete", n2, "--force", "--project", projID)
}

func TestCLIProjectGetNoteCount(t *testing.T) {
	storeDir := initStore(t)
	projID := createProject(t, storeDir, "count-proj", "")

	createNote(t, storeDir, projID, "A", "a", nil)
	createNote(t, storeDir, projID, "B", "b", nil)

	stdout := mustRunZK(t, storeDir, "project", "get", projID)
	var detail map[string]interface{}
	parseJSON(t, stdout, &detail)

	nc, ok := detail["note_count"].(float64)
	if !ok || int(nc) != 2 {
		t.Errorf("expected note_count=2, got %v", detail["note_count"])
	}
}

func TestCLISearchNoProject(t *testing.T) {
	// Search without --project uses global scope; should still work after init.
	storeDir := initStore(t)

	stdout := mustRunZK(t, storeDir, "search", "anything")
	var results []map[string]interface{}
	parseJSON(t, stdout, &results)
	if len(results) != 0 {
		t.Errorf("expected 0 results in empty global scope, got %d", len(results))
	}
}

func TestCLILinkRemove(t *testing.T) {
	storeDir := initStore(t)
	projID := createProject(t, storeDir, "rm-link-proj", "")

	n1 := createNote(t, storeDir, projID, "A", "a", nil)
	n2 := createNote(t, storeDir, projID, "B", "b", nil)

	mustRunZK(t, storeDir, "link", "add", n1, n2, "--type", "related", "--project", projID)

	// Remove the link
	mustRunZK(t, storeDir, "link", "remove", n1, n2, "--project", projID)

	// Verify links are gone
	stdout := mustRunZK(t, storeDir, "link", "list", n1, "--project", projID)
	var linkResult struct {
		Outgoing []interface{} `json:"outgoing"`
		Incoming []interface{} `json:"incoming"`
	}
	parseJSON(t, stdout, &linkResult)
	if len(linkResult.Outgoing) != 0 {
		t.Errorf("expected 0 outgoing links after remove, got %d", len(linkResult.Outgoing))
	}
}

func TestCLIDiagnoseOrphanWarning(t *testing.T) {
	storeDir := initStore(t)
	projID := createProject(t, storeDir, "orphan-proj", "")

	// Create a single unlinked note — it should be flagged as orphan.
	createNote(t, storeDir, projID, "Lonely", "alone", nil)

	stdout := mustRunZK(t, storeDir, "diagnose", "--project", projID)
	var report struct {
		Summary struct {
			WarningCount int    `json:"warning_count"`
			HealthScore  string `json:"health_score"`
		} `json:"summary"`
		Warnings []struct {
			Message string `json:"message"`
		} `json:"warnings"`
	}
	parseJSON(t, stdout, &report)

	if report.Summary.WarningCount == 0 {
		t.Fatal("expected at least 1 warning for orphan note")
	}
	foundOrphan := false
	for _, w := range report.Warnings {
		if strings.Contains(w.Message, "orphan") {
			foundOrphan = true
			break
		}
	}
	if !foundOrphan {
		t.Error("expected an orphan warning in diagnose output")
	}
	if report.Summary.HealthScore != "warnings" {
		t.Errorf("expected health_score 'warnings', got %q", report.Summary.HealthScore)
	}
}

func TestCLINoteUpdate(t *testing.T) {
	storeDir := initStore(t)
	projID := createProject(t, storeDir, "update-proj", "")

	noteID := createNote(t, storeDir, projID, "Original", "original content", []string{"old"})

	// Update multiple fields
	stdout := mustRunZK(t, storeDir, "note", "update", noteID,
		"--title", "Updated",
		"--content", "new content",
		"--tags", "new",
		"--status", "archived",
		"--project", projID,
	)
	var note map[string]interface{}
	parseJSON(t, stdout, &note)

	if note["title"] != "Updated" {
		t.Errorf("expected title 'Updated', got %v", note["title"])
	}
	if note["content"] != "new content" {
		t.Errorf("expected content 'new content', got %v", note["content"])
	}

	// Verify via get
	stdout = mustRunZK(t, storeDir, "note", "get", noteID, "--project", projID)
	parseJSON(t, stdout, &note)
	if note["title"] != "Updated" {
		t.Errorf("get after update: expected title 'Updated', got %v", note["title"])
	}

	// Check metadata.status
	meta, ok := note["metadata"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected metadata to be a map, got %T", note["metadata"])
	}
	if meta["status"] != "archived" {
		t.Errorf("expected status 'archived', got %v", meta["status"])
	}
}

func TestCLIMultipleProjects(t *testing.T) {
	storeDir := initStore(t)

	p1 := createProject(t, storeDir, "proj-alpha", "")
	p2 := createProject(t, storeDir, "proj-beta", "")

	createNote(t, storeDir, p1, "Alpha Note", "in alpha", nil)
	createNote(t, storeDir, p2, "Beta Note", "in beta", nil)

	// List notes in p1: should have 1
	stdout := mustRunZK(t, storeDir, "note", "list", "--project", p1)
	var notes []map[string]interface{}
	parseJSON(t, stdout, &notes)
	if len(notes) != 1 {
		t.Errorf("expected 1 note in proj-alpha, got %d", len(notes))
	}

	// List notes in p2: should have 1
	stdout = mustRunZK(t, storeDir, "note", "list", "--project", p2)
	parseJSON(t, stdout, &notes)
	if len(notes) != 1 {
		t.Errorf("expected 1 note in proj-beta, got %d", len(notes))
	}

	// Project list should have 2
	stdout = mustRunZK(t, storeDir, "project", "list")
	var projects []map[string]interface{}
	parseJSON(t, stdout, &projects)
	if len(projects) != 2 {
		t.Errorf("expected 2 projects, got %d", len(projects))
	}
}

func TestCLIVersionFlag(t *testing.T) {
	storeDir := t.TempDir()
	// Running with no subcommand should not error
	_, _, err := runZK(t, storeDir, "--help")
	if err != nil {
		t.Errorf("expected --help to succeed, got: %v", err)
	}
}

func TestCLIInvalidCommand(t *testing.T) {
	storeDir := t.TempDir()
	_, _, err := runZK(t, storeDir, "nonexistent-command")
	if err == nil {
		t.Error("expected nonexistent command to fail")
	}
}

func TestCLINoteCreateRequiresTitle(t *testing.T) {
	storeDir := initStore(t)
	projID := createProject(t, storeDir, "req-proj", "")

	// note create without --title should fail
	_, _, err := runZK(t, storeDir, "note", "create", "--content", "no title", "--project", projID)
	if err == nil {
		t.Error("expected note create without --title to fail")
	}
}

func TestCLIOutputFormat(t *testing.T) {
	storeDir := initStore(t)
	projID := createProject(t, storeDir, "fmt-proj", "")
	createNote(t, storeDir, projID, "Formatted", "content", nil)

	// YAML format
	stdout := mustRunZK(t, storeDir, "note", "list", "--project", projID, "--format", "yaml")
	if !strings.Contains(stdout, "title:") {
		t.Errorf("expected YAML output to contain 'title:', got: %s", stdout)
	}

	// JSON format (default)
	stdout = mustRunZK(t, storeDir, "note", "list", "--project", projID, "--format", "json")
	var notes []map[string]interface{}
	if err := json.Unmarshal([]byte(stdout), &notes); err != nil {
		t.Errorf("expected valid JSON output, got parse error: %v", err)
	}
}

func TestCLINoteLayer(t *testing.T) {
	storeDir := initStore(t)
	projID := createProject(t, storeDir, "layer-proj", "")

	// Create a concrete note (default layer).
	concreteID := createNote(t, storeDir, projID, "Concrete Note", "concrete content", nil)

	// Create an abstract note using --layer abstract.
	stdout := mustRunZK(t, storeDir, "note", "create", "--title", "Abstract Note", "--content", "abstract content", "--layer", "abstract", "--project", projID)
	var absNote map[string]interface{}
	parseJSON(t, stdout, &absNote)
	abstractID := absNote["id"].(string)
	if abstractID == "" {
		t.Fatal("abstract note create did not return an id")
	}

	// note list --layer concrete → only concrete note
	stdout = mustRunZK(t, storeDir, "note", "list", "--project", projID, "--layer", "concrete")
	var notes []map[string]interface{}
	parseJSON(t, stdout, &notes)
	if len(notes) != 1 {
		t.Fatalf("expected 1 concrete note, got %d", len(notes))
	}
	if notes[0]["id"] != concreteID {
		t.Errorf("expected concrete note id %s, got %v", concreteID, notes[0]["id"])
	}

	// note list --layer abstract → only abstract note
	stdout = mustRunZK(t, storeDir, "note", "list", "--project", projID, "--layer", "abstract")
	parseJSON(t, stdout, &notes)
	if len(notes) != 1 {
		t.Fatalf("expected 1 abstract note, got %d", len(notes))
	}
	if notes[0]["id"] != abstractID {
		t.Errorf("expected abstract note id %s, got %v", abstractID, notes[0]["id"])
	}

	// note list (no --layer) → both notes
	stdout = mustRunZK(t, storeDir, "note", "list", "--project", projID)
	parseJSON(t, stdout, &notes)
	if len(notes) != 2 {
		t.Fatalf("expected 2 notes without layer filter, got %d", len(notes))
	}
}

func TestCLIReflect(t *testing.T) {
	storeDir := initStore(t)
	projID := createProject(t, storeDir, "reflect-proj", "")

	// Create 3 concrete notes.
	n1 := createNote(t, storeDir, projID, "Note One", "content one", nil)
	n2 := createNote(t, storeDir, projID, "Note Two", "content two", nil)
	n3 := createNote(t, storeDir, projID, "Note Three", "content three", nil)
	_ = n3 // n3 is left unlinked (orphan)

	// Link N1 → N2 with --type contradicts (tension).
	mustRunZK(t, storeDir, "link", "add", n1, n2, "--type", "contradicts", "--project", projID)

	// Run reflect.
	stdout := mustRunZK(t, storeDir, "reflect", "--project", projID, "--format", "json")
	var report struct {
		Insights []struct {
			Type        string   `json:"type"`
			SourceNotes []string `json:"source_notes"`
			Suggestion  string   `json:"suggestion"`
		} `json:"insights"`
		Stats struct {
			ConcreteCount int `json:"concrete_count"`
			AbstractCount int `json:"abstract_count"`
		} `json:"stats"`
	}
	parseJSON(t, stdout, &report)

	// Verify stats.
	if report.Stats.ConcreteCount != 3 {
		t.Errorf("expected concrete_count=3, got %d", report.Stats.ConcreteCount)
	}
	if report.Stats.AbstractCount != 0 {
		t.Errorf("expected abstract_count=0, got %d", report.Stats.AbstractCount)
	}

	// Verify insights contain "tension" type.
	foundTension := false
	foundOrphan := false
	for _, ins := range report.Insights {
		if ins.Type == "tension" {
			foundTension = true
		}
		if ins.Type == "orphan_cluster" {
			foundOrphan = true
		}
	}
	if !foundTension {
		t.Error("expected insight of type 'tension' in reflect output")
	}
	if !foundOrphan {
		t.Error("expected insight of type 'orphan_cluster' in reflect output")
	}
}

func TestCLIReflectApply(t *testing.T) {
	storeDir := initStore(t)
	projID := createProject(t, storeDir, "reflect-apply-proj", "")

	// Create 2 concrete notes linked with contradicts.
	n1 := createNote(t, storeDir, projID, "Thesis", "thesis content", nil)
	n2 := createNote(t, storeDir, projID, "Antithesis", "antithesis content", nil)
	mustRunZK(t, storeDir, "link", "add", n1, n2, "--type", "contradicts", "--project", projID)

	// Run reflect --apply.
	mustRunZK(t, storeDir, "reflect", "--project", projID, "--apply", "--format", "json")

	// note list --layer abstract → at least 1 abstract note was created.
	stdout := mustRunZK(t, storeDir, "note", "list", "--project", projID, "--layer", "abstract")
	var abstractNotes []map[string]interface{}
	parseJSON(t, stdout, &abstractNotes)
	if len(abstractNotes) < 1 {
		t.Fatalf("expected at least 1 abstract note after reflect --apply, got %d", len(abstractNotes))
	}

	// Verify source note has an "abstracts" link.
	stdout = mustRunZK(t, storeDir, "link", "list", n1, "--project", projID)
	var linkResult struct {
		Outgoing []map[string]interface{} `json:"outgoing"`
		Incoming []map[string]interface{} `json:"incoming"`
	}
	parseJSON(t, stdout, &linkResult)

	foundAbstracts := false
	for _, link := range linkResult.Outgoing {
		if link["relation_type"] == "abstracts" {
			foundAbstracts = true
			break
		}
	}
	if !foundAbstracts {
		t.Error("expected source note to have an 'abstracts' link after reflect --apply")
	}
}

func TestCLISearchLayer(t *testing.T) {
	storeDir := initStore(t)
	projID := createProject(t, storeDir, "search-layer-proj", "")

	// Create 1 concrete + 1 abstract note.
	createNote(t, storeDir, projID, "Concrete Search", "concrete content for search", []string{"search"})
	mustRunZK(t, storeDir, "note", "create", "--title", "Abstract Search", "--content", "abstract content for search", "--layer", "abstract", "--tags", "search", "--project", projID)

	// Search with --layer concrete → only concrete.
	stdout := mustRunZK(t, storeDir, "search", "search", "--project", projID, "--layer", "concrete")
	var results []map[string]interface{}
	parseJSON(t, stdout, &results)
	if len(results) != 1 {
		t.Fatalf("expected 1 concrete search result, got %d", len(results))
	}
	if results[0]["title"] != "Concrete Search" {
		t.Errorf("expected title 'Concrete Search', got %v", results[0]["title"])
	}

	// Search with --layer abstract → only abstract.
	stdout = mustRunZK(t, storeDir, "search", "search", "--project", projID, "--layer", "abstract")
	parseJSON(t, stdout, &results)
	if len(results) != 1 {
		t.Fatalf("expected 1 abstract search result, got %d", len(results))
	}
	if results[0]["title"] != "Abstract Search" {
		t.Errorf("expected title 'Abstract Search', got %v", results[0]["title"])
	}
}

func TestCLIGraph(t *testing.T) {
	storeDir := initStore(t)
	projID := createProject(t, storeDir, "graph-proj", "")

	n1 := createNote(t, storeDir, projID, "Graph Note A", "content a", nil)
	n2 := createNote(t, storeDir, projID, "Graph Note B", "content b", nil)
	mustRunZK(t, storeDir, "link", "add", n1, n2, "--type", "supports", "--weight", "0.9", "--project", projID)

	// Default mermaid format.
	stdout := mustRunZK(t, storeDir, "graph", "--project", projID)
	if !strings.Contains(stdout, "graph LR") {
		t.Errorf("expected mermaid output to contain 'graph LR', got:\n%s", stdout)
	}
	if !strings.Contains(stdout, n1) {
		t.Errorf("expected mermaid output to contain note ID %s", n1)
	}
	if !strings.Contains(stdout, n2) {
		t.Errorf("expected mermaid output to contain note ID %s", n2)
	}
	if !strings.Contains(stdout, "supports") {
		t.Errorf("expected mermaid output to contain 'supports'")
	}
}

func TestCLIGraphDOT(t *testing.T) {
	storeDir := initStore(t)
	projID := createProject(t, storeDir, "graph-dot-proj", "")

	n1 := createNote(t, storeDir, projID, "DOT Note A", "content a", nil)
	n2 := createNote(t, storeDir, projID, "DOT Note B", "content b", nil)
	mustRunZK(t, storeDir, "link", "add", n1, n2, "--type", "supports", "--weight", "0.8", "--project", projID)

	stdout := mustRunZK(t, storeDir, "graph", "--project", projID, "--format-graph", "dot")
	if !strings.Contains(stdout, "digraph zk") {
		t.Errorf("expected DOT output to contain 'digraph zk', got:\n%s", stdout)
	}
	if !strings.Contains(stdout, n1) {
		t.Errorf("expected DOT output to contain note ID %s", n1)
	}
	if !strings.Contains(stdout, n2) {
		t.Errorf("expected DOT output to contain note ID %s", n2)
	}
}

func TestCLIExplore(t *testing.T) {
	storeDir := initStore(t)
	projID := createProject(t, storeDir, "explore-proj", "")

	n1 := createNote(t, storeDir, projID, "Explore Hub", "hub content", nil)
	n2 := createNote(t, storeDir, projID, "Explore Target A", "target a", nil)
	n3 := createNote(t, storeDir, projID, "Explore Target B", "target b", nil)

	mustRunZK(t, storeDir, "link", "add", n1, n2, "--type", "supports", "--weight", "0.8", "--project", projID)
	mustRunZK(t, storeDir, "link", "add", n1, n3, "--type", "contradicts", "--weight", "0.6", "--project", projID)

	stdout := mustRunZK(t, storeDir, "explore", n1, "--project", projID)

	var result struct {
		Current struct {
			ID string `json:"id"`
		} `json:"current"`
		Outgoing []struct {
			NoteID       string `json:"note_id"`
			RelationType string `json:"relation_type"`
		} `json:"outgoing"`
		Incoming []struct {
			NoteID string `json:"note_id"`
		} `json:"incoming"`
	}
	parseJSON(t, stdout, &result)

	if result.Current.ID != n1 {
		t.Errorf("expected current.id=%s, got %s", n1, result.Current.ID)
	}
	if len(result.Outgoing) != 2 {
		t.Fatalf("expected 2 outgoing edges, got %d", len(result.Outgoing))
	}
	// N1 should have incoming backlinks from n2 and n3 (bidirectional links).
	if len(result.Incoming) < 1 {
		t.Errorf("expected at least 1 incoming edge (bidirectional), got %d", len(result.Incoming))
	}
}

func TestCLIExploreDepth(t *testing.T) {
	storeDir := initStore(t)
	projID := createProject(t, storeDir, "explore-depth-proj", "")

	n1 := createNote(t, storeDir, projID, "Depth Hub", "hub", nil)
	n2 := createNote(t, storeDir, projID, "Depth Mid", "mid", nil)
	n3 := createNote(t, storeDir, projID, "Depth Leaf", "leaf", nil)

	mustRunZK(t, storeDir, "link", "add", n1, n2, "--type", "supports", "--project", projID)
	mustRunZK(t, storeDir, "link", "add", n2, n3, "--type", "extends", "--project", projID)

	stdout := mustRunZK(t, storeDir, "explore", n1, "--project", projID, "--depth", "2")

	var result struct {
		Neighbors []struct {
			ID string `json:"id"`
		} `json:"neighbors"`
	}
	parseJSON(t, stdout, &result)

	if len(result.Neighbors) == 0 {
		t.Error("expected non-empty neighbors with --depth 2")
	}
}

func TestCLIExploreMD(t *testing.T) {
	storeDir := initStore(t)
	projID := createProject(t, storeDir, "explore-md-proj", "")

	n1 := createNote(t, storeDir, projID, "MD Hub", "hub content", nil)
	n2 := createNote(t, storeDir, projID, "MD Target", "target content", nil)
	n3 := createNote(t, storeDir, projID, "MD Other", "other content", nil)

	mustRunZK(t, storeDir, "link", "add", n1, n2, "--type", "supports", "--project", projID)
	mustRunZK(t, storeDir, "link", "add", n1, n3, "--type", "contradicts", "--project", projID)

	stdout := mustRunZK(t, storeDir, "explore", n1, "--project", projID, "--format", "md")

	if !strings.Contains(stdout, "# Exploring:") {
		t.Errorf("expected MD output to contain '# Exploring:', got:\n%s", stdout)
	}
	if !strings.Contains(stdout, "Outgoing Links") {
		t.Errorf("expected MD output to contain 'Outgoing Links'")
	}
	if !strings.Contains(stdout, "zk explore") {
		t.Errorf("expected MD output to contain navigation hints with 'zk explore'")
	}
}

func TestCLIReflectBloatedNote(t *testing.T) {
	storeDir := initStore(t)
	projID := createProject(t, storeDir, "bloat-proj", "")

	// Create a note with very long content (>1000 chars).
	longContent := strings.Repeat("This is a sentence that is repeated many times to create bloated content. ", 30)
	createNote(t, storeDir, projID, "Bloated Note", longContent, nil)

	// Run reflect.
	stdout := mustRunZK(t, storeDir, "reflect", "--project", projID, "--format", "json")
	var report struct {
		Insights []struct {
			Type string `json:"type"`
		} `json:"insights"`
	}
	parseJSON(t, stdout, &report)

	foundBloated := false
	for _, ins := range report.Insights {
		if ins.Type == "bloated_note" {
			foundBloated = true
			break
		}
	}
	if !foundBloated {
		t.Error("expected insight of type 'bloated_note' in reflect output for note with >1000 chars")
	}
}

func TestCLIFormatMDConsistency(t *testing.T) {
	storeDir := initStore(t)
	projID := createProject(t, storeDir, "md-fmt-proj", "")
	noteID := createNote(t, storeDir, projID, "MD Format Note", "Some content here", nil)

	// note get --format md should produce markdown, not JSON.
	stdout := mustRunZK(t, storeDir, "note", "get", noteID, "--project", projID, "--format", "md")
	if !strings.Contains(stdout, "# ") {
		t.Errorf("expected markdown heading '# ' in md output, got:\n%s", stdout)
	}
	if strings.HasPrefix(strings.TrimSpace(stdout), "{") {
		t.Errorf("md format output should not start with '{' (JSON), got:\n%s", stdout)
	}

	// link list --format md should not produce JSON even with 0 links.
	stdout = mustRunZK(t, storeDir, "link", "list", noteID, "--project", projID, "--format", "md")
	trimmed := strings.TrimSpace(stdout)
	if strings.HasPrefix(trimmed, "{") || strings.HasPrefix(trimmed, "[") {
		t.Errorf("link list --format md should not start with '{' or '[', got:\n%s", stdout)
	}
}

func TestLinkRemoveCrossProject(t *testing.T) {
	storeDir := initStore(t)
	p1 := createProject(t, storeDir, "proj1", "first")
	p2 := createProject(t, storeDir, "proj2", "second")
	n1 := createNote(t, storeDir, p1, "note1", "content1", nil)
	n2 := createNote(t, storeDir, p2, "note2", "content2", nil)

	mustRunZK(t, storeDir, "link", "add", n1, n2, "--project", p1, "--target-project", p2)

	// Remove the cross-project link.
	mustRunZK(t, storeDir, "link", "remove", n1, n2, "--project", p1, "--target-project", p2)

	// Verify link is gone.
	stdout := mustRunZK(t, storeDir, "link", "list", n1, "--project", p1, "--format", "json")
	if strings.Contains(stdout, n2) {
		t.Errorf("link to %s should be removed, got: %s", n2, stdout)
	}
}

func TestLinkRemoveByType(t *testing.T) {
	storeDir := initStore(t)
	projID := createProject(t, storeDir, "proj", "test")
	n1 := createNote(t, storeDir, projID, "note1", "c1", nil)
	n2 := createNote(t, storeDir, projID, "note2", "c2", nil)

	mustRunZK(t, storeDir, "link", "add", n1, n2, "--type", "supports", "--project", projID)
	mustRunZK(t, storeDir, "link", "add", n1, n2, "--type", "extends", "--project", projID)

	// Remove only the supports link.
	mustRunZK(t, storeDir, "link", "remove", n1, n2, "--type", "supports", "--project", projID)

	// extends link should still exist.
	stdout := mustRunZK(t, storeDir, "link", "list", n1, "--project", projID, "--format", "json")
	if !strings.Contains(stdout, "extends") {
		t.Errorf("extends link should remain, got: %s", stdout)
	}
	if strings.Contains(stdout, "supports") {
		t.Errorf("supports link should be removed, got: %s", stdout)
	}
}

func TestQuicknoteKoreanTitle(t *testing.T) {
	storeDir := initStore(t)
	projID := createProject(t, storeDir, "proj", "test")

	// 60 Korean characters — should be truncated to 50 runes without breaking UTF-8.
	text := "가나다라마바사아자차카타파하가나다라마바사아자차카타파하가나다라마바사아자차카타파하가나다라마바사아자차카타파하가나다라마바사아자차"
	stdout := mustRunZK(t, storeDir, "quicknote", text, "--project", projID, "--format", "json")

	var note map[string]interface{}
	parseJSON(t, stdout, &note)
	title, ok := note["title"].(string)
	if !ok {
		t.Fatalf("title not found in response")
	}

	// Title should be valid UTF-8 and at most 50 runes.
	runeCount := 0
	for range title {
		runeCount++
	}
	if runeCount > 50 {
		t.Errorf("title has %d runes, want <= 50", runeCount)
	}
}

func TestExportImportPreservesLayer(t *testing.T) {
	storeDir := initStore(t)
	projID := createProject(t, storeDir, "proj", "test")

	// Create an abstract note.
	stdout := mustRunZK(t, storeDir, "note", "create",
		"--title", "abstract note",
		"--content", "insight body",
		"--layer", "abstract",
		"--project", projID, "--format", "json")
	var created map[string]interface{}
	parseJSON(t, stdout, &created)
	noteID := created["id"].(string)

	// Export.
	tmpFile := filepath.Join(t.TempDir(), "export.yaml")
	mustRunZK(t, storeDir, "export", "--project", projID, "--output", tmpFile, "--format", "yaml")

	// Delete original.
	mustRunZK(t, storeDir, "note", "delete", noteID, "--force", "--project", projID)

	// Import.
	mustRunZK(t, storeDir, "import", "--file", tmpFile, "--project", projID, "--conflict", "overwrite")

	// Verify layer preserved.
	stdout = mustRunZK(t, storeDir, "note", "get", noteID, "--project", projID, "--format", "json")
	var imported map[string]interface{}
	parseJSON(t, stdout, &imported)
	layer, _ := imported["layer"].(string)
	if layer != "abstract" {
		t.Errorf("layer = %q, want %q", layer, "abstract")
	}
}

func TestDeleteNoteCrossProjectBacklink(t *testing.T) {
	storeDir := initStore(t)
	p1 := createProject(t, storeDir, "proj1", "first")
	p2 := createProject(t, storeDir, "proj2", "second")
	n1 := createNote(t, storeDir, p1, "note1", "c1", nil)
	n2 := createNote(t, storeDir, p2, "note2", "c2", nil)

	mustRunZK(t, storeDir, "link", "add", n1, n2, "--project", p1, "--target-project", p2)

	// Try deleting n2 without --force — should fail due to cross-project backlink.
	_, _, err := runZK(t, storeDir, "note", "delete", n2, "--project", p2)
	if err == nil {
		t.Errorf("expected delete to fail due to cross-project backlink")
	}
}

func TestDiagnoseCrossProjectLink(t *testing.T) {
	storeDir := initStore(t)
	p1 := createProject(t, storeDir, "proj1", "first")
	p2 := createProject(t, storeDir, "proj2", "second")
	n1 := createNote(t, storeDir, p1, "note1", "c1", nil)
	n2 := createNote(t, storeDir, p2, "note2", "c2", nil)

	mustRunZK(t, storeDir, "link", "add", n1, n2, "--project", p1, "--target-project", p2)

	// Diagnose should NOT report the cross-project link as broken.
	stdout := mustRunZK(t, storeDir, "diagnose", "--project", p1, "--format", "json")
	if strings.Contains(stdout, "broken link") {
		t.Errorf("diagnose should not report cross-project link as broken:\n%s", stdout)
	}
}

func TestTagReplaceDuplicates(t *testing.T) {
	storeDir := initStore(t)
	projID := createProject(t, storeDir, "proj", "test")
	noteID := createNote(t, storeDir, projID, "note", "content", []string{"draft", "draft", "review"})

	mustRunZK(t, storeDir, "tag", "replace", "draft", "final", "--project", projID)

	stdout := mustRunZK(t, storeDir, "note", "get", noteID, "--project", projID, "--format", "json")
	// Should have "final" once and no "draft".
	draftCount := strings.Count(stdout, `"draft"`)
	if draftCount > 0 {
		t.Errorf("draft tag should be fully replaced, found %d occurrences", draftCount)
	}
	finalCount := strings.Count(stdout, `"final"`)
	if finalCount != 1 {
		t.Errorf("final tag should appear exactly once, found %d", finalCount)
	}
}

func TestCLIDiagnoseMissingBacklink(t *testing.T) {
	storeDir := initStore(t)
	projID := createProject(t, storeDir, "backlink-proj", "")

	noteA := createNote(t, storeDir, projID, "Note A", "content A", nil)
	noteB := createNote(t, storeDir, projID, "Note B", "content B", nil)

	// Create bidirectional link via CLI.
	mustRunZK(t, storeDir, "link", "add", noteA, noteB, "--type", "supports", "--project", projID)

	// Manually remove the backlink from noteB's file to create inconsistency.
	noteBPath := filepath.Join(storeDir, "projects", projID, "notes", noteB+".md")
	data, err := os.ReadFile(noteBPath)
	if err != nil {
		t.Fatalf("read noteB file: %v", err)
	}
	// Replace the links section to empty.
	content := string(data)
	content = strings.Replace(content, "links:", "links: []", 1)
	// Remove the link entry lines (- target_id: ... etc).
	lines := strings.Split(content, "\n")
	var cleaned []string
	skip := false
	for _, line := range lines {
		if strings.Contains(line, "- target_id:") {
			skip = true
			continue
		}
		if skip && (strings.HasPrefix(strings.TrimSpace(line), "relation_type:") || strings.HasPrefix(strings.TrimSpace(line), "weight:")) {
			continue
		}
		skip = false
		cleaned = append(cleaned, line)
	}
	if err := os.WriteFile(noteBPath, []byte(strings.Join(cleaned, "\n")), 0644); err != nil {
		t.Fatalf("write noteB file: %v", err)
	}

	// Diagnose should detect missing backlink.
	stdout := mustRunZK(t, storeDir, "diagnose", "--project", projID)
	var report map[string]interface{}
	parseJSON(t, stdout, &report)
	warnings := report["warnings"].([]interface{})
	found := false
	for _, w := range warnings {
		msg := w.(map[string]interface{})["message"].(string)
		if strings.Contains(msg, "missing backlink") {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected 'missing backlink' warning, got: %s", stdout)
	}

	// Fix should repair the missing backlink.
	mustRunZK(t, storeDir, "diagnose", "--fix", "--project", projID)

	// Re-diagnose: should be healthy now.
	stdout = mustRunZK(t, storeDir, "diagnose", "--project", projID)
	parseJSON(t, stdout, &report)
	summary := report["summary"].(map[string]interface{})
	if summary["health_score"] != "healthy" {
		t.Fatalf("expected healthy after fix, got: %s", stdout)
	}
}

func TestCLIDiagnoseFixBrokenLink(t *testing.T) {
	storeDir := initStore(t)
	projID := createProject(t, storeDir, "broken-proj", "")

	noteA := createNote(t, storeDir, projID, "Note A", "content A", nil)
	noteB := createNote(t, storeDir, projID, "Note B", "content B", nil)

	mustRunZK(t, storeDir, "link", "add", noteA, noteB, "--type", "supports", "--project", projID)

	// Delete noteB's file to create a broken link.
	noteBPath := filepath.Join(storeDir, "projects", projID, "notes", noteB+".md")
	if err := os.Remove(noteBPath); err != nil {
		t.Fatalf("remove noteB file: %v", err)
	}

	// Diagnose should detect broken link.
	stdout := mustRunZK(t, storeDir, "diagnose", "--project", projID)
	var report map[string]interface{}
	parseJSON(t, stdout, &report)
	errors := report["errors"].([]interface{})
	found := false
	for _, e := range errors {
		msg := e.(map[string]interface{})["message"].(string)
		if strings.Contains(msg, "broken link") {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected 'broken link' error, got: %s", stdout)
	}

	// Fix should remove the broken link.
	mustRunZK(t, storeDir, "diagnose", "--fix", "--project", projID)

	// Re-diagnose: no more broken link errors.
	stdout = mustRunZK(t, storeDir, "diagnose", "--project", projID)
	parseJSON(t, stdout, &report)
	summary := report["summary"].(map[string]interface{})
	errCount := summary["error_count"].(float64)
	if errCount != 0 {
		t.Fatalf("expected 0 errors after fix, got: %s", stdout)
	}
}

func TestCLINoteRandom(t *testing.T) {
	storeDir := initStore(t)
	projID := createProject(t, storeDir, "rand-proj", "")

	// Empty store should fail.
	_, _, err := runZK(t, storeDir, "note", "random")
	if err == nil {
		t.Fatal("expected error for random on empty store")
	}

	// Create notes across project and global.
	noteA := createNote(t, storeDir, projID, "Project Note", "content A", []string{"a"})
	mustRunZK(t, storeDir, "note", "create", "--title", "Global Note", "--content", "content B", "--tags", "b")
	mustRunZK(t, storeDir, "note", "create", "--title", "Abstract Insight", "--content", "insight", "--layer", "abstract", "--project", projID)

	// Random should return a valid note.
	stdout := mustRunZK(t, storeDir, "note", "random")
	var note map[string]interface{}
	parseJSON(t, stdout, &note)
	id, ok := note["id"].(string)
	if !ok || !strings.HasPrefix(id, "N-") {
		t.Fatalf("expected valid note ID, got %v", note["id"])
	}

	// --layer abstract should only return abstract notes.
	for i := 0; i < 10; i++ {
		stdout = mustRunZK(t, storeDir, "note", "random", "--layer", "abstract")
		parseJSON(t, stdout, &note)
		if note["layer"] != "abstract" {
			t.Fatalf("expected abstract layer, got %v", note["layer"])
		}
	}

	// --layer concrete should never return the abstract note.
	for i := 0; i < 10; i++ {
		stdout = mustRunZK(t, storeDir, "note", "random", "--layer", "concrete")
		parseJSON(t, stdout, &note)
		if note["layer"] != "concrete" {
			t.Fatalf("expected concrete layer, got %v", note["layer"])
		}
	}

	_ = noteA
}

// Prevent unused import warning for fmt.
var _ = fmt.Sprintf
