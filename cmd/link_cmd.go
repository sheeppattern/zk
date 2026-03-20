package cmd

import (
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"github.com/ymh/zk/internal/model"
	"github.com/ymh/zk/internal/store"
)

var linkCmd = &cobra.Command{
	Use:   "link",
	Short: "Manage links between notes",
	Long:  "Commands for adding, removing, and listing links between Zettelkasten notes.",
}

var linkAddCmd = &cobra.Command{
	Use:   "add <sourceID> <targetID>",
	Short: "Add a bidirectional link between two notes",
	Long:  "Add a bidirectional link. Use --target-project for cross-project links.",
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		sourceID := args[0]
		targetID := args[1]

		relType, _ := cmd.Flags().GetString("type")
		weight, _ := cmd.Flags().GetFloat64("weight")
		targetProject, _ := cmd.Flags().GetString("target-project")

		if !model.IsValidRelationType(relType) {
			fmt.Fprintf(os.Stderr, "error: invalid relation type %q (valid: %s)\n",
				relType, strings.Join(model.ValidRelationTypes(), ", "))
			os.Exit(1)
		}

		if weight < 0.0 || weight > 1.0 {
			fmt.Fprintln(os.Stderr, "error: weight must be between 0.0 and 1.0")
			os.Exit(1)
		}

		storePath := getStorePath(cmd)
		s := store.NewStore(storePath)

		// Source note uses --project, target note uses --target-project (or --project if not set)
		sourceProject := flagProject
		if targetProject == "" {
			targetProject = flagProject
		}

		sourceNote, err := s.GetNote(sourceProject, sourceID)
		if err != nil {
			return fmt.Errorf("load source note: %w", err)
		}

		targetNote, err := s.GetNote(targetProject, targetID)
		if err != nil {
			return fmt.Errorf("load target note: %w", err)
		}

		// Add forward link on source note.
		sourceNote.Links = append(sourceNote.Links, model.Link{
			TargetID:     targetID,
			RelationType: relType,
			Weight:       weight,
		})
		if err := s.UpdateNote(sourceNote); err != nil {
			return fmt.Errorf("save source note: %w", err)
		}

		// Add reverse link on target note.
		targetNote.Links = append(targetNote.Links, model.Link{
			TargetID:     sourceID,
			RelationType: relType,
			Weight:       weight,
		})
		if err := s.UpdateNote(targetNote); err != nil {
			return fmt.Errorf("save target note: %w", err)
		}

		if sourceProject != targetProject {
			fmt.Fprintf(os.Stderr, "linked %s(%s) → %s(%s) (type: %s, weight: %.2f)\n",
				sourceID, sourceProject, targetID, targetProject, relType, weight)
		} else {
			fmt.Fprintf(os.Stderr, "linked %s → %s (type: %s, weight: %.2f)\n",
				sourceID, targetID, relType, weight)
		}
		return nil
	},
}

var linkRemoveCmd = &cobra.Command{
	Use:   "remove <sourceID> <targetID>",
	Short: "Remove a bidirectional link between two notes",
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		sourceID := args[0]
		targetID := args[1]

		storePath := getStorePath(cmd)
		s := store.NewStore(storePath)

		// Remove forward link from source note.
		sourceNote, err := s.GetNote(flagProject, sourceID)
		if err != nil {
			return fmt.Errorf("load source note: %w", err)
		}
		sourceNote.Links = removeLink(sourceNote.Links, targetID)
		if err := s.UpdateNote(sourceNote); err != nil {
			return fmt.Errorf("save source note: %w", err)
		}

		// Remove reverse link from target note.
		targetNote, err := s.GetNote(flagProject, targetID)
		if err != nil {
			return fmt.Errorf("load target note: %w", err)
		}
		targetNote.Links = removeLink(targetNote.Links, sourceID)
		if err := s.UpdateNote(targetNote); err != nil {
			return fmt.Errorf("save target note: %w", err)
		}

		fmt.Fprintf(os.Stderr, "removed link %s → %s\n", sourceID, targetID)
		return nil
	},
}

var linkListCmd = &cobra.Command{
	Use:   "list <noteID>",
	Short: "List outgoing and incoming links for a note",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		noteID := args[0]

		storePath := getStorePath(cmd)
		s := store.NewStore(storePath)
		f := getFormatter()

		note, err := s.GetNote(flagProject, noteID)
		if err != nil {
			return fmt.Errorf("load note: %w", err)
		}

		outgoing := note.Links

		// Scan all notes in the project to find incoming links (backlinks).
		allNotes, err := s.ListNotes(flagProject)
		if err != nil {
			return fmt.Errorf("list notes: %w", err)
		}

		var incoming []model.Link
		for _, n := range allNotes {
			if n.ID == noteID {
				continue
			}
			for _, l := range n.Links {
				if l.TargetID == noteID {
					incoming = append(incoming, model.Link{
						TargetID:     n.ID,
						RelationType: l.RelationType,
						Weight:       l.Weight,
					})
				}
			}
		}

		result := struct {
			Outgoing []model.Link `json:"outgoing" yaml:"outgoing"`
			Incoming []model.Link `json:"incoming" yaml:"incoming"`
		}{
			Outgoing: outgoing,
			Incoming: incoming,
		}

		if result.Outgoing == nil {
			result.Outgoing = []model.Link{}
		}
		if result.Incoming == nil {
			result.Incoming = []model.Link{}
		}

		switch f.Format {
		case "json":
			return f.PrintJSON(result)
		case "yaml":
			return f.PrintYAML(result)
		default:
			return f.PrintJSON(result)
		}
	},
}

// removeLink filters out all links whose TargetID matches the given id.
func removeLink(links []model.Link, targetID string) []model.Link {
	filtered := make([]model.Link, 0, len(links))
	for _, l := range links {
		if l.TargetID != targetID {
			filtered = append(filtered, l)
		}
	}
	return filtered
}

func init() {
	linkAddCmd.Flags().String("type", "related", "relation type (e.g. related, supports, contradicts, extends, causes, example-of)")
	linkAddCmd.Flags().Float64("weight", 0.5, "link weight between 0.0 and 1.0")
	linkAddCmd.Flags().String("target-project", "", "project of the target note (for cross-project links)")

	linkCmd.AddCommand(linkAddCmd)
	linkCmd.AddCommand(linkRemoveCmd)
	linkCmd.AddCommand(linkListCmd)

	rootCmd.AddCommand(linkCmd)
}
