package cmd

import (
	"crypto/rand"
	"fmt"
	"math/big"
	"strconv"

	"github.com/spf13/cobra"
	"github.com/sheeppattern/nete/internal/model"
)

var memoCmd = &cobra.Command{
	Use:   "memo",
	Short: "Manage memos",
	Long:  "Create, read, update, delete, list, move, and pick random Zettelkasten memos.",
}

var memoCreateCmd = &cobra.Command{
	Use:   "create",
	Short: "Create a new memo",
	Example: `  nete memo create --title "Discovery" --content "Found something" --tags "research,important"
  nete memo create --title "Idea" --content "..." --note 1
  nete memo create --title "Insight" --content "..." --layer abstract`,
	RunE: func(cmd *cobra.Command, args []string) error {
		title, _ := cmd.Flags().GetString("title")
		content, _ := cmd.Flags().GetString("content")
		tags, _ := cmd.Flags().GetStringSlice("tags")
		layerFlag, _ := cmd.Flags().GetString("layer")
		summary, _ := cmd.Flags().GetString("summary")
		authorFlag, _ := cmd.Flags().GetString("author")

		// Validate layer flag.
		if layerFlag != model.LayerConcrete && layerFlag != model.LayerAbstract {
			return fmt.Errorf("invalid layer %q: must be %q or %q", layerFlag, model.LayerConcrete, model.LayerAbstract)
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
			Content: content,
			Tags:    tags,
			Layer:   layerFlag,
			NoteID:  flagNote,
			Metadata: model.Metadata{
				Status:  model.StatusActive,
				Summary: summary,
				Author:  author,
			},
		}

		if err := s.CreateMemo(memo); err != nil {
			return fmt.Errorf("create memo: %w", err)
		}

		return getFormatter().PrintMemo(memo)
	},
}

var memoGetCmd = &cobra.Command{
	Use:   "get <id>",
	Short: "Get a memo by ID",
	Example: `  nete memo get 1
  nete memo get 1 --format md`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		id, err := strconv.ParseInt(args[0], 10, 64)
		if err != nil {
			return fmt.Errorf("invalid memo ID %q: %w", args[0], err)
		}

		s, err := openStore(cmd)
		if err != nil {
			return err
		}
		defer s.Close()

		memo, err := s.GetMemo(id)
		if err != nil {
			return fmt.Errorf("memo %d not found", id)
		}

		return getFormatter().PrintMemo(memo)
	},
}

var memoListCmd = &cobra.Command{
	Use:   "list",
	Short: "List memos",
	Example: `  nete memo list --note 1
  nete memo list --format md
  nete memo list --layer abstract`,
	RunE: func(cmd *cobra.Command, args []string) error {
		layerFilter, _ := cmd.Flags().GetString("layer")

		s, err := openStore(cmd)
		if err != nil {
			return err
		}
		defer s.Close()

		memos, err := s.ListMemos(flagNote)
		if err != nil {
			return fmt.Errorf("list memos: %w", err)
		}

		if layerFilter != "" {
			filtered := make([]*model.Memo, 0, len(memos))
			for _, m := range memos {
				if m.Layer == layerFilter {
					filtered = append(filtered, m)
				}
			}
			memos = filtered
		}

		return getFormatter().PrintMemos(memos)
	},
}

var memoUpdateCmd = &cobra.Command{
	Use:   "update <id>",
	Short: "Update an existing memo",
	Example: `  nete memo update 1 --title "New Title"
  nete memo update 1 --tags "new-tag" --status archived`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		id, err := strconv.ParseInt(args[0], 10, 64)
		if err != nil {
			return fmt.Errorf("invalid memo ID %q: %w", args[0], err)
		}

		s, err := openStore(cmd)
		if err != nil {
			return err
		}
		defer s.Close()

		memo, err := s.GetMemo(id)
		if err != nil {
			return fmt.Errorf("memo %d not found", id)
		}

		if cmd.Flags().Changed("title") {
			memo.Title, _ = cmd.Flags().GetString("title")
		}
		if cmd.Flags().Changed("content") {
			memo.Content, _ = cmd.Flags().GetString("content")
		}
		if cmd.Flags().Changed("tags") {
			memo.Tags, _ = cmd.Flags().GetStringSlice("tags")
		}
		if cmd.Flags().Changed("status") {
			memo.Metadata.Status, _ = cmd.Flags().GetString("status")
		}
		if cmd.Flags().Changed("summary") {
			memo.Metadata.Summary, _ = cmd.Flags().GetString("summary")
		}
		if cmd.Flags().Changed("author") {
			memo.Metadata.Author, _ = cmd.Flags().GetString("author")
		}

		if err := s.UpdateMemo(memo); err != nil {
			return fmt.Errorf("update memo: %w", err)
		}

		return getFormatter().PrintMemo(memo)
	},
}

