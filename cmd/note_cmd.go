package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/sheeppattern/zk/internal/model"
	"github.com/sheeppattern/zk/internal/store"
)

var noteCmd = &cobra.Command{
	Use:   "note",
	Short: "Manage notes",
	Long:  "Create, read, update, delete, and list Zettelkasten notes.",
}

var noteCreateCmd = &cobra.Command{
	Use:   "create",
	Short: "Create a new note",
	RunE: func(cmd *cobra.Command, args []string) error {
		title, _ := cmd.Flags().GetString("title")
		content, _ := cmd.Flags().GetString("content")
		tags, _ := cmd.Flags().GetStringSlice("tags")

		note := model.NewNote(title, content, tags)

		if flagProject != "" {
			note.ProjectID = flagProject
		}

		s := store.NewStore(getStorePath(cmd))
		if err := s.CreateNote(note); err != nil {
			return fmt.Errorf("create note: %w", err)
		}

		return getFormatter().PrintNote(note)
	},
}

var noteGetCmd = &cobra.Command{
	Use:   "get <id>",
	Short: "Get a note by ID",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		noteID := args[0]

		s := store.NewStore(getStorePath(cmd))
		note, err := s.GetNote(flagProject, noteID)
		if err != nil {
			return fmt.Errorf("get note: %w", err)
		}

		return getFormatter().PrintNote(note)
	},
}

var noteListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all notes",
	RunE: func(cmd *cobra.Command, args []string) error {
		s := store.NewStore(getStorePath(cmd))
		notes, err := s.ListNotes(flagProject)
		if err != nil {
			return fmt.Errorf("list notes: %w", err)
		}

		return getFormatter().PrintNotes(notes)
	},
}

var noteUpdateCmd = &cobra.Command{
	Use:   "update <id>",
	Short: "Update an existing note",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		noteID := args[0]

		s := store.NewStore(getStorePath(cmd))
		note, err := s.GetNote(flagProject, noteID)
		if err != nil {
			return fmt.Errorf("get note for update: %w", err)
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

		if err := s.UpdateNote(note); err != nil {
			return fmt.Errorf("update note: %w", err)
		}

		return getFormatter().PrintNote(note)
	},
}

var noteDeleteCmd = &cobra.Command{
	Use:   "delete <id>",
	Short: "Delete a note by ID",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		noteID := args[0]

		s := store.NewStore(getStorePath(cmd))
		if err := s.DeleteNote(flagProject, noteID); err != nil {
			return fmt.Errorf("delete note: %w", err)
		}

		fmt.Fprintln(os.Stderr, "Deleted note "+noteID)
		return nil
	},
}

func init() {
	// noteCreateCmd flags
	noteCreateCmd.Flags().String("title", "", "note title (required)")
	noteCreateCmd.Flags().String("content", "", "note content (required)")
	noteCreateCmd.Flags().StringSlice("tags", nil, "comma-separated tags")
	_ = noteCreateCmd.MarkFlagRequired("title")
	_ = noteCreateCmd.MarkFlagRequired("content")

	// noteUpdateCmd flags
	noteUpdateCmd.Flags().String("title", "", "new title")
	noteUpdateCmd.Flags().String("content", "", "new content")
	noteUpdateCmd.Flags().StringSlice("tags", nil, "new tags")
	noteUpdateCmd.Flags().String("status", "", "new status (active, archived)")

	// Register subcommands
	noteCmd.AddCommand(noteCreateCmd)
	noteCmd.AddCommand(noteGetCmd)
	noteCmd.AddCommand(noteListCmd)
	noteCmd.AddCommand(noteUpdateCmd)
	noteCmd.AddCommand(noteDeleteCmd)

	rootCmd.AddCommand(noteCmd)
}
