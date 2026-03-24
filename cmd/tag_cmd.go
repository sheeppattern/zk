package cmd

import (
	"fmt"
	"os"
	"sort"
	"strconv"
	"strings"

	"github.com/spf13/cobra"
)

var tagCmd = &cobra.Command{
	Use:   "tag",
	Short: "Manage tags",
	Long:  "Add, remove, replace, list, and batch-add tags on Zettelkasten memos.",
}

var tagAddCmd = &cobra.Command{
	Use:   "add <memoID> <tag1> [tag2...]",
	Short: "Add tags to a memo",
	Example: `  zk tag add 1 important urgent`,
	Args:  cobra.MinimumNArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		memoID, err := strconv.ParseInt(args[0], 10, 64)
		if err != nil {
			return fmt.Errorf("invalid memo ID %q: %w", args[0], err)
		}
		newTags := args[1:]

		s, err := openStore(cmd)
		if err != nil {
			return err
		}
		defer s.Close()

		memo, err := s.GetMemo(memoID)
		if err != nil {
			return fmt.Errorf("get memo: %w", err)
		}

		existing := make(map[string]bool, len(memo.Tags))
		for _, t := range memo.Tags {
			existing[t] = true
		}
		for _, t := range newTags {
			if !existing[t] {
				memo.Tags = append(memo.Tags, t)
				existing[t] = true
			}
		}

		if err := s.UpdateMemo(memo); err != nil {
			return fmt.Errorf("update memo: %w", err)
		}

		return getFormatter().PrintMemo(memo)
	},
}

var tagRemoveCmd = &cobra.Command{
	Use:   "remove <memoID> <tag1> [tag2...]",
	Short: "Remove tags from a memo",
	Example: `  zk tag remove 1 draft`,
	Args:  cobra.MinimumNArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		memoID, err := strconv.ParseInt(args[0], 10, 64)
		if err != nil {
			return fmt.Errorf("invalid memo ID %q: %w", args[0], err)
		}
		removeTags := args[1:]

		s, err := openStore(cmd)
		if err != nil {
			return err
		}
		defer s.Close()

		memo, err := s.GetMemo(memoID)
		if err != nil {
			return fmt.Errorf("get memo: %w", err)
		}

		removeSet := make(map[string]bool, len(removeTags))
		for _, t := range removeTags {
			removeSet[t] = true
		}

		filtered := make([]string, 0, len(memo.Tags))
		for _, t := range memo.Tags {
			if !removeSet[t] {
				filtered = append(filtered, t)
			}
		}
		memo.Tags = filtered

		if err := s.UpdateMemo(memo); err != nil {
			return fmt.Errorf("update memo: %w", err)
		}

		return getFormatter().PrintMemo(memo)
	},
}

var tagReplaceCmd = &cobra.Command{
	Use:   "replace <oldTag> <newTag>",
	Short: "Replace a tag across all memos",
	Example: `  zk tag replace old-tag new-tag`,
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		oldTag := args[0]
		newTag := args[1]

		s, err := openStore(cmd)
		if err != nil {
			return err
		}
		defer s.Close()

		memos, err := s.ListAllMemos()
		if err != nil {
			return fmt.Errorf("list memos: %w", err)
		}

		affected := 0
		for _, memo := range memos {
			var newTags []string
			replaced := false
			addedNew := false
			for _, t := range memo.Tags {
				if t == oldTag {
					replaced = true
					if !addedNew {
						newTags = append(newTags, newTag)
						addedNew = true
					}
				} else if t == newTag {
					if !addedNew {
						newTags = append(newTags, t)
						addedNew = true
					}
				} else {
					newTags = append(newTags, t)
				}
			}
			if replaced {
				memo.Tags = newTags
				if err := s.UpdateMemo(memo); err != nil {
					return fmt.Errorf("update memo %d: %w", memo.ID, err)
				}
				affected++
			}
		}

		statusf("Replaced tag %q with %q in %d memo(s)", oldTag, newTag, affected)
		return nil
	},
}

var tagListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all unique tags",
	Example: `  zk tag list`,
	RunE: func(cmd *cobra.Command, args []string) error {
		s, err := openStore(cmd)
		if err != nil {
			return err
		}
		defer s.Close()

		memos, err := s.ListAllMemos()
		if err != nil {
			return fmt.Errorf("list memos: %w", err)
		}

		tagSet := make(map[string]bool)
		for _, memo := range memos {
			for _, t := range memo.Tags {
				tagSet[t] = true
			}
		}

		tags := make([]string, 0, len(tagSet))
		for t := range tagSet {
			tags = append(tags, t)
		}
		sort.Strings(tags)

		f := getFormatter()
		switch f.Format {
		case "json":
			return f.PrintJSON(tags)
		case "yaml":
			return f.PrintYAML(tags)
		case "md":
			var b strings.Builder
			for _, t := range tags {
				fmt.Fprintf(&b, "- %s\n", t)
			}
			_, err := fmt.Fprint(os.Stdout, b.String())
			return err
		default:
			return fmt.Errorf("unsupported format: %s", f.Format)
		}
	},
}

var tagBatchAddCmd = &cobra.Command{
	Use:   "batch-add <tag> <memoID1> [memoID2...]",
	Short: "Add a tag to multiple memos",
	Example: `  zk tag batch-add reviewed 1 2 3`,
	Args:  cobra.MinimumNArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		tag := args[0]
		memoIDStrs := args[1:]

		s, err := openStore(cmd)
		if err != nil {
			return err
		}
		defer s.Close()

		affected := 0
		for _, idStr := range memoIDStrs {
			memoID, err := strconv.ParseInt(idStr, 10, 64)
			if err != nil {
				return fmt.Errorf("invalid memo ID %q: %w", idStr, err)
			}

			memo, err := s.GetMemo(memoID)
			if err != nil {
				return fmt.Errorf("get memo %d: %w", memoID, err)
			}

			hasTag := false
			for _, t := range memo.Tags {
				if t == tag {
					hasTag = true
					break
				}
			}
			if hasTag {
				continue
			}

			memo.Tags = append(memo.Tags, tag)
			if err := s.UpdateMemo(memo); err != nil {
				return fmt.Errorf("update memo %d: %w", memoID, err)
			}
			affected++
		}

		statusf("Added tag %q to %d memo(s)", tag, affected)
		return nil
	},
}

func init() {
	tagCmd.AddCommand(tagAddCmd)
	tagCmd.AddCommand(tagRemoveCmd)
	tagCmd.AddCommand(tagReplaceCmd)
	tagCmd.AddCommand(tagListCmd)
	tagCmd.AddCommand(tagBatchAddCmd)

	rootCmd.AddCommand(tagCmd)
}
