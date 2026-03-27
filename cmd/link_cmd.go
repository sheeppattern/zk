package cmd

import (
	"fmt"
	"os"
	"sort"
	"strconv"
	"strings"

	"github.com/spf13/cobra"
	"github.com/sheeppattern/nete/internal/model"
	"github.com/sheeppattern/nete/internal/output"
	"github.com/sheeppattern/nete/internal/store"
)

var linkCmd = &cobra.Command{
	Use:   "link",
	Short: "Manage links between memos",
	Long:  "Commands for adding, removing, and listing links between Zettelkasten memos.",
}

var linkAddCmd = &cobra.Command{
	Use:   "add <sourceID> <targetID>",
	Short: "Add a link between two memos",
	Example: `  nete link add 1 2 --type supports --weight 0.8
  nete link add 1 2 --type extends`,
	Args: cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		sourceID, err := strconv.ParseInt(args[0], 10, 64)
		if err != nil {
			return fmt.Errorf("invalid source ID %q: %w", args[0], err)
		}
		targetID, err := strconv.ParseInt(args[1], 10, 64)
		if err != nil {
			return fmt.Errorf("invalid target ID %q: %w", args[1], err)
		}

		relType, _ := cmd.Flags().GetString("type")
		weight, _ := cmd.Flags().GetFloat64("weight")

		if !model.IsValidRelationType(relType) {
			return fmt.Errorf("invalid relation type %q (valid: %s)", relType, strings.Join(model.ValidRelationTypes(), ", "))
		}

		if weight < 0.0 || weight > 1.0 {
			return fmt.Errorf("weight must be between 0.0 and 1.0")
		}

		s, err := openStore(cmd)
		if err != nil {
			return err
		}
		defer s.Close()

		if err := s.AddLink(sourceID, targetID, relType, weight); err != nil {
			return fmt.Errorf("add link: %w", err)
		}

		statusf("linked %d → %d (type: %s, weight: %.2f)", sourceID, targetID, relType, weight)
		return nil
	},
}

var linkRemoveCmd = &cobra.Command{
	Use:   "remove <sourceID> <targetID>",
	Short: "Remove a link between two memos",
	Example: `  nete link remove 1 2
  nete link remove 1 2 --type supports`,
	Args: cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		sourceID, err := strconv.ParseInt(args[0], 10, 64)
		if err != nil {
			return fmt.Errorf("invalid source ID %q: %w", args[0], err)
		}
		targetID, err := strconv.ParseInt(args[1], 10, 64)
		if err != nil {
			return fmt.Errorf("invalid target ID %q: %w", args[1], err)
		}

		relType, _ := cmd.Flags().GetString("type")
		if relType == "" {
			relType = "related"
		}

		if !model.IsValidRelationType(relType) {
			return fmt.Errorf("invalid relation type %q (valid: %s)", relType, strings.Join(model.ValidRelationTypes(), ", "))
		}

		s, err := openStore(cmd)
		if err != nil {
			return err
		}
		defer s.Close()

		if err := s.RemoveLink(sourceID, targetID, relType); err != nil {
			return fmt.Errorf("remove link: %w", err)
		}

		statusf("removed link %d → %d", sourceID, targetID)
		return nil
	},
}

var linkListCmd = &cobra.Command{
	Use:   "list <memoID>",
	Short: "List outgoing and incoming links for a memo",
	Example: `  nete link list 1
  nete link list 1 --type supports --sort-weight
  nete link list 1 --depth 3`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		memoID, err := strconv.ParseInt(args[0], 10, 64)
		if err != nil {
			return fmt.Errorf("invalid memo ID %q: %w", args[0], err)
		}

		typeFilter, _ := cmd.Flags().GetString("type")
		sortWeight, _ := cmd.Flags().GetBool("sort-weight")
		depth, _ := cmd.Flags().GetInt("depth")

		s, err := openStore(cmd)
		if err != nil {
			return err
		}
		defer s.Close()

		f := getFormatter()

		// BFS traversal for depth > 1.
		if depth > 1 {
			return linkListBFS(s, f, memoID, typeFilter, sortWeight, depth)
		}

		outgoing, incoming, err := s.ListLinks(memoID)
		if err != nil {
			return fmt.Errorf("list links: %w", err)
		}

		// Filter by --type if set.
		if typeFilter != "" {
			outgoing = filterLinksByType(outgoing, typeFilter)
			incoming = filterLinksByType(incoming, typeFilter)
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

		if outgoing == nil {
			outgoing = []model.Link{}
		}
		if incoming == nil {
			incoming = []model.Link{}
		}

		result := struct {
			Outgoing []model.Link `json:"outgoing" yaml:"outgoing"`
			Incoming []model.Link `json:"incoming" yaml:"incoming"`
		}{
			Outgoing: outgoing,
			Incoming: incoming,
		}

		switch f.Format {
		case "json":
			return f.PrintJSON(result)
		case "yaml":
			return f.PrintYAML(result)
		case "md":
			return printLinkListMD(memoID, result.Outgoing, result.Incoming)
		default:
			return fmt.Errorf("unsupported format: %s", f.Format)
		}
	},
}

