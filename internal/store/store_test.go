package store

import (
	"path/filepath"
	"testing"

	"github.com/sheeppattern/zk/internal/model"
)

func newTestStore(t *testing.T) *Store {
	t.Helper()
	dbPath := filepath.Join(t.TempDir(), "test.db")
	s, err := NewStore(dbPath)
	if err != nil {
		t.Fatalf("NewStore() error: %v", err)
	}
	if err := s.Init(); err != nil {
		t.Fatalf("Init() error: %v", err)
	}
	t.Cleanup(func() { s.Close() })
	return s
}

// ---------------------------------------------------------------------------
// Note CRUD
// ---------------------------------------------------------------------------

func TestNoteCRUD(t *testing.T) {
	s := newTestStore(t)

	// Create.
	note, err := s.CreateNote("Test Note", "A description")
	if err != nil {
		t.Fatalf("CreateNote() error: %v", err)
	}
	if note.ID == 0 {
		t.Fatal("CreateNote() returned ID 0")
	}
	if note.Name != "Test Note" {
		t.Fatalf("Name = %q; want %q", note.Name, "Test Note")
	}

	// Get.
	got, err := s.GetNote(note.ID)
	if err != nil {
		t.Fatalf("GetNote() error: %v", err)
	}
	if got.Name != "Test Note" {
		t.Fatalf("Name = %q; want %q", got.Name, "Test Note")
	}
	if got.Description != "A description" {
		t.Fatalf("Description = %q; want %q", got.Description, "A description")
	}

	// List.
	notes, err := s.ListNotes()
	if err != nil {
		t.Fatalf("ListNotes() error: %v", err)
	}
	if len(notes) != 1 {
		t.Fatalf("ListNotes() returned %d; want 1", len(notes))
	}

	// Delete.
	if err := s.DeleteNote(note.ID); err != nil {
		t.Fatalf("DeleteNote() error: %v", err)
	}
	notes, err = s.ListNotes()
	if err != nil {
		t.Fatalf("ListNotes() error: %v", err)
	}
	if len(notes) != 0 {
		t.Fatalf("ListNotes() returned %d after delete; want 0", len(notes))
	}
}

func TestDeleteNote_WithMemos(t *testing.T) {
	s := newTestStore(t)

	note, err := s.CreateNote("Has Memos", "")
	if err != nil {
		t.Fatalf("CreateNote() error: %v", err)
	}

	memo := &model.Memo{Title: "test", NoteID: note.ID}
	if err := s.CreateMemo(memo); err != nil {
		t.Fatalf("CreateMemo() error: %v", err)
	}

	// Should fail because note has memos.
	if err := s.DeleteNote(note.ID); err == nil {
		t.Fatal("DeleteNote() should fail when note has memos")
	}
}

// ---------------------------------------------------------------------------
// Memo CRUD with Tags
// ---------------------------------------------------------------------------

func TestMemoCRUD(t *testing.T) {
	s := newTestStore(t)

	// Create.
	memo := &model.Memo{
		Title:   "My Memo",
		Content: "Hello world",
		Tags:    []string{"tag1", "tag2"},
		Layer:   model.LayerConcrete,
		NoteID:  0,
	}
	if err := s.CreateMemo(memo); err != nil {
		t.Fatalf("CreateMemo() error: %v", err)
	}
	if memo.ID == 0 {
		t.Fatal("CreateMemo() did not set ID")
	}
	if memo.Metadata.Status != model.StatusActive {
		t.Fatalf("Status = %q; want %q", memo.Metadata.Status, model.StatusActive)
	}

	// Get and verify tag JSON round-trip.
	got, err := s.GetMemo(memo.ID)
	if err != nil {
		t.Fatalf("GetMemo() error: %v", err)
	}
	if got.Title != "My Memo" {
		t.Fatalf("Title = %q; want %q", got.Title, "My Memo")
	}
	if got.Content != "Hello world" {
		t.Fatalf("Content = %q; want %q", got.Content, "Hello world")
	}
	if len(got.Tags) != 2 || got.Tags[0] != "tag1" || got.Tags[1] != "tag2" {
		t.Fatalf("Tags = %v; want [tag1 tag2]", got.Tags)
	}
	if got.Layer != model.LayerConcrete {
		t.Fatalf("Layer = %q; want %q", got.Layer, model.LayerConcrete)
	}

	// Update.
	got.Title = "Updated Title"
	got.Tags = []string{"tag1", "tag2", "tag3"}
	if err := s.UpdateMemo(got); err != nil {
		t.Fatalf("UpdateMemo() error: %v", err)
	}
	got2, err := s.GetMemo(memo.ID)
	if err != nil {
		t.Fatalf("GetMemo() after update error: %v", err)
	}
	if got2.Title != "Updated Title" {
		t.Fatalf("Title after update = %q; want %q", got2.Title, "Updated Title")
	}
	if len(got2.Tags) != 3 {
		t.Fatalf("Tags length after update = %d; want 3", len(got2.Tags))
	}

	// List by note.
	memos, err := s.ListMemos(0)
	if err != nil {
		t.Fatalf("ListMemos() error: %v", err)
	}
	if len(memos) != 1 {
		t.Fatalf("ListMemos(0) returned %d; want 1", len(memos))
	}

	// ListAll.
	all, err := s.ListAllMemos()
	if err != nil {
		t.Fatalf("ListAllMemos() error: %v", err)
	}
	if len(all) != 1 {
		t.Fatalf("ListAllMemos() returned %d; want 1", len(all))
	}
}

