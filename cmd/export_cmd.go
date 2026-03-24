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
	"gopkg.in/yaml.v3"
)

// MemoExport is a serializable representation of a Memo for export.
type MemoExport struct {
	ID       int64          `json:"id"       yaml:"id"`
	Title    string         `json:"title"    yaml:"title"`
	Content  string         `json:"content"  yaml:"content"`
	Layer    string         `json:"layer"    yaml:"layer"`
	Tags     []string       `json:"tags"     yaml:"tags"`
	NoteID   int64          `json:"note_id"  yaml:"note_id"`
	Metadata model.Metadata `json:"metadata" yaml:"metadata"`
}

// ExportData is the top-level structure for exported data.
type ExportData struct {
	Version    string        `json:"version"     yaml:"version"`
	ExportedAt string        `json:"exported_at" yaml:"exported_at"`
	NoteID     int64         `json:"note_id"     yaml:"note_id"`
	Memos      []*MemoExport `json:"memos"       yaml:"memos"`
}

func memoToExport(m *model.Memo) *MemoExport {
	return &MemoExport{
		ID:       m.ID,
		Title:    m.Title,
		Content:  m.Content,
		Layer:    m.Layer,
		Tags:     m.Tags,
		NoteID:   m.NoteID,
		Metadata: m.Metadata,
	}
}

var exportCmd = &cobra.Command{
	Use:   "export",
	Short: "Export memos to a file",
	Long:  "Export memos as JSON or YAML. Exports all memos by default, or filter by --note.",
	Example: `  zk export --note 1 --format yaml --output backup.yaml
  zk export`,
	RunE: func(cmd *cobra.Command, args []string) error {
		outputPath, _ := cmd.Flags().GetString("output")

		s, err := openStore(cmd)
		if err != nil {
			return err
		}
		defer s.Close()

		var memos []*model.Memo
		if flagNote != 0 {
			memos, err = s.ListMemos(flagNote)
		} else {
			memos, err = s.ListAllMemos()
		}
		if err != nil {
			return fmt.Errorf("list memos: %w", err)
		}

		exportMemos := make([]*MemoExport, len(memos))
		for i, m := range memos {
			exportMemos[i] = memoToExport(m)
		}

		data := ExportData{
			Version:    "2.0",
			ExportedAt: time.Now().UTC().Format(time.RFC3339),
			NoteID:     flagNote,
			Memos:      exportMemos,
		}

		var out []byte

		format := flagFormat
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
			out, err = json.MarshalIndent(data, "", "  ")
		}
		if err != nil {
			return fmt.Errorf("marshal export data: %w", err)
		}

		if outputPath != "" {
			if err := os.WriteFile(outputPath, out, 0644); err != nil {
				return fmt.Errorf("write output file: %w", err)
			}
			statusf("exported %d memos to %s", len(memos), outputPath)
		} else {
			fmt.Fprintln(os.Stdout, string(out))
		}

		return nil
	},
}

var importCmd = &cobra.Command{
	Use:   "import",
	Short: "Import memos from a file",
	Long:  "Import memos from a JSON or YAML export file.",
	Example: `  zk import --file backup.yaml --note 1
  zk import --file data.json`,
	RunE: func(cmd *cobra.Command, args []string) error {
		filePath, _ := cmd.Flags().GetString("file")

		if filePath == "" {
			return fmt.Errorf("--file flag is required")
		}

		raw, err := os.ReadFile(filePath)
		if err != nil {
			return fmt.Errorf("read import file: %w", err)
		}

		var data ExportData
		ext := strings.ToLower(filepath.Ext(filePath))
		switch ext {
		case ".yaml", ".yml":
			err = yaml.Unmarshal(raw, &data)
		case ".json":
			err = json.Unmarshal(raw, &data)
		default:
			err = json.Unmarshal(raw, &data)
			if err != nil {
				err = yaml.Unmarshal(raw, &data)
			}
		}
		if err != nil {
			return fmt.Errorf("parse import file: %w", err)
		}

		s, err := openStore(cmd)
		if err != nil {
			return err
		}
		defer s.Close()

		var imported int
		for _, me := range data.Memos {
			memo := &model.Memo{
				Title:    me.Title,
				Content:  me.Content,
				Layer:    me.Layer,
				Tags:     me.Tags,
				NoteID:   me.NoteID,
				Metadata: me.Metadata,
			}
			// Override note ID if --note flag is set.
			if flagNote != 0 {
				memo.NoteID = flagNote
			}
			if err := s.CreateMemo(memo); err != nil {
				return fmt.Errorf("create memo: %w", err)
			}
			imported++
		}

		statusf("imported %d memos", imported)
		return nil
	},
}

func init() {
	exportCmd.Flags().String("output", "", "file path to write export data (default: stdout)")

	importCmd.Flags().String("file", "", "file path to read import data from (required)")
	_ = importCmd.MarkFlagRequired("file")

	rootCmd.AddCommand(exportCmd)
	rootCmd.AddCommand(importCmd)
}
