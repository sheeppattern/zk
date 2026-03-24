package cmd

import (
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/spf13/cobra"
	"github.com/sheeppattern/zk/internal/model"
)

var quickmemoCmd = &cobra.Command{
	Use:   "quickmemo <text>",
	Short: "Create a memo with minimal input",
	Long:  "Shortcut for memo creation: title is auto-derived from the text (truncated at 50 chars), layer defaults to concrete, no tags required.",
	Example: `  zk quickmemo "Redis cache hit rate is 95%"
  zk quickmemo "This pattern keeps recurring" --note 1
  zk quickmemo "Quick observation" --author claude`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		text := args[0]
		authorFlag, _ := cmd.Flags().GetString("author")

		// Derive title: truncate at 50 runes on word boundary.
		title := text
		if utf8.RuneCountInString(title) > 50 {
			title = string([]rune(title)[:50])
			if idx := strings.LastIndex(title, " "); idx > 20 {
				title = title[:idx]
			}
		}

		s, err := openStore(cmd)
		if err != nil {
			return err
		}
		defer s.Close()

		// Resolve author: --author flag > config default_author > "user".
		author := authorFlag
		if author == "" {
			if cfg, err := s.LoadConfig(); err == nil && cfg.DefaultAuthor != "" {
				author = cfg.DefaultAuthor
			} else {
				author = "user"
			}
		}

		memo := &model.Memo{
			Title:   title,
			Content: text,
			Tags:    []string{},
			Layer:   model.LayerConcrete,
			NoteID:  flagNote,
			Metadata: model.Metadata{
				Status: model.StatusActive,
				Author: author,
			},
		}

		if err := s.CreateMemo(memo); err != nil {
			return fmt.Errorf("create memo: %w", err)
		}

		return getFormatter().PrintMemo(memo)
	},
}

func init() {
	quickmemoCmd.Flags().String("author", "", "memo author (e.g., claude, gemini, human)")
	rootCmd.AddCommand(quickmemoCmd)
}