func TestMemoEmptyTags(t *testing.T) {
	s := newTestStore(t)

	memo := &model.Memo{Title: "No Tags"}
	if err := s.CreateMemo(memo); err != nil {
		t.Fatalf("CreateMemo() error: %v", err)
	}

	got, err := s.GetMemo(memo.ID)
	if err != nil {
		t.Fatalf("GetMemo() error: %v", err)
	}
	if got.Tags == nil {
		t.Fatal("Tags is nil; want empty slice")
	}
	if len(got.Tags) != 0 {
		t.Fatalf("Tags length = %d; want 0", len(got.Tags))
	}
}

func TestDeleteMemoToTrash(t *testing.T) {
	s := newTestStore(t)

	memo := &model.Memo{Title: "Trash Me", Content: "bye"}
	if err := s.CreateMemo(memo); err != nil {
		t.Fatalf("CreateMemo() error: %v", err)
	}

	if err := s.DeleteMemo(memo.ID); err != nil {
		t.Fatalf("DeleteMemo() error: %v", err)
	}

	// Memo should be gone.
	_, err := s.GetMemo(memo.ID)
	if err == nil {
		t.Fatal("GetMemo() after delete should return error")
	}

	// Trash should have the data.
	var trashData string
	err = s.db.QueryRow("SELECT original_data FROM trash WHERE id = ?", memo.ID).Scan(&trashData)
	if err != nil {
		t.Fatalf("query trash error: %v", err)
	}
	if trashData == "" {
		t.Fatal("trash data is empty")
	}
}

func TestDeleteMemoCleanupLinks(t *testing.T) {
	s := newTestStore(t)

	m1 := &model.Memo{Title: "M1"}
	m2 := &model.Memo{Title: "M2"}
	if err := s.CreateMemo(m1); err != nil {
		t.Fatalf("CreateMemo(m1) error: %v", err)
	}
	if err := s.CreateMemo(m2); err != nil {
		t.Fatalf("CreateMemo(m2) error: %v", err)
	}

	if err := s.AddLink(m1.ID, m2.ID, model.RelSupports, 0.8); err != nil {
		t.Fatalf("AddLink() error: %v", err)
	}

	// Delete m1 — links should also be removed.
	if err := s.DeleteMemo(m1.ID); err != nil {
		t.Fatalf("DeleteMemo() error: %v", err)
	}

	// Verify links are cleaned up.
	outgoing, incoming, err := s.ListLinks(m2.ID)
	if err != nil {
		t.Fatalf("ListLinks() error: %v", err)
	}
	if len(outgoing)+len(incoming) != 0 {
		t.Fatalf("expected 0 links after memo delete, got outgoing=%d incoming=%d", len(outgoing), len(incoming))
	}
}

