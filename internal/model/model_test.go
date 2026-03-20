package model

import (
	"regexp"
	"strings"
	"testing"
	"time"
)

func TestGenerateID(t *testing.T) {
	re := regexp.MustCompile(`^N-[0-9A-F]{6}$`)
	seen := make(map[string]bool)

	for i := 0; i < 100; i++ {
		id := GenerateID("N")
		if !re.MatchString(id) {
			t.Fatalf("GenerateID(%q) = %q; want format N-XXXXXX (6 uppercase hex chars)", "N", id)
		}
		if seen[id] {
			t.Fatalf("duplicate ID generated: %s", id)
		}
		seen[id] = true
	}
}

func TestNewNote(t *testing.T) {
	before := time.Now()
	n := NewNote("Test Title", "Test Content", nil)
	after := time.Now()

	if n.ID == "" {
		t.Fatal("NewNote ID is empty")
	}
	if !strings.HasPrefix(n.ID, "N-") {
		t.Fatalf("NewNote ID = %q; want prefix N-", n.ID)
	}
	if n.Title != "Test Title" {
		t.Fatalf("Title = %q; want %q", n.Title, "Test Title")
	}
	if n.Content != "Test Content" {
		t.Fatalf("Content = %q; want %q", n.Content, "Test Content")
	}
	if n.Metadata.Status != StatusActive {
		t.Fatalf("Status = %q; want %q", n.Metadata.Status, StatusActive)
	}
	if n.Metadata.CreatedAt.Before(before) || n.Metadata.CreatedAt.After(after) {
		t.Fatalf("CreatedAt %v not between %v and %v", n.Metadata.CreatedAt, before, after)
	}
	if n.Metadata.UpdatedAt.Before(before) || n.Metadata.UpdatedAt.After(after) {
		t.Fatalf("UpdatedAt %v not between %v and %v", n.Metadata.UpdatedAt, before, after)
	}
	if n.Tags == nil {
		t.Fatal("Tags is nil; want empty slice")
	}
	if len(n.Tags) != 0 {
		t.Fatalf("Tags length = %d; want 0", len(n.Tags))
	}
	if n.Links == nil {
		t.Fatal("Links is nil; want empty slice")
	}
	if len(n.Links) != 0 {
		t.Fatalf("Links length = %d; want 0", len(n.Links))
	}
}

func TestNewProject(t *testing.T) {
	before := time.Now()
	p := NewProject("My Project", "A description")
	after := time.Now()

	if !strings.HasPrefix(p.ID, "P-") {
		t.Fatalf("Project ID = %q; want prefix P-", p.ID)
	}
	if p.Name != "My Project" {
		t.Fatalf("Name = %q; want %q", p.Name, "My Project")
	}
	if p.Description != "A description" {
		t.Fatalf("Description = %q; want %q", p.Description, "A description")
	}
	if p.CreatedAt.Before(before) || p.CreatedAt.After(after) {
		t.Fatalf("CreatedAt %v not in expected range", p.CreatedAt)
	}
	if p.UpdatedAt.Before(before) || p.UpdatedAt.After(after) {
		t.Fatalf("UpdatedAt %v not in expected range", p.UpdatedAt)
	}
}

func TestRelationTypes(t *testing.T) {
	// Reset custom types for a clean test.
	customRelationTypes = nil

	types := ValidRelationTypes()
	if len(types) != 10 {
		t.Fatalf("ValidRelationTypes() returned %d types; want 10", len(types))
	}

	expected := []string{
		RelRelated, RelSupports, RelContradicts,
		RelExtends, RelCauses, RelExampleOf,
		RelAbstracts, RelGrounds,
		RelReplaces, RelInvalidates,
	}
	for _, e := range expected {
		if !IsValidRelationType(e) {
			t.Fatalf("IsValidRelationType(%q) = false; want true", e)
		}
	}
}

