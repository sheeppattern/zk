package store

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/sheeppattern/zk/internal/model"
)

func TestInit(t *testing.T) {
	dir := t.TempDir()
	s := NewStore(dir)
	if err := s.Init(); err != nil {
		t.Fatalf("Init() error: %v", err)
	}

	for _, sub := range []string{
		"projects",
		filepath.Join("global", "notes"),
		"trash",
	} {
		path := filepath.Join(dir, sub)
		info, err := os.Stat(path)
		if err != nil {
			t.Fatalf("expected directory %s to exist: %v", sub, err)
		}
		if !info.IsDir() {
			t.Fatalf("%s is not a directory", sub)
		}
	}

	cfgPath := filepath.Join(dir, "config.yaml")
	if _, err := os.Stat(cfgPath); err != nil {
		t.Fatalf("config.yaml does not exist: %v", err)
	}
}

func TestConfigRoundTrip(t *testing.T) {
	dir := t.TempDir()
	s := NewStore(dir)
	if err := s.Init(); err != nil {
		t.Fatalf("Init() error: %v", err)
	}

	cfg := &model.Config{
		StorePath:      dir,
		DefaultProject: "my-project",
		DefaultFormat:  "json",
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
	if loaded.DefaultProject != cfg.DefaultProject {
		t.Fatalf("DefaultProject = %q; want %q", loaded.DefaultProject, cfg.DefaultProject)
	}
	if loaded.DefaultFormat != cfg.DefaultFormat {
		t.Fatalf("DefaultFormat = %q; want %q", loaded.DefaultFormat, cfg.DefaultFormat)
	}
	if len(loaded.CustomRelationTypes) != 2 {
		t.Fatalf("CustomRelationTypes length = %d; want 2", len(loaded.CustomRelationTypes))
	}
	if loaded.CustomRelationTypes[0] != "custom-a" || loaded.CustomRelationTypes[1] != "custom-b" {
		t.Fatalf("CustomRelationTypes = %v; want [custom-a custom-b]", loaded.CustomRelationTypes)
	}
}

func TestProjectCRUD(t *testing.T) {
	dir := t.TempDir()
	s := NewStore(dir)
	if err := s.Init(); err != nil {
		t.Fatalf("Init() error: %v", err)
	}

	p := model.NewProject("Test Project", "desc")
	if err := s.CreateProject(p); err != nil {
		t.Fatalf("CreateProject() error: %v", err)
	}

	got, err := s.GetProject(p.ID)
	if err != nil {
		t.Fatalf("GetProject() error: %v", err)
	}
	if got.Name != "Test Project" {
		t.Fatalf("Name = %q; want %q", got.Name, "Test Project")
	}
	if got.Description != "desc" {
		t.Fatalf("Description = %q; want %q", got.Description, "desc")
	}

	projects, err := s.ListProjects()
	if err != nil {
		t.Fatalf("ListProjects() error: %v", err)
	}
	if len(projects) != 1 {
		t.Fatalf("ListProjects() returned %d; want 1", len(projects))
	}

	if err := s.DeleteProject(p.ID); err != nil {
		t.Fatalf("DeleteProject() error: %v", err)
	}

	projects, err = s.ListProjects()
	if err != nil {
		t.Fatalf("ListProjects() error: %v", err)
	}
	if len(projects) != 0 {
		t.Fatalf("ListProjects() returned %d after delete; want 0", len(projects))
	}
}

func TestNoteCRUD(t *testing.T) {
	dir := t.TempDir()
	s := NewStore(dir)
	if err := s.Init(); err != nil {
		t.Fatalf("Init() error: %v", err)
	}

	note := model.NewNote("My Note", "Hello world", []string{"tag1"})
	if err := s.CreateNote(note); err != nil {
		t.Fatalf("CreateNote() error: %v", err)
	}

	got, err := s.GetNote("", note.ID)
	if err != nil {
		t.Fatalf("GetNote() error: %v", err)
	}
	if got.Title != "My Note" {
		t.Fatalf("Title = %q; want %q", got.Title, "My Note")
	}
	if got.Content != "Hello world" {
		t.Fatalf("Content = %q; want %q", got.Content, "Hello world")
	}
	if len(got.Tags) != 1 || got.Tags[0] != "tag1" {
		t.Fatalf("Tags = %v; want [tag1]", got.Tags)
	}

	// Update
	got.Title = "Updated Title"
	if err := s.UpdateNote(got); err != nil {
		t.Fatalf("UpdateNote() error: %v", err)
	}

	got2, err := s.GetNote("", note.ID)
	if err != nil {
		t.Fatalf("GetNote() after update error: %v", err)
	}
	if got2.Title != "Updated Title" {
		t.Fatalf("Title after update = %q; want %q", got2.Title, "Updated Title")
	}

	// Delete
	if err := s.DeleteNote("", note.ID); err != nil {
		t.Fatalf("DeleteNote() error: %v", err)
	}

	_, err = s.GetNote("", note.ID)
	if err == nil {
		t.Fatal("GetNote() after delete should return error")
	}
}

func TestNoteProjectScoping(t *testing.T) {
	dir := t.TempDir()
	s := NewStore(dir)
	if err := s.Init(); err != nil {
		t.Fatalf("Init() error: %v", err)
	}

	projA := model.NewProject("Project A", "")
	if err := s.CreateProject(projA); err != nil {
		t.Fatalf("CreateProject() error: %v", err)
	}

	noteA := model.NewNote("Note in A", "content A", nil)
	noteA.ProjectID = projA.ID
	if err := s.CreateNote(noteA); err != nil {
		t.Fatalf("CreateNote(projA) error: %v", err)
	}

	noteGlobal := model.NewNote("Global Note", "content global", nil)
	if err := s.CreateNote(noteGlobal); err != nil {
		t.Fatalf("CreateNote(global) error: %v", err)
	}

	notesA, err := s.ListNotes(projA.ID)
	if err != nil {
		t.Fatalf("ListNotes(projA) error: %v", err)
	}
	if len(notesA) != 1 {
		t.Fatalf("ListNotes(projA) = %d; want 1", len(notesA))
	}

	notesGlobal, err := s.ListNotes("")
	if err != nil {
		t.Fatalf("ListNotes(\"\") error: %v", err)
	}
	if len(notesGlobal) != 1 {
		t.Fatalf("ListNotes(\"\") = %d; want 1", len(notesGlobal))
	}
}

func TestMarshalUnmarshalRoundTrip(t *testing.T) {
	dir := t.TempDir()
	s := NewStore(dir)
	if err := s.Init(); err != nil {
		t.Fatalf("Init() error: %v", err)
	}

	note := model.NewNote("테스트 노트", "본문 내용\n여러 줄", []string{"tag1", "tag2"})
	note.Links = []model.Link{
		{TargetID: "N-AAAAAA", RelationType: "related", Weight: 0.8},
	}
	if err := s.CreateNote(note); err != nil {
		t.Fatalf("CreateNote() error: %v", err)
	}

	got, err := s.GetNote("", note.ID)
	if err != nil {
		t.Fatalf("GetNote() error: %v", err)
	}
	if got.Title != "테스트 노트" {
		t.Fatalf("Title = %q; want %q", got.Title, "테스트 노트")
	}
	if got.Content != "본문 내용\n여러 줄" {
		t.Fatalf("Content = %q; want %q", got.Content, "본문 내용\n여러 줄")
	}
	if len(got.Tags) != 2 || got.Tags[0] != "tag1" || got.Tags[1] != "tag2" {
		t.Fatalf("Tags = %v; want [tag1 tag2]", got.Tags)
	}
	if len(got.Links) != 1 {
		t.Fatalf("Links length = %d; want 1", len(got.Links))
	}
	if got.Links[0].TargetID != "N-AAAAAA" {
		t.Fatalf("Link TargetID = %q; want %q", got.Links[0].TargetID, "N-AAAAAA")
	}
	if got.Links[0].RelationType != "related" {
		t.Fatalf("Link RelationType = %q; want %q", got.Links[0].RelationType, "related")
	}
	if got.Links[0].Weight != 0.8 {
		t.Fatalf("Link Weight = %f; want 0.8", got.Links[0].Weight)
	}
}

func TestDeleteNoteTrash(t *testing.T) {
	dir := t.TempDir()
	s := NewStore(dir)
	if err := s.Init(); err != nil {
		t.Fatalf("Init() error: %v", err)
	}

	note := model.NewNote("Trash Me", "bye", nil)
	if err := s.CreateNote(note); err != nil {
		t.Fatalf("CreateNote() error: %v", err)
	}

	if err := s.DeleteNote("", note.ID); err != nil {
		t.Fatalf("DeleteNote() error: %v", err)
	}

	trashDir := filepath.Join(dir, "trash")
	entries, err := os.ReadDir(trashDir)
	if err != nil {
		t.Fatalf("ReadDir(trash) error: %v", err)
	}

	found := false
	for _, e := range entries {
		if filepath.Ext(e.Name()) == ".md" {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("deleted note file not found in trash directory")
	}
}

func TestMoveNote(t *testing.T) {
	dir := t.TempDir()
	s := NewStore(dir)
	if err := s.Init(); err != nil {
		t.Fatalf("Init() error: %v", err)
	}

	projA := model.NewProject("Project A", "")
	projB := model.NewProject("Project B", "")
	if err := s.CreateProject(projA); err != nil {
		t.Fatalf("CreateProject(A) error: %v", err)
	}
	if err := s.CreateProject(projB); err != nil {
		t.Fatalf("CreateProject(B) error: %v", err)
	}

	note := model.NewNote("Moveable", "content", nil)
	note.ProjectID = projA.ID
	if err := s.CreateNote(note); err != nil {
		t.Fatalf("CreateNote() error: %v", err)
	}

	if err := s.MoveNote(note.ID, projA.ID, projB.ID); err != nil {
		t.Fatalf("MoveNote() error: %v", err)
	}

	got, err := s.GetNote(projB.ID, note.ID)
	if err != nil {
		t.Fatalf("GetNote(projB) after move error: %v", err)
	}
	if got.ProjectID != projB.ID {
		t.Fatalf("ProjectID after move = %q; want %q", got.ProjectID, projB.ID)
	}

	_, err = s.GetNote(projA.ID, note.ID)
	if err == nil {
		t.Fatal("GetNote(projA) after move should return error")
	}
}

func TestListNotesPartial(t *testing.T) {
	dir := t.TempDir()
	s := NewStore(dir)
	if err := s.Init(); err != nil {
		t.Fatalf("Init() error: %v", err)
	}

	// Create a valid note.
	note := model.NewNote("Valid Note", "valid content", nil)
	if err := s.CreateNote(note); err != nil {
		t.Fatalf("CreateNote() error: %v", err)
	}

	// Write a corrupt .md file directly into the global notes directory.
	corruptPath := filepath.Join(dir, "global", "notes", "N-BADONE.md")
	if err := os.WriteFile(corruptPath, []byte("not valid frontmatter at all"), 0644); err != nil {
		t.Fatalf("WriteFile(corrupt) error: %v", err)
	}

	notes, noteErrors := s.ListNotesPartial("")
	if len(notes) != 1 {
		t.Fatalf("ListNotesPartial notes = %d; want 1", len(notes))
	}
	if len(noteErrors) != 1 {
		t.Fatalf("ListNotesPartial errors = %d; want 1", len(noteErrors))
	}
}
