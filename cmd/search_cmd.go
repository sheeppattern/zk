package cmd

import (
	"fmt"
	"time"

	"github.com/spf13/cobra"
	"github.com/sheeppattern/nete/internal/store"
)

var searchCmd = &cobra.Command{
	Use:   "search <query>",
	Short: "Search memos by query",
	Long:  "Search memos using full-text search. Supports filtering by tags, status, layer, author, and date range.",
	Example: `  nete search "Redis" --note 1
  nete search "auth" --tags "security" --status active
  nete search "data" --created-after 2026-01-01 --sort created`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		query := args[0]

		tags, _ := cmd.Flags().GetStringSlice("tags")
		status, _ := cmd.Flags().GetString("status")
		sortFlag, _ := cmd.Flags().GetString("sort")
		createdAfterStr, _ := cmd.Flags().GetString("created-after")
		createdBeforeStr, _ := cmd.Flags().GetString("created-before")
		layer, _ := cmd.Flags().GetString("layer")
		authorFilter, _ := cmd.Flags().GetString("author")

		var createdAfter, createdBefore time.Time
		if createdAfterStr != "" {
			var err error
			createdAfter, err = time.Parse("2006-01-02", createdAfterStr)
			if err != nil {
				return fmt.Errorf("invalid --created-after date %q: expected format YYYY-MM-DD", createdAfterStr)
			}
		}
		if createdBeforeStr != "" {
			var err error
			createdBefore, err = time.Parse("2006-01-02", createdBeforeStr)
			if err != nil {
				return fmt.Errorf("invalid --created-before date %q: expected format YYYY-MM-DD", createdBeforeStr)
			}
		}

		s, err := openStore(cmd)
		if err != nil {
			return err
		}
		defer s.Close()

		opts := store.SearchOptions{
			NoteID:        flagNote,
			Layer:         layer,
			Status:        status,
			Author:        authorFilter,
			Tags:          tags,
			CreatedAfter:  createdAfter,
			CreatedBefore: createdBefore,
			Sort:          sortFlag,
		}

		memos, err := s.SearchMemos(query, opts)
		if err != nil {
			return fmt.Errorf("search memos: %w", err)
		}

		return getFormatter().PrintMemos(memos)
	},
}

func init() {
	searchCmd.Flags().StringSlice("tags", nil, "filter by tags (AND logic)")
	searchCmd.Flags().String("status", "", "filter by status (active/archived)")
	searchCmd.Flags().String("sort", "relevance", "sort results: relevance, created, updated")
	searchCmd.Flags().String("created-after", "", "filter memos created on or after this date (YYYY-MM-DD)")
	searchCmd.Flags().String("created-before", "", "filter memos created on or before this date (YYYY-MM-DD)")
	searchCmd.Flags().String("layer", "", "filter by layer (concrete, abstract)")
	searchCmd.Flags().String("author", "", "filter by memo author")

	rootCmd.AddCommand(searchCmd)
}
