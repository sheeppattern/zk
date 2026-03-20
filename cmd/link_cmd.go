package cmd

import (
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/spf13/cobra"
	"github.com/sheeppattern/zk/internal/model"
	"github.com/sheeppattern/zk/internal/output"
	"github.com/sheeppattern/zk/internal/store"
)

var linkCmd = &cobra.Command{
	Use:   "link",
	Short: "Manage links between notes",
	Long:  "Commands for adding, removing, and listing links between Zettelkasten notes.",
}

// hasLink checks whether a link with the given targetID and relationType already exists.
func hasLink(links []model.Link, targetID, relationType string) bool {
	for _, l := range links {
		if l.TargetID == targetID && l.RelationType == relationType {
			return true
		}
	}
	return false
}

// LinkWithSource represents a link discovered during BFS traversal.
type LinkWithSource struct {
	SourceID     string  `json:"source_id" yaml:"source_id"`
	TargetID     string  `json:"target_id" yaml:"target_id"`
	RelationType string  `json:"relation_type" yaml:"relation_type"`
	Weight       float64 `json:"weight" yaml:"weight"`
	Depth        int     `json:"depth" yaml:"depth"`
}

// IncomingLink represents a backlink from any project, including cross-project backlinks.
type IncomingLink struct {
	SourceNoteID  string  `json:"source_note_id" yaml:"source_note_id"`
	SourceProject string  `json:"source_project" yaml:"source_project"`
	RelationType  string  `json:"relation_type" yaml:"relation_type"`
	Weight        float64 `json:"weight" yaml:"weight"`
}

var linkAddCmd = &cobra.Command{
	Use:   "add <sourceID> <targetID>",
	Short: "Add a bidirectional link between two notes",
	Long:  "Add a bidirectional link. Use --target-project for cross-project links.",
	Example: `  zk link add N-AAAAAA N-BBBBBB --type supports --weight 0.8 --project P-XXXXXX
  zk link add N-AAAAAA N-BBBBBB --type extends --project P-111 --target-project P-222`,
	Args: cobra.ExactArgs(2),
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

		// Duplicate link prevention: check before adding forward link.
		if hasLink(sourceNote.Links, targetID, relType) {
			statusf("link %s → %s (type: %s) already exists, skipping", sourceID, targetID, relType)
			return nil
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

		// Duplicate link prevention: check before adding reverse link.
		if hasLink(targetNote.Links, sourceID, relType) {
			statusf("reverse link %s → %s (type: %s) already exists, skipping reverse", targetID, sourceID, relType)
		} else {
			// Add reverse link on target note.
			targetNote.Links = append(targetNote.Links, model.Link{
				TargetID:     sourceID,
				RelationType: relType,
				Weight:       weight,
			})
			if err := s.UpdateNote(targetNote); err != nil {
				return fmt.Errorf("save target note: %w", err)
			}
		}

		if sourceProject != targetProject {
			statusf("linked %s(%s) → %s(%s) (type: %s, weight: %.2f)",
				sourceID, sourceProject, targetID, targetProject, relType, weight)
		} else {
			statusf("linked %s → %s (type: %s, weight: %.2f)",
				sourceID, targetID, relType, weight)
		}
		return nil
	},
}

var linkRemoveCmd = &cobra.Command{
	Use:   "remove <sourceID> <targetID>",
	Short: "Remove a bidirectional link between two notes",
	Example: `  zk link remove N-AAAAAA N-BBBBBB --project P-XXXXXX`,
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

		statusf("removed link %s → %s", sourceID, targetID)
		return nil
	},
}