var memoDeleteCmd = &cobra.Command{
	Use:   "delete <id>",
	Short: "Delete a memo by ID",
	Example: `  nete memo delete 1`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		id, err := strconv.ParseInt(args[0], 10, 64)
		if err != nil {
			return fmt.Errorf("invalid memo ID %q: %w", args[0], err)
		}

		s, err := openStore(cmd)
		if err != nil {
			return err
		}
		defer s.Close()

		if err := s.DeleteMemo(id); err != nil {
			return fmt.Errorf("delete memo: %w", err)
		}

		statusf("Deleted memo %d", id)
		return nil
	},
}

var memoMoveCmd = &cobra.Command{
	Use:   "move <memoID> <targetNoteID>",
	Short: "Move a memo to a different note",
	Example: `  nete memo move 1 2`,
	Args: cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		memoID, err := strconv.ParseInt(args[0], 10, 64)
		if err != nil {
			return fmt.Errorf("invalid memo ID %q: %w", args[0], err)
		}
		targetNoteID, err := strconv.ParseInt(args[1], 10, 64)
		if err != nil {
			return fmt.Errorf("invalid target note ID %q: %w", args[1], err)
		}

		s, err := openStore(cmd)
		if err != nil {
			return err
		}
		defer s.Close()

		if err := s.MoveMemo(memoID, targetNoteID); err != nil {
			return fmt.Errorf("move memo: %w", err)
		}

		statusf("moved memo %d to note %d", memoID, targetNoteID)
		return nil
	},
}

var memoRandomCmd = &cobra.Command{
	Use:   "random",
	Short: "Pick a random memo from all memos across every note",
	Example: `  nete memo random
  nete memo random --layer abstract
  nete memo random --format md`,
	RunE: func(cmd *cobra.Command, args []string) error {
		layerFilter, _ := cmd.Flags().GetString("layer")

		s, err := openStore(cmd)
		if err != nil {
			return err
		}
		defer s.Close()

		allMemos, err := s.ListAllMemos()
		if err != nil {
			return fmt.Errorf("list memos: %w", err)
		}

		if layerFilter != "" {
			filtered := make([]*model.Memo, 0, len(allMemos))
			for _, m := range allMemos {
				if m.Layer == layerFilter {
					filtered = append(filtered, m)
				}
			}
			allMemos = filtered
		}

		if len(allMemos) == 0 {
			return fmt.Errorf("no memos found")
		}

		idx, err := rand.Int(rand.Reader, big.NewInt(int64(len(allMemos))))
		if err != nil {
			return fmt.Errorf("random selection: %w", err)
		}

		return getFormatter().PrintMemo(allMemos[idx.Int64()])
	},
}

func init() {
	// memoRandomCmd flags
	memoRandomCmd.Flags().String("layer", "", "filter by layer (concrete, abstract)")

	// memoCreateCmd flags
	memoCreateCmd.Flags().String("title", "", "memo title (required)")
	memoCreateCmd.Flags().String("content", "", "memo content")
	memoCreateCmd.Flags().StringSlice("tags", nil, "comma-separated tags")
	memoCreateCmd.Flags().String("layer", model.LayerConcrete, "memo layer (concrete, abstract)")
	memoCreateCmd.Flags().String("summary", "", "brief summary for quick scanning")
	memoCreateCmd.Flags().String("author", "", "memo author (e.g., claude, gemini, human)")
	_ = memoCreateCmd.MarkFlagRequired("title")

	// memoUpdateCmd flags
	memoUpdateCmd.Flags().String("title", "", "new title")
	memoUpdateCmd.Flags().String("content", "", "new content")
	memoUpdateCmd.Flags().StringSlice("tags", nil, "new tags")
	memoUpdateCmd.Flags().String("status", "", "new status (active, archived)")
	memoUpdateCmd.Flags().String("summary", "", "brief summary for quick scanning")
	memoUpdateCmd.Flags().String("author", "", "memo author (e.g., claude, gemini, human)")

	// memoListCmd flags
	memoListCmd.Flags().String("layer", "", "filter by layer (concrete, abstract)")

	// Register subcommands
	memoCmd.AddCommand(memoCreateCmd)
	memoCmd.AddCommand(memoGetCmd)
	memoCmd.AddCommand(memoListCmd)
	memoCmd.AddCommand(memoUpdateCmd)
	memoCmd.AddCommand(memoDeleteCmd)
	memoCmd.AddCommand(memoMoveCmd)
	memoCmd.AddCommand(memoRandomCmd)

	rootCmd.AddCommand(memoCmd)
}
