package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
	"github.com/sheeppattern/zk/internal/model"
	"github.com/sheeppattern/zk/internal/store"
	"gopkg.in/yaml.v3"
)

// NoteTemplate defines a reusable template for note creation.
type NoteTemplate struct {
	TitlePrefix     string   `yaml:"title_prefix"`
	DefaultTags     []string `yaml:"default_tags"`
	DefaultStatus   string   `yaml:"default_status"`
	ContentTemplate string   `yaml:"content_template"`
}

var noteCmd = &cobra.Command{
	Use:   "note",
	Short: "Manage notes",
	Long:  "Create, read, update, delete, and list Zettelkasten notes.",
}

var noteCreateCmd = &cobra.Command{
	Use:   "create",
	Short: "Create a new note",
	Example: `  zk note create --title "Discovery" --content "Found something" --tags "research,important"
  zk note create --title "Idea" --content "..." --project P-XXXXXX
  zk note create --title "Paper Notes" --template research --project P-XXXXXX
  zk note create --title "Insight" --content "..." --layer abstract`,
	RunE: func(cmd *cobra.Command, args []string) error {
		title, _ := cmd.Flags().GetString("title")
		content, _ := cmd.Flags().GetString("content")
		tags, _ := cmd.Flags().GetStringSlice("tags")
		templateName, _ := cmd.Flags().GetString("template")
		layerFlag, _ := cmd.Flags().GetString("layer")
		summary, _ := cmd.Flags().GetString("summary")

		// Validate layer flag.
		if layerFlag != model.LayerConcrete && layerFlag != model.LayerAbstract {
			return fmt.Errorf("invalid layer %q: must be %q or %q", layerFlag, model.LayerConcrete, model.LayerAbstract)
		}

		storePath := getStorePath(cmd)

		// Apply template if specified.
		if templateName != "" {
			tmplPath := filepath.Join(storePath, "templates", templateName+".yaml")
			tmplData, err := os.ReadFile(tmplPath)
			if err != nil {
				return fmt.Errorf("template %q not found at %s: %w", templateName, tmplPath, err)
			}
			var tmpl NoteTemplate
			if err := yaml.Unmarshal(tmplData, &tmpl); err != nil {
				return fmt.Errorf("parse template %q: %w", templateName, err)
			}

			// Prepend title_prefix to --title.
			if tmpl.TitlePrefix != "" {
				title = tmpl.TitlePrefix + title
			}

			// Merge default_tags with --tags (template tags first, user tags appended).
			if len(tmpl.DefaultTags) > 0 {
				merged := make([]string, 0, len(tmpl.DefaultTags)+len(tags))
				merged = append(merged, tmpl.DefaultTags...)
				for _, t := range tags {
					// Avoid duplicates from user tags.
					dup := false
					for _, dt := range tmpl.DefaultTags {
						if strings.EqualFold(t, dt) {
							dup = true
							break
						}
					}
					if !dup {
						merged = append(merged, t)
					}
				}
				tags = merged
			}

			// If --content is empty, use content_template as content.
			if content == "" && tmpl.ContentTemplate != "" {
				content = tmpl.ContentTemplate
			}

			// Set status from template if not overridden by a flag (noteCreateCmd has no --status flag,
			// so template status always applies when present).
			if tmpl.DefaultStatus != "" {
				// Will be applied after note creation below.
			}

			note := model.NewNote(title, content, tags)
			note.Layer = layerFlag

			if summary != "" {
				note.Metadata.Summary = summary
			}

			if tmpl.DefaultStatus != "" {
				note.Metadata.Status = tmpl.DefaultStatus
			}

			if flagProject != "" {
				note.ProjectID = flagProject
			}

			s := store.NewStore(storePath)
			if err := s.CreateNote(note); err != nil {
				return fmt.Errorf("create note: %w", err)
			}

			return getFormatter().PrintNote(note)
		}

		note := model.NewNote(title, content, tags)
		note.Layer = layerFlag

		if summary != "" {
			note.Metadata.Summary = summary
		}

		if flagProject != "" {
			note.ProjectID = flagProject
		}

		s := store.NewStore(storePath)
		if err := s.CreateNote(note); err != nil {
			return fmt.Errorf("create note: %w", err)
		}

		return getFormatter().PrintNote(note)
	},
}

var noteGetCmd = &cobra.Command{
	Use:   "get <id>",
	Short: "Get a note by ID",
	Example: `  zk note get N-XXXXXX --project P-XXXXXX
  zk note get N-XXXXXX --format md`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		noteID := args[0]

		s := store.NewStore(getStorePath(cmd))
		note, err := s.GetNote(flagProject, noteID)
		if err != nil {
			return fmt.Errorf("note %s not found in project %s (check note ID and --project flag)", noteID, flagProject)
		}

		return getFormatter().PrintNote(note)
	},
}

var noteListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all notes",
	Example: `  zk note list --project P-XXXXXX
  zk note list --format md
  zk note list --layer abstract`,
	RunE: func(cmd *cobra.Command, args []string) error {
		layerFilter, _ := cmd.Flags().GetString("layer")

		s := store.NewStore(getStorePath(cmd))
		notes, err := s.ListNotes(flagProject)
		if err != nil {
			return fmt.Errorf("list notes: %w", err)
		}

		if layerFilter != "" {
			filtered := make([]*model.Note, 0, len(notes))
			for _, n := range notes {
				if n.Layer == layerFilter {
					filtered = append(filtered, n)
				}
			}
			notes = filtered
		}

		return getFormatter().PrintNotes(notes)
	},
}

