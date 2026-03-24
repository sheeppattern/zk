package model

import (
	"fmt"
	"strings"
	"testing"
)

func TestMemoStruct(t *testing.T) {
	m := Memo{
		ID:      1,
		Title:   "Test Title",
		Content: "Test Content",
		Tags:    []string{"tag1", "tag2"},
		Layer:   LayerConcrete,
		NoteID:  0,
	}
	if m.ID != 1 {
		t.Fatalf("ID = %d; want 1", m.ID)
	}
	if m.Title != "Test Title" {
		t.Fatalf("Title = %q; want %q", m.Title, "Test Title")
	}
	if m.Layer != "concrete" {
		t.Fatalf("Layer = %q; want %q", m.Layer, "concrete")
	}
	if len(m.Tags) != 2 {
		t.Fatalf("Tags length = %d; want 2", len(m.Tags))
	}
}

func TestNoteStruct(t *testing.T) {
	n := Note{
		ID:          1,
		Name:        "My Note",
		Description: "A description",
	}
	if n.ID != 1 {
		t.Fatalf("ID = %d; want 1", n.ID)
	}
	if n.Name != "My Note" {
		t.Fatalf("Name = %q; want %q", n.Name, "My Note")
	}
}

func TestLinkStruct(t *testing.T) {
	l := Link{
		SourceID:     1,
		TargetID:     2,
		RelationType: RelSupports,
		Weight:       0.8,
	}
	if l.SourceID != 1 {
		t.Fatalf("SourceID = %d; want 1", l.SourceID)
	}
	if l.TargetID != 2 {
		t.Fatalf("TargetID = %d; want 2", l.TargetID)
	}
	if l.Weight != 0.8 {
		t.Fatalf("Weight = %f; want 0.8", l.Weight)
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
	if !strings.Contains(cfg.StorePath, ".nete") {
		t.Fatalf("StorePath = %q; want to contain .nete", cfg.StorePath)
	}
	if cfg.DefaultFormat != "md" {
		t.Fatalf("DefaultFormat = %q; want %q", cfg.DefaultFormat, "md")
	}
	if cfg.DefaultNote != "" {
		t.Fatalf("DefaultNote = %q; want empty", cfg.DefaultNote)
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
	customRelationTypes = nil

	types := ValidRelationTypes()
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
}

func TestNewRelationTypesExtended(t *testing.T) {
	customRelationTypes = nil

	if !IsValidRelationType(RelReplaces) {
		t.Fatalf("IsValidRelationType(%q) = false; want true", RelReplaces)
	}
	if !IsValidRelationType(RelInvalidates) {
		t.Fatalf("IsValidRelationType(%q) = false; want true", RelInvalidates)
	}
}

func TestCustomRelationTypesConcurrency(t *testing.T) {
	ResetCustomRelationTypes()
	defer ResetCustomRelationTypes()

	done := make(chan bool)
	go func() {
		for i := 0; i < 100; i++ {
			RegisterRelationType(fmt.Sprintf("custom-%d", i))
		}
		done <- true
	}()
	go func() {
		for i := 0; i < 100; i++ {
			_ = ValidRelationTypes()
			_ = IsValidRelationType("supports")
		}
		done <- true
	}()
	<-done
	<-done
}

func TestValidRelationTypesNoBacking(t *testing.T) {
	ResetCustomRelationTypes()
	defer ResetCustomRelationTypes()

	RegisterRelationType("test-type")

	vrt := ValidRelationTypes()
	originalLen := len(vrt)

	vrt = append(vrt, "injected")

	vrt2 := ValidRelationTypes()
	if len(vrt2) != originalLen {
		t.Fatalf("ValidRelationTypes() length changed after caller append: got %d, want %d", len(vrt2), originalLen)
	}
}

func TestStatusConstants(t *testing.T) {
	if StatusActive != "active" {
		t.Fatalf("StatusActive = %q; want %q", StatusActive, "active")
	}
	if StatusArchived != "archived" {
		t.Fatalf("StatusArchived = %q; want %q", StatusArchived, "archived")
	}
}
