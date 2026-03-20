package store

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/ymh/zk/internal/model"
	"gopkg.in/yaml.v3"
)

// Store manages file-system-based persistence for notes, projects, and config.
type Store struct {
	rootPath string
}

// NewStore creates a new Store rooted at the given directory path.
func NewStore(rootPath string) *Store {
	return &Store{rootPath: rootPath}
}

// Init creates the full directory structure and a default config.yaml.
func (s *Store) Init() error {
	dirs := []string{
		s.rootPath,
		filepath.Join(s.rootPath, "projects"),
		filepath.Join(s.rootPath, "global", "notes"),
	}
	for _, d := range dirs {
		if err := os.MkdirAll(d, 0755); err != nil {
			return fmt.Errorf("create directory %s: %w", d, err)
		}
	}

	cfgPath := filepath.Join(s.rootPath, "config.yaml")
	if _, err := os.Stat(cfgPath); os.IsNotExist(err) {
		cfg := model.DefaultConfig()
		cfg.StorePath = s.rootPath
		if err := s.SaveConfig(cfg); err != nil {
			return fmt.Errorf("write default config: %w", err)
		}
	}
	return nil
}

// ---------------------------------------------------------------------------
// Config
// ---------------------------------------------------------------------------

// LoadConfig reads and parses config.yaml from the store root.
func (s *Store) LoadConfig() (*model.Config, error) {
	data, err := os.ReadFile(filepath.Join(s.rootPath, "config.yaml"))
	if err != nil {
		return nil, fmt.Errorf("read config: %w", err)
	}
	var cfg model.Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parse config: %w", err)
	}
	return &cfg, nil
}

// SaveConfig writes the given Config as config.yaml in the store root.
func (s *Store) SaveConfig(cfg *model.Config) error {
	data, err := yaml.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("marshal config: %w", err)
	}
	return os.WriteFile(filepath.Join(s.rootPath, "config.yaml"), data, 0644)
}

// ---------------------------------------------------------------------------
// Projects
// ---------------------------------------------------------------------------

// CreateProject persists a new project to disk.
func (s *Store) CreateProject(p *model.Project) error {
	dir := filepath.Join(s.rootPath, "projects", p.ID)
	if err := os.MkdirAll(filepath.Join(dir, "notes"), 0755); err != nil {
		return fmt.Errorf("create project dirs: %w", err)
	}
	data, err := yaml.Marshal(p)
	if err != nil {
		return fmt.Errorf("marshal project: %w", err)
	}
	return os.WriteFile(filepath.Join(dir, "project.yaml"), data, 0644)
}

// ListProjects returns all projects found under the projects/ directory.
func (s *Store) ListProjects() ([]*model.Project, error) {
	projectsDir := filepath.Join(s.rootPath, "projects")
	entries, err := os.ReadDir(projectsDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("read projects dir: %w", err)
	}

	var projects []*model.Project
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		p, err := s.GetProject(e.Name())
		if err != nil {
			return nil, err
		}
		projects = append(projects, p)
	}
	return projects, nil
}

// GetProject reads a single project by its ID.
func (s *Store) GetProject(id string) (*model.Project, error) {
	data, err := os.ReadFile(filepath.Join(s.rootPath, "projects", id, "project.yaml"))
	if err != nil {
		return nil, fmt.Errorf("read project %s: %w", id, err)
	}
	var p model.Project
	if err := yaml.Unmarshal(data, &p); err != nil {
		return nil, fmt.Errorf("parse project %s: %w", id, err)
	}
	return &p, nil
}

// DeleteProject removes a project directory and all its notes.
func (s *Store) DeleteProject(id string) error {
	dir := filepath.Join(s.rootPath, "projects", id)
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		return fmt.Errorf("project %s not found", id)
	}
	return os.RemoveAll(dir)
}

// ---------------------------------------------------------------------------
// Notes
// ---------------------------------------------------------------------------

// CreateNote writes a new note to disk in the appropriate directory.
func (s *Store) CreateNote(note *model.Note) error {
	dir := s.notesDir(note.ProjectID)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("create notes dir: %w", err)
	}
	data, err := s.marshalNote(note)
	if err != nil {
		return err
	}
	return os.WriteFile(s.noteFilePath(note.ProjectID, note.ID), data, 0644)
}