func TestMoveMemo(t *testing.T) {
	s := newTestStore(t)

	noteA, err := s.CreateNote("Note A", "")
	if err != nil {
		t.Fatalf("CreateNote(A) error: %v", err)
	}
	noteB, err := s.CreateNote("Note B", "")
	if err != nil {
		t.Fatalf("CreateNote(B) error: %v", err)
	}

	memo := &model.Memo{Title: "Moveable", NoteID: noteA.ID}
	if err := s.CreateMemo(memo); err != nil {
		t.Fatalf("CreateMemo() error: %v", err)
	}

	if err := s.MoveMemo(memo.ID, noteB.ID); err != nil {
		t.Fatalf("MoveMemo() error: %v", err)
	}

	got, err := s.GetMemo(memo.ID)
	if err != nil {
		t.Fatalf("GetMemo() after move error: %v", err)
	}
	if got.NoteID != noteB.ID {
		t.Fatalf("NoteID after move = %d; want %d", got.NoteID, noteB.ID)
	}

	// Should be in noteB's list, not noteA's.
	memosA, _ := s.ListMemos(noteA.ID)
	memosB, _ := s.ListMemos(noteB.ID)
	if len(memosA) != 0 {
		t.Fatalf("ListMemos(A) = %d; want 0", len(memosA))
	}
	if len(memosB) != 1 {
		t.Fatalf("ListMemos(B) = %d; want 1", len(memosB))
	}
}

// ---------------------------------------------------------------------------
// Links
// ---------------------------------------------------------------------------

func TestLinkAddRemoveList(t *testing.T) {
	s := newTestStore(t)

	// Create two memos.
	m1 := &model.Memo{Title: "Memo 1"}
	m2 := &model.Memo{Title: "Memo 2"}
	if err := s.CreateMemo(m1); err != nil {
		t.Fatalf("CreateMemo(1) error: %v", err)
	}
	if err := s.CreateMemo(m2); err != nil {
		t.Fatalf("CreateMemo(2) error: %v", err)
	}

	// Add a single link (no bidirectional duplication).
	if err := s.AddLink(m1.ID, m2.ID, model.RelSupports, 0.9); err != nil {
		t.Fatalf("AddLink() error: %v", err)
	}

	// Verify single storage.
	var count int
	s.db.QueryRow("SELECT COUNT(*) FROM links").Scan(&count)
	if count != 1 {
		t.Fatalf("links count = %d; want 1 (no bidirectional duplication)", count)
	}

	// Verify bidirectional query.
	outgoing, incoming, err := s.ListLinks(m1.ID)
	if err != nil {
		t.Fatalf("ListLinks(m1) error: %v", err)
	}
	if len(outgoing) != 1 {
		t.Fatalf("outgoing from m1 = %d; want 1", len(outgoing))
	}
	if len(incoming) != 0 {
		t.Fatalf("incoming to m1 = %d; want 0", len(incoming))
	}

	outgoing2, incoming2, err := s.ListLinks(m2.ID)
	if err != nil {
		t.Fatalf("ListLinks(m2) error: %v", err)
	}
	if len(outgoing2) != 0 {
		t.Fatalf("outgoing from m2 = %d; want 0", len(outgoing2))
	}
	if len(incoming2) != 1 {
		t.Fatalf("incoming to m2 = %d; want 1", len(incoming2))
	}

	// Remove link.
	if err := s.RemoveLink(m1.ID, m2.ID, model.RelSupports); err != nil {
		t.Fatalf("RemoveLink() error: %v", err)
	}
	outgoing3, incoming3, err := s.ListLinks(m1.ID)
	if err != nil {
		t.Fatalf("ListLinks after remove error: %v", err)
	}
	if len(outgoing3) != 0 || len(incoming3) != 0 {
		t.Fatalf("links after remove: outgoing=%d, incoming=%d; want 0, 0", len(outgoing3), len(incoming3))
	}
}

func TestLinkBFS(t *testing.T) {
	s := newTestStore(t)

	// Create a chain: m1 -> m2 -> m3.
	m1 := &model.Memo{Title: "M1"}
	m2 := &model.Memo{Title: "M2"}
	m3 := &model.Memo{Title: "M3"}
	for _, m := range []*model.Memo{m1, m2, m3} {
		if err := s.CreateMemo(m); err != nil {
			t.Fatalf("CreateMemo() error: %v", err)
		}
	}

	s.AddLink(m1.ID, m2.ID, model.RelRelated, 0.5)
	s.AddLink(m2.ID, m3.ID, model.RelRelated, 0.5)

	// BFS from m1, depth 1 should find m2 but not m3.
	links1, err := s.ListLinksBFS(m1.ID, 1)
	if err != nil {
		t.Fatalf("ListLinksBFS(depth=1) error: %v", err)
	}
	if len(links1) != 1 {
		t.Fatalf("BFS depth=1 links = %d; want 1", len(links1))
	}

	// BFS from m1, depth 2 should find both links.
	links2, err := s.ListLinksBFS(m1.ID, 2)
	if err != nil {
		t.Fatalf("ListLinksBFS(depth=2) error: %v", err)
	}
	if len(links2) != 2 {
		t.Fatalf("BFS depth=2 links = %d; want 2", len(links2))
	}
}