func linkListBFS(s *store.Store, f *output.Formatter, memoID int64, typeFilter string, sortWeight bool, maxDepth int) error {
	links, err := s.ListLinksBFS(memoID, maxDepth)
	if err != nil {
		return fmt.Errorf("bfs traversal: %w", err)
	}

	// Filter by type if set.
	if typeFilter != "" {
		var filtered []store.LinkWithDepth
		for _, l := range links {
			if strings.EqualFold(l.RelationType, typeFilter) {
				filtered = append(filtered, l)
			}
		}
		links = filtered
	}

	if sortWeight {
		sort.Slice(links, func(i, j int) bool {
			return links[i].Weight > links[j].Weight
		})
	}

	if links == nil {
		links = []store.LinkWithDepth{}
	}

	switch f.Format {
	case "json":
		return f.PrintJSON(links)
	case "yaml":
		return f.PrintYAML(links)
	case "md":
		return printLinkBFSMD(memoID, links)
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

// printLinkListMD renders outgoing and incoming links as markdown tables.
func printLinkListMD(memoID int64, outgoing []model.Link, incoming []model.Link) error {
	var b strings.Builder

	fmt.Fprintf(&b, "# Links for memo %d\n\n", memoID)

	fmt.Fprintln(&b, "## Outgoing")
	if len(outgoing) == 0 {
		fmt.Fprintln(&b, "\nNo outgoing links.")
	} else {
		fmt.Fprintln(&b, "")
		fmt.Fprintln(&b, "| Target | Type | Weight |")
		fmt.Fprintln(&b, "|--------|------|--------|")
		for _, l := range outgoing {
			fmt.Fprintf(&b, "| %d | %s | %.2f |\n", l.TargetID, l.RelationType, l.Weight)
		}
		fmt.Fprintln(&b)
	}

	fmt.Fprintln(&b, "## Incoming")
	if len(incoming) == 0 {
		fmt.Fprintln(&b, "\nNo incoming links.")
	} else {
		fmt.Fprintln(&b, "")
		fmt.Fprintln(&b, "| Source | Type | Weight |")
		fmt.Fprintln(&b, "|--------|------|--------|")
		for _, l := range incoming {
			fmt.Fprintf(&b, "| %d | %s | %.2f |\n", l.SourceID, l.RelationType, l.Weight)
		}
		fmt.Fprintln(&b)
	}

	_, err := fmt.Fprint(os.Stdout, b.String())
	return err
}

// printLinkBFSMD renders BFS traversal results as a markdown table.
func printLinkBFSMD(memoID int64, links []store.LinkWithDepth) error {
	var b strings.Builder

	fmt.Fprintf(&b, "# Link Graph for memo %d\n\n", memoID)

	if len(links) == 0 {
		fmt.Fprintln(&b, "No links found.")
	} else {
		fmt.Fprintln(&b, "| Source | Target | Type | Weight | Depth |")
		fmt.Fprintln(&b, "|--------|--------|------|--------|-------|")
		for _, l := range links {
			fmt.Fprintf(&b, "| %d | %d | %s | %.2f | %d |\n", l.SourceID, l.TargetID, l.RelationType, l.Weight, l.Depth)
		}
		fmt.Fprintln(&b)
	}

	_, err := fmt.Fprint(os.Stdout, b.String())
	return err
}

func init() {
	linkAddCmd.Flags().String("type", "related", "relation type (e.g. related, supports, contradicts, extends, causes, example-of)")
	linkAddCmd.Flags().Float64("weight", 0.5, "link weight between 0.0 and 1.0")

	linkRemoveCmd.Flags().String("type", "", "remove only links of this relation type (default: related)")

	linkListCmd.Flags().String("type", "", "filter links by relation type")
	linkListCmd.Flags().Bool("sort-weight", false, "sort links by weight descending")
	linkListCmd.Flags().Int("depth", 1, "BFS traversal depth")

	linkCmd.AddCommand(linkAddCmd)
	linkCmd.AddCommand(linkRemoveCmd)
	linkCmd.AddCommand(linkListCmd)

	rootCmd.AddCommand(linkCmd)
}
