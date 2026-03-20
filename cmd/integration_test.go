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

// Prevent unused import warning for fmt.
var _ = fmt.Sprintf