func TestIsValidRelationType_Invalid(t *testing.T) {
	if IsValidRelationType("invalid-type") {
		t.Fatal("IsValidRelationType(\"invalid-type\") = true; want false")
	}
}

func TestCustomRelationTypes(t *testing.T) {
	// Reset custom types.
	customRelationTypes = nil

	RegisterRelationType("custom-type")
	if !IsValidRelationType("custom-type") {
		t.Fatal("IsValidRelationType(\"custom-type\") = false after registration; want true")
	}

	types := ValidRelationTypes()
	found := false
	for _, rt := range types {
		if rt == "custom-type" {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("ValidRelationTypes() does not include \"custom-type\" after registration")
	}

	// Duplicate registration should not add twice.
	RegisterRelationType("custom-type")
	count := 0
	for _, rt := range ValidRelationTypes() {
		if rt == "custom-type" {
			count++
		}
	}
	if count != 1 {
		t.Fatalf("\"custom-type\" appears %d times after duplicate registration; want 1", count)
	}

	// Cleanup.
	customRelationTypes = nil
}

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()
	if !strings.Contains(cfg.StorePath, ".zk-memory") {
		t.Fatalf("StorePath = %q; want to contain .zk-memory", cfg.StorePath)
	}
	if cfg.DefaultFormat != "md" {
		t.Fatalf("DefaultFormat = %q; want %q", cfg.DefaultFormat, "md")
	}
}

func TestNewNoteDefaultLayer(t *testing.T) {
	n := NewNote("Layer Test", "content", nil)
	if n.Layer != "concrete" {
		t.Fatalf("NewNote Layer = %q; want %q", n.Layer, "concrete")
	}
}

func TestLayerConstants(t *testing.T) {
	if LayerConcrete != "concrete" {
		t.Fatalf("LayerConcrete = %q; want %q", LayerConcrete, "concrete")
	}
	if LayerAbstract != "abstract" {
		t.Fatalf("LayerAbstract = %q; want %q", LayerAbstract, "abstract")
	}
}

func TestNewRelationTypes(t *testing.T) {
	// Reset custom types for a clean test.
	customRelationTypes = nil

	types := ValidRelationTypes()
	// "abstracts" and "grounds" should be in the built-in list.
	foundAbstracts := false
	foundGrounds := false
	for _, rt := range types {
		if rt == RelAbstracts {
			foundAbstracts = true
		}
		if rt == RelGrounds {
			foundGrounds = true
		}
	}
	if !foundAbstracts {
		t.Fatalf("ValidRelationTypes() does not include %q", RelAbstracts)
	}
	if !foundGrounds {
		t.Fatalf("ValidRelationTypes() does not include %q", RelGrounds)
	}

	if !IsValidRelationType(RelAbstracts) {
		t.Fatalf("IsValidRelationType(%q) = false; want true", RelAbstracts)
	}
	if !IsValidRelationType(RelGrounds) {
		t.Fatalf("IsValidRelationType(%q) = false; want true", RelGrounds)
	}
}

func TestNewRelationTypesExtended(t *testing.T) {
	// Reset custom types for a clean test.
	customRelationTypes = nil

	types := ValidRelationTypes()

	// "replaces" and "invalidates" should be in the built-in list.
	foundReplaces := false
	foundInvalidates := false
	for _, rt := range types {
		if rt == RelReplaces {
			foundReplaces = true
		}
		if rt == RelInvalidates {
			foundInvalidates = true
		}
	}
	if !foundReplaces {
		t.Fatalf("ValidRelationTypes() does not include %q", RelReplaces)
	}
	if !foundInvalidates {
		t.Fatalf("ValidRelationTypes() does not include %q", RelInvalidates)
	}

	if !IsValidRelationType(RelReplaces) {
		t.Fatalf("IsValidRelationType(%q) = false; want true", RelReplaces)
	}
	if !IsValidRelationType(RelInvalidates) {
		t.Fatalf("IsValidRelationType(%q) = false; want true", RelInvalidates)
	}
}
