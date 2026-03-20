package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/sheeppattern/zk/internal/model"
	"github.com/sheeppattern/zk/internal/store"
	"gopkg.in/yaml.v3"
)

// NoteExport is a serializable representation of a Note that includes Content,
// since model.Note tags Content as json:"-" yaml:"-".
type NoteExport struct {
	ID        string         `json:"id"                    yaml:"id"`
	Title     string         `json:"title"                 yaml:"title"`
	Content   string         `json:"content"               yaml:"content"`
	Tags      []string       `json:"tags"                  yaml:"tags"`
	Links     []model.Link   `json:"links"                 yaml:"links"`
	Metadata  model.Metadata `json:"metadata"              yaml:"metadata"`
	ProjectID string         `json:"project_id,omitempty"  yaml:"project_id,omitempty"`
}

// ExportData is the top-level structure for exported data.
type ExportData struct {
	Version    string        `json:"version"     yaml:"version"`
	ExportedAt string        `json:"exported_at" yaml:"exported_at"`
	ProjectID  string        `json:"project_id"  yaml:"project_id"`
	Notes      []*NoteExport `json:"notes"       yaml:"notes"`
}

func noteToExport(n *model.Note) *NoteExport {
	return &NoteExport{
		ID:        n.ID,
		Title:     n.Title,
		Content:   n.Content,
		Tags:      n.Tags,
		Links:     n.Links,
		Metadata:  n.Metadata,
		ProjectID: n.ProjectID,
	}
}

func exportToNote(e *NoteExport) *model.Note {
	return &model.Note{
		ID:        e.ID,
		Title:     e.Title,
		Content:   e.Content,
		Tags:      e.Tags,
		Links:     e.Links,
		Metadata:  e.Metadata,
		ProjectID: e.ProjectID,
	}
}

var exportCmd = &cobra.Command{
	Use:   "export",
	Short: "Export notes to a file",
	Long:  "Export notes as JSON or YAML. Exports all notes by default, or specific notes via --notes flag.",
	RunE: func(cmd *cobra.Command, args []string) error {
		outputPath, _ := cmd.Flags().GetString("output")
		noteIDs, _ := cmd.Flags().GetStringSlice("notes")

		s := store.NewStore(getStorePath(cmd))

		var notes []*model.Note

		if len(noteIDs) > 0 {
			for _, id := range noteIDs {
				n, err := s.GetNote(flagProject, id)
				if err != nil {
					return fmt.Errorf("get note %s: %w", id, err)
				}
				notes = append(notes, n)
			}
		} else {
			var err error
			notes, err = s.ListNotes(flagProject)
			if err != nil {
				return fmt.Errorf("list notes: %w", err)
			}
		}

		exportNotes := make([]*NoteExport, len(notes))
		for i, n := range notes {
			exportNotes[i] = noteToExport(n)
		}

		data := ExportData{
			Version:    "1.0",
			ExportedAt: time.Now().UTC().Format(time.RFC3339),
			ProjectID:  flagProject,
			Notes:      exportNotes,
		}

		var out []byte
		var err error

		format := flagFormat
		// If output path is specified, infer format from extension if --format wasn't explicitly set.
		if outputPath != "" && !cmd.Flags().Changed("format") {
			ext := strings.ToLower(filepath.Ext(outputPath))
			switch ext {
			case ".yaml", ".yml":
				format = "yaml"
			case ".json":
				format = "json"
			}
		}

		switch format {
		case "json":
			out, err = json.MarshalIndent(data, "", "  ")
		case "yaml":
			out, err = yaml.Marshal(data)
		default:
			// Default to JSON for unsupported formats in export context.
			out, err = json.MarshalIndent(data, "", "  ")
		}
		if err != nil {
			return fmt.Errorf("marshal export data: %w", err)
		}

		if outputPath != "" {
			if err := os.WriteFile(outputPath, out, 0644); err != nil {
				return fmt.Errorf("write output file: %w", err)
			}
			statusf("exported %d notes to %s", len(notes), outputPath)
		} else {
			fmt.Fprintln(os.Stdout, string(out))
		}

		return nil
	},
}

var importCmd = &cobra.Command{
	Use:   "import",
	Short: "Import notes from a file",
	Long:  "Import notes from a JSON or YAML export file. Use --conflict to control how duplicate IDs are handled.",
	RunE: func(cmd *cobra.Command, args []string) error {
		filePath, _ := cmd.Flags().GetString("file")
		conflict, _ := cmd.Flags().GetString("conflict")

		if filePath == "" {
			return fmt.Errorf("--file flag is required")
		}

		switch conflict {
		case "skip", "overwrite", "new-id":
			// valid
		default:
			return fmt.Errorf("invalid --conflict value %q: must be skip, overwrite, or new-id", conflict)
		}

		raw, err := os.ReadFile(filePath)
		if err != nil {
			return fmt.Errorf("read import file: %w", err)
		}

		var data ExportData

		// Try to detect format by extension, fall back to trying both.
		ext := strings.ToLower(filepath.Ext(filePath))
		switch ext {
		case ".yaml", ".yml":
			err = yaml.Unmarshal(raw, &data)
		case ".json":
			err = json.Unmarshal(raw, &data)
		default:
			// Try JSON first, then YAML.
			err = json.Unmarshal(raw, &data)
			if err != nil {
				err = yaml.Unmarshal(raw, &data)
			}
		}
		if err != nil {
			return fmt.Errorf("parse import file: %w", err)
		}

		s := store.NewStore(getStorePath(cmd))

		var imported, skipped, overwritten int

		for _, ne := range data.Notes {
			note := exportToNote(ne)

			// Override project if --project flag is set.
			if flagProject != "" {
				note.ProjectID = flagProject
			}

			projectID := note.ProjectID

			// Check if note already exists.
			existing, _ := s.GetNote(projectID, note.ID)

			if existing != nil {
				switch conflict {
				case "skip":
					skipped++
					continue
				case "overwrite":
					if err := s.UpdateNote(note); err != nil {
						return fmt.Errorf("overwrite note %s: %w", note.ID, err)
					}
					overwritten++
					continue
				case "new-id":
					note.ID = model.GenerateID("N")
				}
			}

			if err := s.CreateNote(note); err != nil {
				return fmt.Errorf("create note %s: %w", note.ID, err)
			}
			imported++
		}

		statusf("imported %d notes, skipped %d, overwritten %d", imported, skipped, overwritten)
		return nil
	},
}

func init() {
	// exportCmd flags
	exportCmd.Flags().String("output", "", "file path to write export data (default: stdout)")
	exportCmd.Flags().StringSlice("notes", nil, "specific note IDs to export (default: all)")

	// importCmd flags
	importCmd.Flags().String("file", "", "file path to read import data from (required)")
	importCmd.Flags().String("conflict", "skip", "conflict resolution: skip, overwrite, new-id")
	_ = importCmd.MarkFlagRequired("file")

	rootCmd.AddCommand(exportCmd)
	rootCmd.AddCommand(importCmd)
}
