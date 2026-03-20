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
	RelAbstracts   = "abstracts"
	RelGrounds     = "grounds"
	RelReplaces    = "replaces"     // new note supersedes old one
	RelInvalidates = "invalidates"  // data disproves a hypothesis
)

// Layer constants.
const (
	LayerConcrete = "concrete"
	LayerAbstract = "abstract"
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
	Layer     string   `yaml:"layer,omitempty"      json:"layer,omitempty"`
}

// NoteFrontmatter is the YAML-serializable portion of a Note (without Content).
type NoteFrontmatter struct {
	ID        string   `yaml:"id"        json:"id"`
	Title     string   `yaml:"title"     json:"title"`
	Tags      []string `yaml:"tags"      json:"tags"`
	Links     []Link   `yaml:"links"     json:"links"`
	Metadata  Metadata `yaml:"metadata"  json:"metadata"`
	ProjectID string   `yaml:"project_id,omitempty" json:"project_id,omitempty"`
	Layer     string   `yaml:"layer,omitempty"      json:"layer,omitempty"`
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
	Summary  string    `yaml:"summary,omitempty" json:"summary,omitempty"`
	Author   string    `yaml:"author,omitempty"  json:"author,omitempty"`
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
	StorePath           string   `yaml:"store_path"      json:"store_path"`
	DefaultProject      string   `yaml:"default_project" json:"default_project"`
	DefaultFormat       string   `yaml:"default_format"  json:"default_format"`
	DefaultAuthor       string   `yaml:"default_author,omitempty" json:"default_author,omitempty"`
	CustomRelationTypes []string `yaml:"custom_relation_types,omitempty" json:"custom_relation_types,omitempty"`
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
		Layer:   LayerConcrete,
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

// customRelationTypes holds user-registered relation types beyond the built-in set.
var customRelationTypes []string

// RegisterRelationType adds a custom relation type if it doesn't already exist.
func RegisterRelationType(rt string) {
	for _, existing := range customRelationTypes {
		if existing == rt {
			return
		}
	}
	customRelationTypes = append(customRelationTypes, rt)
}

// CustomRelationTypes returns only the user-registered custom relation types.
func CustomRelationTypes() []string {
	result := make([]string, len(customRelationTypes))
	copy(result, customRelationTypes)
	return result
}

// LoadCustomRelationTypes replaces the custom relation types from a config slice.
func LoadCustomRelationTypes(types []string) {
	customRelationTypes = make([]string, len(types))
	copy(customRelationTypes, types)
}

// ValidRelationTypes returns all valid relation type constants (built-in + custom).
func ValidRelationTypes() []string {
	builtIn := []string{
		RelRelated,
		RelSupports,
		RelContradicts,
		RelExtends,
		RelCauses,
		RelExampleOf,
		RelAbstracts,
		RelGrounds,
		RelReplaces,
		RelInvalidates,
	}
	return append(builtIn, customRelationTypes...)
}

// IsValidRelationType checks whether the given relation type is valid (built-in or custom).
func IsValidRelationType(rt string) bool {
	for _, valid := range ValidRelationTypes() {
		if rt == valid {
			return true
		}
	}
	return false
}