var linkListCmd = &cobra.Command{
	Use:   "list <noteID>",
	Short: "List outgoing and incoming links for a note",
	Example: `  zk link list N-XXXXXX --project P-XXXXXX
  zk link list N-XXXXXX --type supports --sort-weight
  zk link list N-XXXXXX --depth 3 --project P-XXXXXX`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		noteID := args[0]

		typeFilter, _ := cmd.Flags().GetString("type")
		sortWeight, _ := cmd.Flags().GetBool("sort-weight")
		depth, _ := cmd.Flags().GetInt("depth")

		storePath := getStorePath(cmd)
		s := store.NewStore(storePath)
		f := getFormatter()

		// BFS traversal for depth > 1.
		if depth > 1 {
			return linkListBFS(s, f, noteID, typeFilter, sortWeight, depth)
		}

		note, err := s.GetNote(flagProject, noteID)
		if err != nil {
			return fmt.Errorf("load note: %w", err)
		}

		outgoing := note.Links

		// Scan all notes in the current project to find incoming links (backlinks).
		allNotes, err := s.ListNotes(flagProject)
		if err != nil {
			return fmt.Errorf("list notes: %w", err)
		}

		var incoming []IncomingLink
		for _, n := range allNotes {
			if n.ID == noteID {
				continue
			}
			for _, l := range n.Links {
				if l.TargetID == noteID {
					incoming = append(incoming, IncomingLink{
						SourceNoteID:  n.ID,
						SourceProject: flagProject,
						RelationType:  l.RelationType,
						Weight:        l.Weight,
					})
				}
			}
		}

		// Cross-project backlink scan: check other projects for notes linking to noteID.
		projects, err := s.ListProjects()
		if err != nil {
			debugf("could not list projects for cross-project backlinks: %v", err)
		} else {
			for _, p := range projects {
				if p.ID == flagProject {
					continue
				}
				projectNotes, err := s.ListNotes(p.ID)
				if err != nil {
					debugf("could not list notes for project %s: %v", p.ID, err)
					continue
				}
				for _, n := range projectNotes {
					for _, l := range n.Links {
						if l.TargetID == noteID {
							incoming = append(incoming, IncomingLink{
								SourceNoteID:  n.ID,
								SourceProject: p.ID,
								RelationType:  l.RelationType,
								Weight:        l.Weight,
							})
						}
					}
				}
			}
		}

		// Also scan global notes if flagProject is not empty (i.e. we are in a project scope).
		if flagProject != "" {
			globalNotes, err := s.ListNotes("")
			if err != nil {
				debugf("could not list global notes for cross-project backlinks: %v", err)
			} else {
				for _, n := range globalNotes {
					for _, l := range n.Links {
						if l.TargetID == noteID {
							incoming = append(incoming, IncomingLink{
								SourceNoteID:  n.ID,
								SourceProject: "",
								RelationType:  l.RelationType,
								Weight:        l.Weight,
							})
						}
					}
				}
			}
		}

		// Filter by --type if set.
		if typeFilter != "" {
			outgoing = filterLinksByType(outgoing, typeFilter)
			incoming = filterIncomingByType(incoming, typeFilter)
		}

		// Sort by weight descending if --sort-weight is set.
		if sortWeight {
			sort.Slice(outgoing, func(i, j int) bool {
				return outgoing[i].Weight > outgoing[j].Weight
			})
			sort.Slice(incoming, func(i, j int) bool {
				return incoming[i].Weight > incoming[j].Weight
			})
		}

		result := struct {
			Outgoing []model.Link   `json:"outgoing" yaml:"outgoing"`
			Incoming []IncomingLink `json:"incoming" yaml:"incoming"`
		}{
			Outgoing: outgoing,
			Incoming: incoming,
		}

		if result.Outgoing == nil {
			result.Outgoing = []model.Link{}
		}
		if result.Incoming == nil {
			result.Incoming = []IncomingLink{}
		}

		switch f.Format {
		case "json":
			return f.PrintJSON(result)
		case "yaml":
			return f.PrintYAML(result)
		case "md":
			return printLinkListMD(noteID, result.Outgoing, result.Incoming)
		default:
			return fmt.Errorf("unsupported format: %s", f.Format)
		}
	},
}

// linkListBFS performs a breadth-first traversal of outgoing links up to the given depth.
func linkListBFS(s *store.Store, f *output.Formatter, noteID, typeFilter string, sortWeight bool, maxDepth int) error {
	visited := map[string]bool{noteID: true}
	queue := []string{noteID}
	var allLinks []LinkWithSource
	cache := map[string]*model.Note{}

	for currentDepth := 1; currentDepth <= maxDepth && len(queue) > 0; currentDepth++ {
		var nextQueue []string
		for _, id := range queue {
			note, ok := cache[id]
			if !ok {
				var err error
				note, err = s.GetNote(flagProject, id)
				if err != nil {
					continue // skip notes that can't be loaded
				}
				cache[id] = note
			}
			for _, l := range note.Links {
				if typeFilter != "" && !strings.EqualFold(l.RelationType, typeFilter) {
					continue
				}
				allLinks = append(allLinks, LinkWithSource{
					SourceID:     id,
					TargetID:     l.TargetID,
					RelationType: l.RelationType,
					Weight:       l.Weight,
					Depth:        currentDepth,
				})
				if !visited[l.TargetID] {
					visited[l.TargetID] = true
					nextQueue = append(nextQueue, l.TargetID)
				}
			}
		}
		queue = nextQueue
	}

	if sortWeight {
		sort.Slice(allLinks, func(i, j int) bool {
			return allLinks[i].Weight > allLinks[j].Weight
		})
	}

	if allLinks == nil {
		allLinks = []LinkWithSource{}
	}

	switch f.Format {
	case "json":
		return f.PrintJSON(allLinks)
	case "yaml":
		return f.PrintYAML(allLinks)
	case "md":
		return printLinkBFSMD(noteID, allLinks)
	default:
		return fmt.Errorf("unsupported format: %s", f.Format)
	}
}

