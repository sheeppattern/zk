package model

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/uuid"
)

// Relation type constants.
const (
	RelRelated     = "related"
	RelSupports    = "supports"
	RelContradicts = "contradicts"
	RelExtends     = "extends"
	RelCauses      = "causes"
	RelExampleOf   = "example-of"
)

// Note status constants.
const (
	StatusActive   = "active"
	StatusArchived = "archived"
)

// Note represents an atomic memory note in the Zettelkasten.
type Note struct {
	ID        string   `yaml:"id"        json:"id"`
	Title     string   `yaml:"title"     json:"title"`
	Content   string   `yaml:"-"         json:"-"`
	Tags      []string `yaml:"tags"      json:"tags"`
	Links     []Link   `yaml:"links"     json:"links"`
	Metadata  Metadata `yaml:"metadata"  json:"metadata"`
	ProjectID string   `yaml:"project_id,omitempty" json:"project_id,omitempty"`
}

// NoteFrontmatter is the YAML-serializable portion of a Note (without Content).
type NoteFrontmatter struct {
	ID        string   `yaml:"id"        json:"id"`
	Title     string   `yaml:"title"     json:"title"`
	Tags      []string `yaml:"tags"      json:"tags"`
	Links     []Link   `yaml:"links"     json:"links"`
	Metadata  Metadata `yaml:"metadata"  json:"metadata"`
	ProjectID string   `yaml:"project_id,omitempty" json:"project_id,omitempty"`
}

// Link represents a weighted, typed connection between notes.
type Link struct {
	TargetID     string  `yaml:"target_id"     json:"target_id"`
	RelationType string  `yaml:"relation_type" json:"relation_type"`
	Weight       float64 `yaml:"weight"        json:"weight"`
}

// Metadata holds auto-recorded metadata for a note.
type Metadata struct {
	CreatedAt time.Time `yaml:"created_at" json:"created_at"`
	UpdatedAt time.Time `yaml:"updated_at" json:"updated_at"`
	Source    string    `yaml:"source"     json:"source"`
	Status   string    `yaml:"status"     json:"status"`
}

// Project groups related notes together.
type Project struct {
	ID          string    `yaml:"id"          json:"id"`
	Name        string    `yaml:"name"        json:"name"`
	Description string    `yaml:"description" json:"description"`
	CreatedAt   time.Time `yaml:"created_at"  json:"created_at"`
	UpdatedAt   time.Time `yaml:"updated_at"  json:"updated_at"`
}

// Config holds CLI configuration.
type Config struct {
	StorePath     string `yaml:"store_path"      json:"store_path"`
	DefaultProject string `yaml:"default_project" json:"default_project"`
	DefaultFormat string `yaml:"default_format"  json:"default_format"`
}

// GenerateID produces an ID like "N-ABCDEF" using the given prefix
// and 6 uppercase alphanumeric characters derived from a UUID.
func GenerateID(prefix string) string {
	raw := uuid.New().String()
	clean := strings.ReplaceAll(raw, "-", "")
	upper := strings.ToUpper(clean)
	return fmt.Sprintf("%s-%s", prefix, upper[:6])
}

// NewNote creates a new Note with an auto-generated ID, timestamps, and active status.
func NewNote(title, content string, tags []string) *Note {
	now := time.Now()
	if tags == nil {
		tags = []string{}
	}
	return &Note{
		ID:      GenerateID("N"),
		Title:   title,
		Content: content,
		Tags:    tags,
		Links:   []Link{},
		Metadata: Metadata{
			CreatedAt: now,
			UpdatedAt: now,
			Status:    StatusActive,
		},
	}
}

// NewProject creates a new Project with an auto-generated ID and timestamps.
func NewProject(name, description string) *Project {
	now := time.Now()
	return &Project{
		ID:          GenerateID("P"),
		Name:        name,
		Description: description,
		CreatedAt:   now,
		UpdatedAt:   now,
	}
}

// DefaultConfig returns a Config with sensible defaults.
// StorePath defaults to ~/.zk-memory.
func DefaultConfig() *Config {
	home, err := os.UserHomeDir()
	if err != nil {
		home = "."
	}
	return &Config{
		StorePath:     filepath.Join(home, ".zk-memory"),
		DefaultProject: "",
		DefaultFormat: "md",
	}
}

// ValidRelationTypes returns all valid relation type constants.
func ValidRelationTypes() []string {
	return []string{
		RelRelated,
		RelSupports,
		RelContradicts,
		RelExtends,
		RelCauses,
		RelExampleOf,
	}
}

// IsValidRelationType checks whether the given relation type is valid.
func IsValidRelationType(rt string) bool {
	for _, valid := range ValidRelationTypes() {
		if rt == valid {
			return true
		}
	}
	return false
}