// ---------------------------------------------------------------------------
// FTS5 Search
// ---------------------------------------------------------------------------

func TestSearchMemos_FTS5(t *testing.T) {
	s := newTestStore(t)

	m1 := &model.Memo{Title: "Quantum Computing", Content: "Introduction to quantum mechanics and computing"}
	m2 := &model.Memo{Title: "Classical Music", Content: "History of classical music"}
	m3 := &model.Memo{Title: "Quantum Physics", Content: "Advanced quantum field theory"}
	for _, m := range []*model.Memo{m1, m2, m3} {
		if err := s.CreateMemo(m); err != nil {
			t.Fatalf("CreateMemo() error: %v", err)
		}
	}

	// Search for "quantum".
	results, err := s.SearchMemos("quantum", SearchOptions{})
	if err != nil {
		t.Fatalf("SearchMemos() error: %v", err)
	}
	if len(results) != 2 {
		t.Fatalf("search 'quantum' returned %d; want 2", len(results))
	}

	// Search for "classical".
	results2, err := s.SearchMemos("classical", SearchOptions{})
	if err != nil {
		t.Fatalf("SearchMemos() error: %v", err)
	}
	if len(results2) != 1 {
		t.Fatalf("search 'classical' returned %d; want 1", len(results2))
	}
}

