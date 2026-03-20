package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/sheeppattern/zk/internal/model"
	"github.com/sheeppattern/zk/internal/store"
)

var quicknoteCmd = &cobra.Command{
	Use:   "quicknote <text>",
	Short: "Create a note with minimal input",
	Long:  "Shortcut for note creation: title is auto-derived from the text (truncated at 50 chars), layer defaults to concrete, no tags required.",
	Example: `  zk quicknote "Redis cache hit rate is 95%"
  zk quicknote "This pattern keeps recurring" --project P-XXXXXX
  zk quicknote "Quick observation" --author claude`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		text := args[0]
		authorFlag, _ := cmd.Flags().GetString("author")

		// Derive title: truncate at 50 chars on word boundary.
		title := text
		if len(title) > 50 {
			title = title[:50]
			// Try to cut at last space for cleaner title.
			if idx := lastSpaceIndex(title); idx > 20 {
				title = title[:idx]
			}
		}

		storePath := getStorePath(cmd)
		s := store.NewStore(storePath)

		// Resolve author: --author flag > config default_author > "user".
		author := authorFlag
		if author == "" {
			if cfg, err := s.LoadConfig(); err == nil && cfg.DefaultAuthor != "" {
				author = cfg.DefaultAuthor
			} else {
				author = "user"
			}
		}

		note := model.NewNote(title, text, []string{})
		note.Metadata.Author = author

		if flagProject != "" {
			note.ProjectID = flagProject
		}

		if err := s.CreateNote(note); err != nil {
			return fmt.Errorf("create note: %w", err)
		}

		return getFormatter().PrintNote(note)
	},
}

// lastSpaceIndex returns the index of the last space in s, or -1 if none.
func lastSpaceIndex(s string) int {
	for i := len(s) - 1; i >= 0; i-- {
		if s[i] == ' ' {
			return i
		}
	}
	return -1
}

func init() {
	quicknoteCmd.Flags().String("author", "", "note author (e.g., claude, gemini, human)")
	rootCmd.AddCommand(quicknoteCmd)
}