// filterLinksByType returns only links matching the given relation type (case-insensitive).
func filterLinksByType(links []model.Link, relType string) []model.Link {
	var filtered []model.Link
	for _, l := range links {
		if strings.EqualFold(l.RelationType, relType) {
			filtered = append(filtered, l)
		}
	}
	return filtered
}

// filterIncomingByType returns only incoming links matching the given relation type (case-insensitive).
func filterIncomingByType(links []IncomingLink, relType string) []IncomingLink {
	var filtered []IncomingLink
	for _, l := range links {
		if strings.EqualFold(l.RelationType, relType) {
			filtered = append(filtered, l)
		}
	}
	return filtered
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

// printLinkListMD renders outgoing and incoming links as markdown tables.
func printLinkListMD(noteID string, outgoing []model.Link, incoming []IncomingLink) error {
	var b strings.Builder

	fmt.Fprintf(&b, "# Links for %s\n\n", noteID)

	fmt.Fprintln(&b, "## Outgoing")
	if len(outgoing) == 0 {
		fmt.Fprintln(&b, "\nNo outgoing links.")
	} else {
		fmt.Fprintln(&b, "")
		fmt.Fprintln(&b, "| Target | Type | Weight |")
		fmt.Fprintln(&b, "|--------|------|--------|")
		for _, l := range outgoing {
			fmt.Fprintf(&b, "| %s | %s | %.2f |\n", l.TargetID, l.RelationType, l.Weight)
		}
		fmt.Fprintln(&b)
	}

	fmt.Fprintln(&b, "## Incoming")
	if len(incoming) == 0 {
		fmt.Fprintln(&b, "\nNo incoming links.")
	} else {
		fmt.Fprintln(&b, "")
		fmt.Fprintln(&b, "| Source | Project | Type | Weight |")
		fmt.Fprintln(&b, "|--------|---------|------|--------|")
		for _, l := range incoming {
			proj := l.SourceProject
			if proj == "" {
				proj = "global"
			}
			fmt.Fprintf(&b, "| %s | %s | %s | %.2f |\n", l.SourceNoteID, proj, l.RelationType, l.Weight)
		}
		fmt.Fprintln(&b)
	}

	_, err := fmt.Fprint(os.Stdout, b.String())
	return err
}

// printLinkBFSMD renders BFS traversal results as a markdown table.
func printLinkBFSMD(noteID string, links []LinkWithSource) error {
	var b strings.Builder

	fmt.Fprintf(&b, "# Link Graph for %s\n\n", noteID)

	if len(links) == 0 {
		fmt.Fprintln(&b, "No links found.")
	} else {
		fmt.Fprintln(&b, "| Source | Target | Type | Weight | Depth |")
		fmt.Fprintln(&b, "|--------|--------|------|--------|-------|")
		for _, l := range links {
			fmt.Fprintf(&b, "| %s | %s | %s | %.2f | %d |\n", l.SourceID, l.TargetID, l.RelationType, l.Weight, l.Depth)
		}
		fmt.Fprintln(&b)
	}

	_, err := fmt.Fprint(os.Stdout, b.String())
	return err
}

func init() {
	linkAddCmd.Flags().String("type", "related", "relation type (e.g. related, supports, contradicts, extends, causes, example-of)")
	linkAddCmd.Flags().Float64("weight", 0.5, "link weight between 0.0 and 1.0")
	linkAddCmd.Flags().String("target-project", "", "project of the target note (for cross-project links)")

	linkListCmd.Flags().String("type", "", "filter links by relation type")
	linkListCmd.Flags().Bool("sort-weight", false, "sort links by weight descending")
	linkListCmd.Flags().Int("depth", 1, "BFS traversal depth for outgoing links")

	linkCmd.AddCommand(linkAddCmd)
	linkCmd.AddCommand(linkRemoveCmd)
	linkCmd.AddCommand(linkListCmd)

	rootCmd.AddCommand(linkCmd)
}