var noteUpdateCmd = &cobra.Command{
	Use:   "update <id>",
	Short: "Update an existing note",
	Example: `  zk note update N-XXXXXX --title "New Title" --project P-XXXXXX
  zk note update N-XXXXXX --tags "new-tag" --status archived`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		noteID := args[0]

		s := store.NewStore(getStorePath(cmd))
		note, err := s.GetNote(flagProject, noteID)
		if err != nil {
			return fmt.Errorf("note %s not found in project %s (check note ID and --project flag)", noteID, flagProject)
		}

		if cmd.Flags().Changed("title") {
			note.Title, _ = cmd.Flags().GetString("title")
		}
		if cmd.Flags().Changed("content") {
			note.Content, _ = cmd.Flags().GetString("content")
		}
		if cmd.Flags().Changed("tags") {
			note.Tags, _ = cmd.Flags().GetStringSlice("tags")
		}
		if cmd.Flags().Changed("status") {
			note.Metadata.Status, _ = cmd.Flags().GetString("status")
		}
		if cmd.Flags().Changed("summary") {
			note.Metadata.Summary, _ = cmd.Flags().GetString("summary")
		}

		if err := s.UpdateNote(note); err != nil {
			return fmt.Errorf("update note: %w", err)
		}

		return getFormatter().PrintNote(note)
	},
}

var noteDeleteCmd = &cobra.Command{
	Use:   "delete <id>",
	Short: "Delete a note by ID",
	Example: `  zk note delete N-XXXXXX --project P-XXXXXX
  zk note delete N-XXXXXX --force --project P-XXXXXX`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		noteID := args[0]
		force, _ := cmd.Flags().GetBool("force")

		s := store.NewStore(getStorePath(cmd))

		if !force {
			// Load the target note to confirm it exists.
			if _, err := s.GetNote(flagProject, noteID); err != nil {
				return fmt.Errorf("delete note: %w", err)
			}
			// Scan for backlinks pointing to this note.
			allNotes, _ := s.ListNotesPartial(flagProject)
			var backlinkCount int
			for _, n := range allNotes {
				for _, link := range n.Links {
					if link.TargetID == noteID {
						backlinkCount++
					}
				}
			}
			if backlinkCount > 0 {
				fmt.Fprintf(os.Stderr, "Warning: %d backlink(s) point to note %s\n", backlinkCount, noteID)
				return fmt.Errorf("note %s has backlinks; use --force to delete anyway", noteID)
			}
		}

		if err := s.DeleteNote(flagProject, noteID); err != nil {
			return fmt.Errorf("delete note: %w", err)
		}

		statusf("Deleted note %s", noteID)
		return nil
	},
}

var noteMoveCmd = &cobra.Command{
	Use:   "move <noteID> <targetProject>",
	Short: "Move a note to a different project",
	Example: `  zk note move N-XXXXXX P-TARGET --project P-SOURCE
  zk note move N-XXXXXX P-TARGET`,
	Args: cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		noteID := args[0]
		targetProject := args[1]

		s := store.NewStore(getStorePath(cmd))

		if err := s.MoveNote(noteID, flagProject, targetProject); err != nil {
			return fmt.Errorf("note %s not found in project %s (verify the note exists with: zk note list --project %s)",
				noteID, flagProject, flagProject)
		}

		statusf("moved note %s from project %s to %s", noteID, flagProject, targetProject)
		return nil
	},
}

func init() {
	// noteCreateCmd flags
	noteCreateCmd.Flags().String("title", "", "note title (required)")
	noteCreateCmd.Flags().String("content", "", "note content (required)")
	noteCreateCmd.Flags().StringSlice("tags", nil, "comma-separated tags")
	noteCreateCmd.Flags().String("template", "", "template name (loads from {store}/templates/{name}.yaml)")
	noteCreateCmd.Flags().String("layer", model.LayerConcrete, "note layer (concrete, abstract)")
	noteCreateCmd.Flags().String("summary", "", "brief summary for quick scanning")
	_ = noteCreateCmd.MarkFlagRequired("title")

	// noteUpdateCmd flags
	noteUpdateCmd.Flags().String("title", "", "new title")
	noteUpdateCmd.Flags().String("content", "", "new content")
	noteUpdateCmd.Flags().StringSlice("tags", nil, "new tags")
	noteUpdateCmd.Flags().String("status", "", "new status (active, archived)")
	noteUpdateCmd.Flags().String("summary", "", "brief summary for quick scanning")

	// noteListCmd flags
	noteListCmd.Flags().String("layer", "", "filter by layer (concrete, abstract)")

	// noteDeleteCmd flags
	noteDeleteCmd.Flags().Bool("force", false, "force deletion even if backlinks exist")

	// Register subcommands
	noteCmd.AddCommand(noteCreateCmd)
	noteCmd.AddCommand(noteGetCmd)
	noteCmd.AddCommand(noteListCmd)
	noteCmd.AddCommand(noteUpdateCmd)
	noteCmd.AddCommand(noteDeleteCmd)
	noteCmd.AddCommand(noteMoveCmd)

	rootCmd.AddCommand(noteCmd)
}