func TestSearchMemos_WithFilters(t *testing.T) {
	s := newTestStore(t)

	note, _ := s.CreateNote("Science", "")

	m1 := &model.Memo{Title: "Quantum", Content: "quantum stuff", NoteID: note.ID, Layer: model.LayerConcrete}
	m2 := &model.Memo{Title: "Quantum Abstract", Content: "quantum insights", NoteID: 0, Layer: model.LayerAbstract}
	s.CreateMemo(m1)
	s.CreateMemo(m2)

	// Filter by layer.
	results, err := s.SearchMemos("quantum", SearchOptions{Layer: model.LayerConcrete})
	if err != nil {
		t.Fatalf("SearchMemos() error: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("search with layer filter returned %d; want 1", len(results))
	}
	if results[0].Title != "Quantum" {
		t.Fatalf("filtered result title = %q; want %q", results[0].Title, "Quantum")
	}

	// Filter by note.
	results2, err := s.SearchMemos("quantum", SearchOptions{NoteID: note.ID})
	if err != nil {
		t.Fatalf("SearchMemos() error: %v", err)
	}
	if len(results2) != 1 {
		t.Fatalf("search with note filter returned %d; want 1", len(results2))
	}
}

func TestSearchMemos_Limit(t *testing.T) {
	s := newTestStore(t)

	for i := 0; i < 10; i++ {
		m := &model.Memo{Title: "Test memo", Content: "test content for searching"}
		s.CreateMemo(m)
	}

	results, err := s.SearchMemos("test", SearchOptions{Limit: 3})
	if err != nil {
		t.Fatalf("SearchMemos() error: %v", err)
	}
	if len(results) != 3 {
		t.Fatalf("search with limit returned %d; want 3", len(results))
	}
}

// ---------------------------------------------------------------------------
// Config
// ---------------------------------------------------------------------------

func TestConfigRoundTrip(t *testing.T) {
	s := newTestStore(t)

	cfg := &model.Config{
		StorePath:           "/tmp/test",
		DefaultNote:         "my-note",
		DefaultFormat:       "json",
		DefaultAuthor:       "tester",
		CustomRelationTypes: []string{"custom-a", "custom-b"},
	}
	if err := s.SaveConfig(cfg); err != nil {
		t.Fatalf("SaveConfig() error: %v", err)
	}

	loaded, err := s.LoadConfig()
	if err != nil {
		t.Fatalf("LoadConfig() error: %v", err)
	}
	if loaded.StorePath != cfg.StorePath {
		t.Fatalf("StorePath = %q; want %q", loaded.StorePath, cfg.StorePath)
	}
	if loaded.DefaultNote != cfg.DefaultNote {
		t.Fatalf("DefaultNote = %q; want %q", loaded.DefaultNote, cfg.DefaultNote)
	}
	if loaded.DefaultFormat != cfg.DefaultFormat {
		t.Fatalf("DefaultFormat = %q; want %q", loaded.DefaultFormat, cfg.DefaultFormat)
	}
	if loaded.DefaultAuthor != cfg.DefaultAuthor {
		t.Fatalf("DefaultAuthor = %q; want %q", loaded.DefaultAuthor, cfg.DefaultAuthor)
	}
	if len(loaded.CustomRelationTypes) != 2 {
		t.Fatalf("CustomRelationTypes length = %d; want 2", len(loaded.CustomRelationTypes))
	}
	if loaded.CustomRelationTypes[0] != "custom-a" || loaded.CustomRelationTypes[1] != "custom-b" {
		t.Fatalf("CustomRelationTypes = %v; want [custom-a custom-b]", loaded.CustomRelationTypes)
	}
}

func TestConfigUpsert(t *testing.T) {
	s := newTestStore(t)

	cfg1 := &model.Config{StorePath: "/first", DefaultFormat: "md"}
	if err := s.SaveConfig(cfg1); err != nil {
		t.Fatalf("SaveConfig(1) error: %v", err)
	}

	cfg2 := &model.Config{StorePath: "/second", DefaultFormat: "json"}
	if err := s.SaveConfig(cfg2); err != nil {
		t.Fatalf("SaveConfig(2) error: %v", err)
	}

	loaded, err := s.LoadConfig()
	if err != nil {
		t.Fatalf("LoadConfig() error: %v", err)
	}
	if loaded.StorePath != "/second" {
		t.Fatalf("StorePath = %q; want %q", loaded.StorePath, "/second")
	}
	if loaded.DefaultFormat != "json" {
		t.Fatalf("DefaultFormat = %q; want %q", loaded.DefaultFormat, "json")
	}
}

// ---------------------------------------------------------------------------
// Memo with metadata
// ---------------------------------------------------------------------------

func TestMemoMetadataRoundTrip(t *testing.T) {
	s := newTestStore(t)

	memo := &model.Memo{
		Title:   "With Metadata",
		Content: "content",
		Tags:    []string{"go", "test"},
		Layer:   model.LayerAbstract,
		Metadata: model.Metadata{
			Source:  "https://example.com",
			Author:  "test-author",
			Summary: "A summary",
		},
	}
	if err := s.CreateMemo(memo); err != nil {
		t.Fatalf("CreateMemo() error: %v", err)
	}

	got, err := s.GetMemo(memo.ID)
	if err != nil {
		t.Fatalf("GetMemo() error: %v", err)
	}
	if got.Metadata.Source != "https://example.com" {
		t.Fatalf("Source = %q; want %q", got.Metadata.Source, "https://example.com")
	}
	if got.Metadata.Author != "test-author" {
		t.Fatalf("Author = %q; want %q", got.Metadata.Author, "test-author")
	}
	if got.Metadata.Summary != "A summary" {
		t.Fatalf("Summary = %q; want %q", got.Metadata.Summary, "A summary")
	}
	if got.Metadata.Status != model.StatusActive {
		t.Fatalf("Status = %q; want %q", got.Metadata.Status, model.StatusActive)
	}
	if got.Metadata.CreatedAt.IsZero() {
		t.Fatal("CreatedAt is zero")
	}
	if got.Metadata.UpdatedAt.IsZero() {
		t.Fatal("UpdatedAt is zero")
	}
	if got.Layer != model.LayerAbstract {
		t.Fatalf("Layer = %q; want %q", got.Layer, model.LayerAbstract)
	}
}

// ---------------------------------------------------------------------------
// Unicode round-trip
// ---------------------------------------------------------------------------

func TestMemoUnicodeRoundTrip(t *testing.T) {
	s := newTestStore(t)

	memo := &model.Memo{
		Title:   "테스트 메모",
		Content: "본문 내용\n여러 줄",
		Tags:    []string{"한국어", "테스트"},
	}
	if err := s.CreateMemo(memo); err != nil {
		t.Fatalf("CreateMemo() error: %v", err)
	}

	got, err := s.GetMemo(memo.ID)
	if err != nil {
		t.Fatalf("GetMemo() error: %v", err)
	}
	if got.Title != "테스트 메모" {
		t.Fatalf("Title = %q; want %q", got.Title, "테스트 메모")
	}
	if got.Content != "본문 내용\n여러 줄" {
		t.Fatalf("Content = %q; want %q", got.Content, "본문 내용\n여러 줄")
	}
	if len(got.Tags) != 2 || got.Tags[0] != "한국어" {
		t.Fatalf("Tags = %v; want [한국어 테스트]", got.Tags)
	}
}
