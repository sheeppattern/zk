package cmd

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/spf13/cobra"
)

// NoteDetail wraps a Note with computed statistics for get output.
type NoteDetail struct {
	ID          int64  `json:"id" yaml:"id"`
	Name        string `json:"name" yaml:"name"`
	Description string `json:"description" yaml:"description"`
	CreatedAt   string `json:"created_at" yaml:"created_at"`
	UpdatedAt   string `json:"updated_at" yaml:"updated_at"`
	MemoCount   int    `json:"memo_count" yaml:"memo_count"`
	LinkCount   int    `json:"link_count" yaml:"link_count"`
}

var noteCmd = &cobra.Command{
	Use:   "note",
	Short: "Manage notes",
	Long:  "Create, list, get, and delete notes (groups of memos) in the Zettelkasten store.",
}

var noteCreateCmd = &cobra.Command{
	Use:   "create <name>",
	Short: "Create a new note",
	Example: `  zk note create "my-research" --description "Research note"`,
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		name := args[0]
		description, _ := cmd.Flags().GetString("description")

		s, err := openStore(cmd)
		if err != nil {
			return err
		}
		defer s.Close()

		n, err := s.CreateNote(name, description)
		if err != nil {
			return fmt.Errorf("create note: %w", err)
		}

		return getFormatter().PrintNote(n)
	},
}

var noteListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all notes",
	Example: `  zk note list
  zk note list --format md`,
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		s, err := openStore(cmd)
		if err != nil {
			return err
		}
		defer s.Close()

		notes, err := s.ListNotes()
		if err != nil {
			return fmt.Errorf("list notes: %w", err)
		}

		return getFormatter().PrintNotes(notes)
	},
}

var noteGetCmd = &cobra.Command{
	Use:   "get <id>",
	Short: "Get a note by ID",
	Example: `  zk note get 1
  zk note get 1 --format md`,
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		id, err := strconv.ParseInt(args[0], 10, 64)
		if err != nil {
			return fmt.Errorf("invalid note ID %q: %w", args[0], err)
		}

		s, err := openStore(cmd)
		if err != nil {
			return err
		}
		defer s.Close()

		n, err := s.GetNote(id)
		if err != nil {
			return fmt.Errorf("get note: %w", err)
		}

		// Gather memo and link statistics.
		memos, err := s.ListMemos(id)
		if err != nil {
			return fmt.Errorf("list memos for note: %w", err)
		}

		linkCount := 0
		for _, m := range memos {
			out, in, lErr := s.ListLinks(m.ID)
			if lErr == nil {
				linkCount += len(out) + len(in)
			}
		}

		detail := NoteDetail{
			ID:          n.ID,
			Name:        n.Name,
			Description: n.Description,
			CreatedAt:   n.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
			UpdatedAt:   n.UpdatedAt.Format("2006-01-02T15:04:05Z07:00"),
			MemoCount:   len(memos),
			LinkCount:   linkCount,
		}

		f := getFormatter()
		switch f.Format {
		case "json":
			return f.PrintJSON(detail)
		case "yaml":
			return f.PrintYAML(detail)
		case "md":
			var b strings.Builder
			fmt.Fprintf(&b, "# %s\n\n", n.Name)
			fmt.Fprintf(&b, "**ID**: %d\n", n.ID)
			fmt.Fprintf(&b, "**Description**: %s\n", n.Description)
			fmt.Fprintf(&b, "**Created**: %s\n", n.CreatedAt.Format("2006-01-02 15:04:05"))
			fmt.Fprintf(&b, "**Memos**: %d\n", len(memos))
			fmt.Fprintf(&b, "**Links**: %d\n", linkCount)
			fmt.Fprintln(&b)
			_, err := fmt.Fprint(os.Stdout, b.String())
			return err
		default:
			return fmt.Errorf("unsupported format: %s", f.Format)
		}
	},
}

var noteDeleteCmd = &cobra.Command{
	Use:   "delete <id>",
	Short: "Delete a note by ID",
	Example: `  zk note delete 1`,
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		id, err := strconv.ParseInt(args[0], 10, 64)
		if err != nil {
			return fmt.Errorf("invalid note ID %q: %w", args[0], err)
		}

		s, err := openStore(cmd)
		if err != nil {
			return err
		}
		defer s.Close()

		if err := s.DeleteNote(id); err != nil {
			return fmt.Errorf("delete note: %w", err)
		}

		statusf("deleted note %d", id)
		return nil
	},
}

func init() {
	noteCreateCmd.Flags().String("description", "", "note description")

	noteCmd.AddCommand(noteCreateCmd)
	noteCmd.AddCommand(noteListCmd)
	noteCmd.AddCommand(noteGetCmd)
	noteCmd.AddCommand(noteDeleteCmd)

	rootCmd.AddCommand(noteCmd)
}