// GetNote reads a single note by project and note ID.
// Pass an empty projectID for global notes.
func (s *Store) GetNote(projectID, noteID string) (*model.Note, error) {
	data, err := os.ReadFile(s.noteFilePath(projectID, noteID))
	if err != nil {
		return nil, fmt.Errorf("read note %s: %w", noteID, err)
	}
	return s.unmarshalNote(data)
}

// UpdateNote overwrites an existing note on disk, automatically setting UpdatedAt.
func (s *Store) UpdateNote(note *model.Note) error {
	note.Metadata.UpdatedAt = time.Now()
	data, err := s.marshalNote(note)
	if err != nil {
		return err
	}
	path := s.noteFilePath(note.ProjectID, note.ID)
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return fmt.Errorf("note %s not found", note.ID)
	}
	return os.WriteFile(path, data, 0644)
}

// DeleteNote removes a note file from disk.
func (s *Store) DeleteNote(projectID, noteID string) error {
	path := s.noteFilePath(projectID, noteID)
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return fmt.Errorf("note %s not found", noteID)
	}
	return os.Remove(path)
}

// ListNotes returns all notes in a project. Pass an empty projectID for global notes.
func (s *Store) ListNotes(projectID string) ([]*model.Note, error) {
	dir := s.notesDir(projectID)
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("read notes dir: %w", err)
	}

	var notes []*model.Note
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".md") {
			continue
		}
		noteID := strings.TrimSuffix(e.Name(), ".md")
		n, err := s.GetNote(projectID, noteID)
		if err != nil {
			return nil, err
		}
		notes = append(notes, n)
	}
	return notes, nil
}

// ---------------------------------------------------------------------------
// Internal helpers
// ---------------------------------------------------------------------------

// notesDir returns the notes directory for a project, or global if projectID is empty.
func (s *Store) notesDir(projectID string) string {
	if projectID == "" {
		return filepath.Join(s.rootPath, "global", "notes")
	}
	return filepath.Join(s.rootPath, "projects", projectID, "notes")
}

// noteFilePath returns the full file path for a note's markdown file.
func (s *Store) noteFilePath(projectID, noteID string) string {
	return filepath.Join(s.notesDir(projectID), noteID+".md")
}

// marshalNote converts a Note into Markdown with YAML frontmatter.
func (s *Store) marshalNote(note *model.Note) ([]byte, error) {
	fm := model.NoteFrontmatter{
		ID:        note.ID,
		Title:     note.Title,
		Tags:      note.Tags,
		Links:     note.Links,
		Metadata:  note.Metadata,
		ProjectID: note.ProjectID,
	}
	yamlData, err := yaml.Marshal(&fm)
	if err != nil {
		return nil, fmt.Errorf("marshal note frontmatter: %w", err)
	}

	var buf strings.Builder
	buf.WriteString("---\n")
	buf.Write(yamlData)
	buf.WriteString("---\n")
	buf.WriteString(note.Content)

	return []byte(buf.String()), nil
}

// unmarshalNote parses Markdown with YAML frontmatter into a Note.
func (s *Store) unmarshalNote(data []byte) (*model.Note, error) {
	content := string(data)

	// Strip leading "---\n"
	if !strings.HasPrefix(content, "---\n") {
		return nil, fmt.Errorf("invalid note format: missing opening frontmatter delimiter")
	}
	content = content[4:] // skip "---\n"

	// Find closing "---\n"
	idx := strings.Index(content, "\n---\n")
	if idx == -1 {
		return nil, fmt.Errorf("invalid note format: missing closing frontmatter delimiter")
	}

	yamlPart := content[:idx]
	bodyPart := content[idx+5:] // skip "\n---\n"

	var fm model.NoteFrontmatter
	if err := yaml.Unmarshal([]byte(yamlPart), &fm); err != nil {
		return nil, fmt.Errorf("parse note frontmatter: %w", err)
	}

	note := &model.Note{
		ID:        fm.ID,
		Title:     fm.Title,
		Content:   bodyPart,
		Tags:      fm.Tags,
		Links:     fm.Links,
		Metadata:  fm.Metadata,
		ProjectID: fm.ProjectID,
	}
	return note, nil
}
