package model

import (
	"os"
	"path/filepath"
	"sync"
	"time"
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
	RelReplaces    = "replaces"
	RelInvalidates = "invalidates"
)

// Layer constants.
const (
	LayerConcrete = "concrete"
	LayerAbstract = "abstract"
)

// Memo status constants.
const (
	StatusActive   = "active"
	StatusArchived = "archived"
)

// Memo represents an atomic memory record in the Zettelkasten.
type Memo struct {
	ID       int64    `json:"id"`
	Title    string   `json:"title"`
	Content  string   `json:"content"`
	Tags     []string `json:"tags"`
	Layer    string   `json:"layer"`
	NoteID   int64    `json:"note_id"`
	Metadata Metadata `json:"metadata"`
}

// Note groups related memos together (formerly Project).
type Note struct {
	ID          int64     `json:"id"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// Link represents a weighted, typed connection between memos.
type Link struct {
	SourceID     int64   `json:"source_id"`
	TargetID     int64   `json:"target_id"`
	RelationType string  `json:"relation_type"`
	Weight       float64 `json:"weight"`
}

// Metadata holds auto-recorded metadata for a memo.
type Metadata struct {
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
	Source    string    `json:"source"`
	Status    string    `json:"status"`
	Summary   string    `json:"summary,omitempty"`
	Author    string    `json:"author,omitempty"`
}

// Config holds CLI configuration.
type Config struct {
	StorePath           string   `json:"store_path"`
	DefaultNote         string   `json:"default_note"`
	DefaultFormat       string   `json:"default_format"`
	DefaultAuthor       string   `json:"default_author,omitempty"`
	CustomRelationTypes []string `json:"custom_relation_types,omitempty"`
}

// DefaultConfig returns a Config with sensible defaults.
// StorePath defaults to ~/.nete.
func DefaultConfig() *Config {
	home, err := os.UserHomeDir()
	if err != nil {
		home = "."
	}
	return &Config{
		StorePath:     filepath.Join(home, ".nete"),
		DefaultNote:   "",
		DefaultFormat: "md",
	}
}

// customRelationTypes holds user-registered relation types beyond the built-in set.
var (
	customRelationTypes   []string
	customRelationTypesMu sync.RWMutex
)

// RegisterRelationType adds a custom relation type if it doesn't already exist.
func RegisterRelationType(rt string) {
	customRelationTypesMu.Lock()
	defer customRelationTypesMu.Unlock()
	for _, existing := range customRelationTypes {
		if existing == rt {
			return
		}
	}
	customRelationTypes = append(customRelationTypes, rt)
}

// CustomRelationTypes returns only the user-registered custom relation types.
func CustomRelationTypes() []string {
	customRelationTypesMu.RLock()
	defer customRelationTypesMu.RUnlock()
	result := make([]string, len(customRelationTypes))
	copy(result, customRelationTypes)
	return result
}

// LoadCustomRelationTypes replaces the custom relation types from a config slice.
func LoadCustomRelationTypes(types []string) {
	customRelationTypesMu.Lock()
	defer customRelationTypesMu.Unlock()
	customRelationTypes = make([]string, len(types))
	copy(customRelationTypes, types)
}

// ResetCustomRelationTypes clears all custom relation types (for testing).
func ResetCustomRelationTypes() {
	customRelationTypesMu.Lock()
	defer customRelationTypesMu.Unlock()
	customRelationTypes = nil
}

// ValidRelationTypes returns all valid relation type constants (built-in + custom).
func ValidRelationTypes() []string {
	customRelationTypesMu.RLock()
	defer customRelationTypesMu.RUnlock()
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
	result := make([]string, 0, len(builtIn)+len(customRelationTypes))
	result = append(result, builtIn...)
	result = append(result, customRelationTypes...)
	return result
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
