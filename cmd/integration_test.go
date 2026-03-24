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
	tmp, err := os.MkdirTemp("", "nete-test-bin-*")
	if err != nil {
		panic("failed to create temp dir for binary: " + err.Error())
	}

	binName := "nete"
	if runtime.GOOS == "windows" {
		binName = "nete.exe"
	}
	zkBinary = filepath.Join(tmp, binName)

	// Build the binary from the project root (one level up from cmd/).
	projectRoot := filepath.Join("..", ".")
	cmd := exec.Command("go", "build", "-o", zkBinary, ".")
	cmd.Dir = projectRoot
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		panic("failed to build nete: " + err.Error())
	}

	code := m.Run()
	os.RemoveAll(tmp)
	os.Exit(code)
}

// runZK executes the nete binary with NETE_PATH pointing to storeDir.
func runZK(t *testing.T, storeDir string, args ...string) (string, string, error) {
	t.Helper()
	cmd := exec.Command(zkBinary, args...)
	cmd.Env = append(os.Environ(), "NETE_PATH="+storeDir)
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
		t.Fatalf("nete %s failed: %v\nstdout: %s\nstderr: %s", strings.Join(args, " "), err, stdout, stderr)
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

// initStore runs "nete init" on a fresh temp dir and returns the store path.
func initStore(t *testing.T) string {
	t.Helper()
	storeDir := t.TempDir()
	mustRunZK(t, storeDir, "init")
	return storeDir
}

// createNote creates a note and returns its int64 ID as a string.
func createNote(t *testing.T, storeDir, name, description string) string {
	t.Helper()
	args := []string{"note", "create", name}
	if description != "" {
		args = append(args, "--description", description)
	}
	stdout := mustRunZK(t, storeDir, args...)
	var note map[string]interface{}
	parseJSON(t, stdout, &note)
	id, ok := note["id"].(float64)
	if !ok {
		t.Fatalf("note create did not return a numeric id; output: %s", stdout)
	}
	return fmt.Sprintf("%d", int64(id))
}

// createMemo creates a memo and returns its int64 ID as a string.
func createMemo(t *testing.T, storeDir string, noteID string, title, content string, tags []string) string {
	t.Helper()
	args := []string{"memo", "create", "--title", title}
	if noteID != "0" && noteID != "" {
		args = append(args, "--note", noteID)
	}
	if content != "" {
		args = append(args, "--content", content)
	}
	if len(tags) > 0 {
		args = append(args, "--tags", strings.Join(tags, ","))
	}
	stdout := mustRunZK(t, storeDir, args...)
	var memo map[string]interface{}
	parseJSON(t, stdout, &memo)
	id, ok := memo["id"].(float64)
	if !ok {
		t.Fatalf("memo create did not return a numeric id; output: %s", stdout)
	}
	return fmt.Sprintf("%d", int64(id))
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

	// Verify store.db exists.
	dbPath := filepath.Join(storeDir, "store.db")
	if _, err := os.Stat(dbPath); os.IsNotExist(err) {
		t.Errorf("expected store.db to exist after init")
	}
}

func TestCLINoteCRUD(t *testing.T) {
	storeDir := initStore(t)

	// Create
	noteID := createNote(t, storeDir, "test-note", "a test note")

	// List: expect 1 note
	stdout := mustRunZK(t, storeDir, "note", "list")
	var notes []map[string]interface{}
	parseJSON(t, stdout, &notes)
	if len(notes) != 1 {
		t.Fatalf("expected 1 note, got %d", len(notes))
	}

	// Get: verify name and memo_count
	stdout = mustRunZK(t, storeDir, "note", "get", noteID)
	var detail map[string]interface{}
	parseJSON(t, stdout, &detail)
	if detail["name"] != "test-note" {
		t.Errorf("expected name 'test-note', got %v", detail["name"])
	}
	if mc, ok := detail["memo_count"].(float64); !ok || int(mc) != 0 {
		t.Errorf("expected memo_count=0, got %v", detail["memo_count"])
	}

	// Delete
	mustRunZK(t, storeDir, "note", "delete", noteID)

	// List: expect 0 notes
	stdout = mustRunZK(t, storeDir, "note", "list")
	parseJSON(t, stdout, &notes)
	if len(notes) != 0 {
		t.Fatalf("expected 0 notes after delete, got %d", len(notes))
	}
}

func TestCLIMemoCRUD(t *testing.T) {
	storeDir := initStore(t)
	noteID := createNote(t, storeDir, "memo-note", "")

	// Create memo
	memoID := createMemo(t, storeDir, noteID, "Test Title", "Test Content", []string{"alpha", "beta"})

	// Get memo: verify fields
	stdout := mustRunZK(t, storeDir, "memo", "get", memoID)
	var memo map[string]interface{}
	parseJSON(t, stdout, &memo)
	if memo["title"] != "Test Title" {
		t.Errorf("expected title 'Test Title', got %v", memo["title"])
	}
	if memo["content"] != "Test Content" {
		t.Errorf("expected content 'Test Content', got %v", memo["content"])
	}
	tags, ok := memo["tags"].([]interface{})
	if !ok || len(tags) != 2 {
		t.Errorf("expected 2 tags, got %v", memo["tags"])
	}

	// Update memo title
	stdout = mustRunZK(t, storeDir, "memo", "update", memoID, "--title", "New Title")
	parseJSON(t, stdout, &memo)
	if memo["title"] != "New Title" {
		t.Errorf("expected updated title 'New Title', got %v", memo["title"])
	}

	// List memos: expect 1
	stdout = mustRunZK(t, storeDir, "memo", "list", "--note", noteID)
	var memos []map[string]interface{}
	parseJSON(t, stdout, &memos)
	if len(memos) != 1 {
		t.Fatalf("expected 1 memo, got %d", len(memos))
	}

	// Delete memo
	mustRunZK(t, storeDir, "memo", "delete", memoID)

	// List memos: expect 0
	stdout = mustRunZK(t, storeDir, "memo", "list", "--note", noteID)
	parseJSON(t, stdout, &memos)
	if len(memos) != 0 {
		t.Fatalf("expected 0 memos after delete, got %d", len(memos))
	}
}

func TestCLILinkWorkflow(t *testing.T) {
	storeDir := initStore(t)

	m1 := createMemo(t, storeDir, "0", "Memo One", "Content one", nil)
	m2 := createMemo(t, storeDir, "0", "Memo Two", "Content two", nil)

	// Add link
	mustRunZK(t, storeDir, "link", "add", m1, m2, "--type", "supports", "--weight", "0.8")

	// List links for m1
	stdout := mustRunZK(t, storeDir, "link", "list", m1)
	var linkResult struct {
		Outgoing []map[string]interface{} `json:"outgoing"`
		Incoming []map[string]interface{} `json:"incoming"`
	}
	parseJSON(t, stdout, &linkResult)
	if len(linkResult.Outgoing) != 1 {
		t.Fatalf("expected 1 outgoing link, got %d", len(linkResult.Outgoing))
	}
	if linkResult.Outgoing[0]["relation_type"] != "supports" {
		t.Errorf("expected relation_type 'supports', got %v", linkResult.Outgoing[0]["relation_type"])
	}

	// m2 should have an incoming link from m1
	stdout = mustRunZK(t, storeDir, "link", "list", m2)
	var linkResult2 struct {
		Outgoing []map[string]interface{} `json:"outgoing"`
		Incoming []map[string]interface{} `json:"incoming"`
	}
	parseJSON(t, stdout, &linkResult2)
	if len(linkResult2.Incoming) != 1 {
		t.Errorf("expected 1 incoming link on m2, got %d", len(linkResult2.Incoming))
	}
}

func TestCLISearch(t *testing.T) {
	storeDir := initStore(t)

	createMemo(t, storeDir, "0", "Redis Caching", "Redis is a fast in-memory store", []string{"database", "cache"})
	createMemo(t, storeDir, "0", "PostgreSQL Guide", "PostgreSQL is a relational database", []string{"database", "sql"})
	createMemo(t, storeDir, "0", "Go Concurrency", "Goroutines and channels", []string{"golang", "concurrency"})

	// Search for "Redis": should match 1 memo
	stdout := mustRunZK(t, storeDir, "search", "Redis")
	var results []map[string]interface{}
	parseJSON(t, stdout, &results)
	if len(results) != 1 {
		t.Fatalf("expected 1 search result for 'Redis', got %d", len(results))
	}
	if results[0]["title"] != "Redis Caching" {
		t.Errorf("expected title 'Redis Caching', got %v", results[0]["title"])
	}

	// Search for "database": should match 2 memos
	stdout = mustRunZK(t, storeDir, "search", "database")
	parseJSON(t, stdout, &results)
	if len(results) != 2 {
		t.Fatalf("expected 2 search results for 'database', got %d", len(results))
	}

	// Search with --tags filter
	stdout = mustRunZK(t, storeDir, "search", "database", "--tags", "sql")
	parseJSON(t, stdout, &results)
	if len(results) != 1 {
		t.Fatalf("expected 1 search result for 'database' with tag 'sql', got %d", len(results))
	}

	// Search with no matches
	stdout = mustRunZK(t, storeDir, "search", "xyznonexistent")
	parseJSON(t, stdout, &results)
	if results == nil {
		results = []map[string]interface{}{}
	}
	if len(results) != 0 {
		t.Errorf("expected 0 results for non-matching query, got %d", len(results))
	}
}

func TestCLIDiagnose(t *testing.T) {
	storeDir := initStore(t)

	m1 := createMemo(t, storeDir, "0", "Diag Memo 1", "Content 1", nil)
	m2 := createMemo(t, storeDir, "0", "Diag Memo 2", "Content 2", nil)

	// Link them so they are not orphans
	mustRunZK(t, storeDir, "link", "add", m1, m2, "--type", "related", "--weight", "0.5")

	// Run diagnose
	stdout := mustRunZK(t, storeDir, "diagnose")
	var report struct {
		TotalMemos int           `json:"total_memos"`
		TotalLinks int           `json:"total_links"`
		Errors     []interface{} `json:"errors"`
		Warnings   []interface{} `json:"warnings"`
		Summary    struct {
			ErrorCount   int    `json:"error_count"`
			WarningCount int    `json:"warning_count"`
			HealthScore  string `json:"health_score"`
		} `json:"summary"`
	}
	parseJSON(t, stdout, &report)

	if report.TotalMemos != 2 {
		t.Errorf("expected 2 total_memos, got %d", report.TotalMemos)
	}
	if report.TotalLinks != 1 {
		t.Errorf("expected 1 total_links, got %d", report.TotalLinks)
	}
	if report.Summary.HealthScore != "healthy" {
		t.Errorf("expected health_score 'healthy', got %q", report.Summary.HealthScore)
	}
}

func TestCLIDiagnoseOrphanWarning(t *testing.T) {
	storeDir := initStore(t)

	// Create a single unlinked memo -- should be flagged as orphan.
	createMemo(t, storeDir, "0", "Lonely", "alone", nil)

	stdout := mustRunZK(t, storeDir, "diagnose")
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
		t.Fatal("expected at least 1 warning for orphan memo")
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
}

func TestCLIMemoUpdate(t *testing.T) {
	storeDir := initStore(t)

	memoID := createMemo(t, storeDir, "0", "Original", "original content", []string{"old"})

	// Update multiple fields
	stdout := mustRunZK(t, storeDir, "memo", "update", memoID,
		"--title", "Updated",
		"--content", "new content",
		"--tags", "new",
		"--status", "archived",
	)
	var memo map[string]interface{}
	parseJSON(t, stdout, &memo)

	if memo["title"] != "Updated" {
		t.Errorf("expected title 'Updated', got %v", memo["title"])
	}
	if memo["content"] != "new content" {
		t.Errorf("expected content 'new content', got %v", memo["content"])
	}

	// Check metadata.status
	meta, ok := memo["metadata"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected metadata to be a map, got %T", memo["metadata"])
	}
	if meta["status"] != "archived" {
		t.Errorf("expected status 'archived', got %v", meta["status"])
	}
}

func TestCLISearchNoNote(t *testing.T) {
	// Search without --note uses all memos; should still work after init.
	storeDir := initStore(t)

	stdout := mustRunZK(t, storeDir, "search", "anything")
	var results []map[string]interface{}
	parseJSON(t, stdout, &results)
	if results == nil {
		results = []map[string]interface{}{}
	}
	if len(results) != 0 {
		t.Errorf("expected 0 results in empty store, got %d", len(results))
	}
}

func TestCLILinkRemove(t *testing.T) {
	storeDir := initStore(t)

	m1 := createMemo(t, storeDir, "0", "A", "a", nil)
	m2 := createMemo(t, storeDir, "0", "B", "b", nil)

	mustRunZK(t, storeDir, "link", "add", m1, m2, "--type", "related")

	// Remove the link
	mustRunZK(t, storeDir, "link", "remove", m1, m2, "--type", "related")

	// Verify links are gone
	stdout := mustRunZK(t, storeDir, "link", "list", m1)
	var linkResult struct {
		Outgoing []interface{} `json:"outgoing"`
		Incoming []interface{} `json:"incoming"`
	}
	parseJSON(t, stdout, &linkResult)
	if len(linkResult.Outgoing) != 0 {
		t.Errorf("expected 0 outgoing links after remove, got %d", len(linkResult.Outgoing))
	}
}

func TestCLIVersionFlag(t *testing.T) {
	storeDir := t.TempDir()
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

func TestCLIMemoCreateRequiresTitle(t *testing.T) {
	storeDir := initStore(t)

	// memo create without --title should fail
	_, _, err := runZK(t, storeDir, "memo", "create", "--content", "no title")
	if err == nil {
		t.Error("expected memo create without --title to fail")
	}
}

func TestCLIOutputFormat(t *testing.T) {
	storeDir := initStore(t)
	createMemo(t, storeDir, "0", "Formatted", "content", nil)

	// YAML format
	stdout := mustRunZK(t, storeDir, "memo", "list", "--format", "yaml")
	if !strings.Contains(stdout, "title:") {
		t.Errorf("expected YAML output to contain 'title:', got: %s", stdout)
	}

	// JSON format (default)
	stdout = mustRunZK(t, storeDir, "memo", "list", "--format", "json")
	var memos []map[string]interface{}
	if err := json.Unmarshal([]byte(stdout), &memos); err != nil {
		t.Errorf("expected valid JSON output, got parse error: %v", err)
	}
}

func TestCLIMemoLayer(t *testing.T) {
	storeDir := initStore(t)

	// Create a concrete memo (default layer).
	createMemo(t, storeDir, "0", "Concrete Memo", "concrete content", nil)

	// Create an abstract memo using --layer abstract.
	stdout := mustRunZK(t, storeDir, "memo", "create", "--title", "Abstract Memo", "--content", "abstract content", "--layer", "abstract")
	var absMemo map[string]interface{}
	parseJSON(t, stdout, &absMemo)

	// memo list --layer concrete -> only concrete memo
	stdout = mustRunZK(t, storeDir, "memo", "list", "--layer", "concrete")
	var memos []map[string]interface{}
	parseJSON(t, stdout, &memos)
	if len(memos) != 1 {
		t.Fatalf("expected 1 concrete memo, got %d", len(memos))
	}

	// memo list --layer abstract -> only abstract memo
	stdout = mustRunZK(t, storeDir, "memo", "list", "--layer", "abstract")
	parseJSON(t, stdout, &memos)
	if len(memos) != 1 {
		t.Fatalf("expected 1 abstract memo, got %d", len(memos))
	}

	// memo list (no --layer) -> both memos
	stdout = mustRunZK(t, storeDir, "memo", "list")
	parseJSON(t, stdout, &memos)
	if len(memos) != 2 {
		t.Fatalf("expected 2 memos without layer filter, got %d", len(memos))
	}
}

func TestCLIReflect(t *testing.T) {
	storeDir := initStore(t)

	// Create 3 concrete memos.
	m1 := createMemo(t, storeDir, "0", "Memo One", "content one", nil)
	m2 := createMemo(t, storeDir, "0", "Memo Two", "content two", nil)
	createMemo(t, storeDir, "0", "Memo Three", "content three", nil) // orphan

	// Link m1 -> m2 with --type contradicts (tension).
	mustRunZK(t, storeDir, "link", "add", m1, m2, "--type", "contradicts")

	// Run reflect.
	stdout := mustRunZK(t, storeDir, "reflect", "--format", "json")
	var report struct {
		Insights []struct {
			Type        string  `json:"type"`
			SourceMemos []int64 `json:"source_memos"`
			Suggestion  string  `json:"suggestion"`
		} `json:"insights"`
		Stats struct {
			ConcreteCount int `json:"concrete_count"`
			AbstractCount int `json:"abstract_count"`
		} `json:"stats"`
	}
	parseJSON(t, stdout, &report)

	if report.Stats.ConcreteCount != 3 {
		t.Errorf("expected concrete_count=3, got %d", report.Stats.ConcreteCount)
	}

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

	m1 := createMemo(t, storeDir, "0", "Thesis", "thesis content", nil)
	m2 := createMemo(t, storeDir, "0", "Antithesis", "antithesis content", nil)
	mustRunZK(t, storeDir, "link", "add", m1, m2, "--type", "contradicts")

	// Run reflect --apply.
	mustRunZK(t, storeDir, "reflect", "--apply", "--format", "json")

	// memo list --layer abstract -> at least 1 abstract memo was created.
	stdout := mustRunZK(t, storeDir, "memo", "list", "--layer", "abstract")
	var abstractMemos []map[string]interface{}
	parseJSON(t, stdout, &abstractMemos)
	if len(abstractMemos) < 1 {
		t.Fatalf("expected at least 1 abstract memo after reflect --apply, got %d", len(abstractMemos))
	}

	// Verify source memo has an "abstracts" outgoing link.
	stdout = mustRunZK(t, storeDir, "link", "list", m1)
	var linkResult struct {
		Outgoing []map[string]interface{} `json:"outgoing"`
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
		t.Error("expected source memo to have an 'abstracts' link after reflect --apply")
	}
}

func TestCLIGraph(t *testing.T) {
	storeDir := initStore(t)

	m1 := createMemo(t, storeDir, "0", "Graph Memo A", "content a", nil)
	m2 := createMemo(t, storeDir, "0", "Graph Memo B", "content b", nil)
	mustRunZK(t, storeDir, "link", "add", m1, m2, "--type", "supports", "--weight", "0.9")

	// Default mermaid format.
	stdout := mustRunZK(t, storeDir, "graph")
	if !strings.Contains(stdout, "graph LR") {
		t.Errorf("expected mermaid output to contain 'graph LR', got:\n%s", stdout)
	}
	if !strings.Contains(stdout, "supports") {
		t.Errorf("expected mermaid output to contain 'supports'")
	}
}

func TestCLIGraphDOT(t *testing.T) {
	storeDir := initStore(t)

	m1 := createMemo(t, storeDir, "0", "DOT Memo A", "content a", nil)
	m2 := createMemo(t, storeDir, "0", "DOT Memo B", "content b", nil)
	mustRunZK(t, storeDir, "link", "add", m1, m2, "--type", "supports", "--weight", "0.8")

	stdout := mustRunZK(t, storeDir, "graph", "--format-graph", "dot")
	if !strings.Contains(stdout, "digraph nete") {
		t.Errorf("expected DOT output to contain 'digraph nete', got:\n%s", stdout)
	}
}

func TestCLIExplore(t *testing.T) {
	storeDir := initStore(t)

	m1 := createMemo(t, storeDir, "0", "Explore Hub", "hub content", nil)
	m2 := createMemo(t, storeDir, "0", "Explore Target A", "target a", nil)
	m3 := createMemo(t, storeDir, "0", "Explore Target B", "target b", nil)

	mustRunZK(t, storeDir, "link", "add", m1, m2, "--type", "supports", "--weight", "0.8")
	mustRunZK(t, storeDir, "link", "add", m1, m3, "--type", "contradicts", "--weight", "0.6")

	stdout := mustRunZK(t, storeDir, "explore", m1)

	var result struct {
		Current struct {
			ID int64 `json:"id"`
		} `json:"current"`
		Outgoing []struct {
			MemoID       int64  `json:"memo_id"`
			RelationType string `json:"relation_type"`
		} `json:"outgoing"`
		Incoming []struct {
			MemoID int64 `json:"memo_id"`
		} `json:"incoming"`
	}
	parseJSON(t, stdout, &result)

	if fmt.Sprintf("%d", result.Current.ID) != m1 {
		t.Errorf("expected current.id=%s, got %d", m1, result.Current.ID)
	}
	if len(result.Outgoing) != 2 {
		t.Fatalf("expected 2 outgoing edges, got %d", len(result.Outgoing))
	}
}

func TestCLIExploreDepth(t *testing.T) {
	storeDir := initStore(t)

	m1 := createMemo(t, storeDir, "0", "Depth Hub", "hub", nil)
	m2 := createMemo(t, storeDir, "0", "Depth Mid", "mid", nil)
	m3 := createMemo(t, storeDir, "0", "Depth Leaf", "leaf", nil)

	mustRunZK(t, storeDir, "link", "add", m1, m2, "--type", "supports")
	mustRunZK(t, storeDir, "link", "add", m2, m3, "--type", "extends")

	stdout := mustRunZK(t, storeDir, "explore", m1, "--depth", "2")

	var result struct {
		Neighbors []struct {
			ID int64 `json:"id"`
		} `json:"neighbors"`
	}
	parseJSON(t, stdout, &result)

	if len(result.Neighbors) == 0 {
		t.Error("expected non-empty neighbors with --depth 2")
	}
}

func TestCLIReflectBloatedNote(t *testing.T) {
	storeDir := initStore(t)

	longContent := strings.Repeat("This is a sentence that is repeated many times to create bloated content. ", 30)
	createMemo(t, storeDir, "0", "Bloated Memo", longContent, nil)

	stdout := mustRunZK(t, storeDir, "reflect", "--format", "json")
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
		t.Error("expected insight of type 'bloated_note' in reflect output")
	}
}

func TestCLIMemoRandom(t *testing.T) {
	storeDir := initStore(t)

	// Empty store should fail.
	_, _, err := runZK(t, storeDir, "memo", "random")
	if err == nil {
		t.Fatal("expected error for random on empty store")
	}

	// Create memos.
	createMemo(t, storeDir, "0", "Memo A", "content A", []string{"a"})
	createMemo(t, storeDir, "0", "Memo B", "content B", []string{"b"})
	mustRunZK(t, storeDir, "memo", "create", "--title", "Abstract Insight", "--content", "insight", "--layer", "abstract")

	// Random should return a valid memo.
	stdout := mustRunZK(t, storeDir, "memo", "random")
	var memo map[string]interface{}
	parseJSON(t, stdout, &memo)
	_, ok := memo["id"].(float64)
	if !ok {
		t.Fatalf("expected valid numeric memo ID, got %v", memo["id"])
	}

	// --layer abstract should only return abstract memos.
	for i := 0; i < 10; i++ {
		stdout = mustRunZK(t, storeDir, "memo", "random", "--layer", "abstract")
		parseJSON(t, stdout, &memo)
		if memo["layer"] != "abstract" {
			t.Fatalf("expected abstract layer, got %v", memo["layer"])
		}
	}
}

func TestCLIQuickmemo(t *testing.T) {
	storeDir := initStore(t)

	// 60 Korean characters -- should be truncated to 50 runes.
	text := "가나다라마바사아자차카타파하가나다라마바사아자차카타파하가나다라마바사아자차카타파하가나다라마바사아자차카타파하가나다라마바사아자차"
	stdout := mustRunZK(t, storeDir, "quickmemo", text, "--format", "json")

	var memo map[string]interface{}
	parseJSON(t, stdout, &memo)
	title, ok := memo["title"].(string)
	if !ok {
		t.Fatalf("title not found in response")
	}

	runeCount := 0
	for range title {
		runeCount++
	}
	if runeCount > 50 {
		t.Errorf("title has %d runes, want <= 50", runeCount)
	}
}

func TestCLIMemoMove(t *testing.T) {
	storeDir := initStore(t)
	noteID := createNote(t, storeDir, "move-note", "target")

	// Create a global memo (note_id=0).
	memoID := createMemo(t, storeDir, "0", "Movable", "content", nil)

	// Move to the note.
	mustRunZK(t, storeDir, "memo", "move", memoID, noteID)

	// Verify by getting the memo.
	stdout := mustRunZK(t, storeDir, "memo", "get", memoID)
	var memo map[string]interface{}
	parseJSON(t, stdout, &memo)
	noteIDFloat, _ := memo["note_id"].(float64)
	if fmt.Sprintf("%d", int64(noteIDFloat)) != noteID {
		t.Errorf("expected note_id=%s after move, got %v", noteID, memo["note_id"])
	}
}

func TestTagReplace(t *testing.T) {
	storeDir := initStore(t)
	memoID := createMemo(t, storeDir, "0", "tagged", "content", []string{"draft", "review"})

	mustRunZK(t, storeDir, "tag", "replace", "draft", "final")

	stdout := mustRunZK(t, storeDir, "memo", "get", memoID, "--format", "json")
	draftCount := strings.Count(stdout, `"draft"`)
	if draftCount > 0 {
		t.Errorf("draft tag should be replaced, found %d occurrences", draftCount)
	}
	finalCount := strings.Count(stdout, `"final"`)
	if finalCount != 1 {
		t.Errorf("final tag should appear exactly once, found %d", finalCount)
	}
}

func TestLinkRemoveByType(t *testing.T) {
	storeDir := initStore(t)
	m1 := createMemo(t, storeDir, "0", "memo1", "c1", nil)
	m2 := createMemo(t, storeDir, "0", "memo2", "c2", nil)

	mustRunZK(t, storeDir, "link", "add", m1, m2, "--type", "supports")
	mustRunZK(t, storeDir, "link", "add", m1, m2, "--type", "extends")

	// Remove only the supports link.
	mustRunZK(t, storeDir, "link", "remove", m1, m2, "--type", "supports")

	// extends link should still exist.
	stdout := mustRunZK(t, storeDir, "link", "list", m1, "--format", "json")
	if !strings.Contains(stdout, "extends") {
		t.Errorf("extends link should remain, got: %s", stdout)
	}
	if strings.Contains(stdout, "supports") {
		t.Errorf("supports link should be removed, got: %s", stdout)
	}
}

// Prevent unused import warning for fmt.
var _ = fmt.Sprintf
