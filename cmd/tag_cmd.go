package cmd

import (
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/spf13/cobra"
	"github.com/sheeppattern/zk/internal/store"
)

var tagCmd = &cobra.Command{
	Use:   "tag",
	Short: "Manage tags",
	Long:  "Add, remove, replace, list, and batch-add tags on Zettelkasten notes.",
}

var tagAddCmd = &cobra.Command{
	Use:   "add <noteID> <tag1> [tag2...]",
	Short: "Add tags to a note",
	Args:  cobra.MinimumNArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		noteID := args[0]
		newTags := args[1:]

		s := store.NewStore(getStorePath(cmd))
		note, err := s.GetNote(flagProject, noteID)
		if err != nil {
			return fmt.Errorf("get note: %w", err)
		}

		existing := make(map[string]bool, len(note.Tags))
		for _, t := range note.Tags {
			existing[t] = true
		}
		for _, t := range newTags {
			if !existing[t] {
				note.Tags = append(note.Tags, t)
				existing[t] = true
			}
		}

		if err := s.UpdateNote(note); err != nil {
			return fmt.Errorf("update note: %w", err)
		}

		return getFormatter().PrintNote(note)
	},
}

var tagRemoveCmd = &cobra.Command{
	Use:   "remove <noteID> <tag1> [tag2...]",
	Short: "Remove tags from a note",
	Args:  cobra.MinimumNArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		noteID := args[0]
		removeTags := args[1:]

		s := store.NewStore(getStorePath(cmd))
		note, err := s.GetNote(flagProject, noteID)
		if err != nil {
			return fmt.Errorf("get note: %w", err)
		}

		removeSet := make(map[string]bool, len(removeTags))
		for _, t := range removeTags {
			removeSet[t] = true
		}

		filtered := make([]string, 0, len(note.Tags))
		for _, t := range note.Tags {
			if !removeSet[t] {
				filtered = append(filtered, t)
			}
		}
		note.Tags = filtered

		if err := s.UpdateNote(note); err != nil {
			return fmt.Errorf("update note: %w", err)
		}

		return getFormatter().PrintNote(note)
	},
}

var tagReplaceCmd = &cobra.Command{
	Use:   "replace <oldTag> <newTag>",
	Short: "Replace a tag across all notes in the project",
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		oldTag := args[0]
		newTag := args[1]

		s := store.NewStore(getStorePath(cmd))
		notes, err := s.ListNotes(flagProject)
		if err != nil {
			return fmt.Errorf("list notes: %w", err)
		}

		affected := 0
		for _, note := range notes {
			replaced := false
			hasNew := false
			for _, t := range note.Tags {
				if t == newTag {
					hasNew = true
				}
			}
			for i, t := range note.Tags {
				if t == oldTag {
					if hasNew {
						// Remove the old tag since newTag already exists
						note.Tags = append(note.Tags[:i], note.Tags[i+1:]...)
					} else {
						note.Tags[i] = newTag
					}
					replaced = true
					break
				}
			}
			if replaced {
				if err := s.UpdateNote(note); err != nil {
					return fmt.Errorf("update note %s: %w", note.ID, err)
				}
				affected++
			}
		}

		fmt.Fprintf(os.Stderr, "Replaced tag %q with %q in %d note(s)\n", oldTag, newTag, affected)
		return nil
	},
}

var tagListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all unique tags in the project",
	RunE: func(cmd *cobra.Command, args []string) error {
		s := store.NewStore(getStorePath(cmd))
		notes, err := s.ListNotes(flagProject)
		if err != nil {
			return fmt.Errorf("list notes: %w", err)
		}

		tagSet := make(map[string]bool)
		for _, note := range notes {
			for _, t := range note.Tags {
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
	Use:   "batch-add <tag> <noteID1> [noteID2...]",
	Short: "Add a tag to multiple notes",
	Args:  cobra.MinimumNArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		tag := args[0]
		noteIDs := args[1:]

		s := store.NewStore(getStorePath(cmd))
		affected := 0

		for _, noteID := range noteIDs {
			note, err := s.GetNote(flagProject, noteID)
			if err != nil {
				return fmt.Errorf("get note %s: %w", noteID, err)
			}

			hasTag := false
			for _, t := range note.Tags {
				if t == tag {
					hasTag = true
					break
				}
			}
			if hasTag {
				continue
			}

			note.Tags = append(note.Tags, tag)
			if err := s.UpdateNote(note); err != nil {
				return fmt.Errorf("update note %s: %w", noteID, err)
			}
			affected++
		}

		fmt.Fprintf(os.Stderr, "Added tag %q to %d note(s)\n", tag, affected)
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
