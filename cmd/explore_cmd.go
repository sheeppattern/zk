package cmd

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/spf13/cobra"
	"github.com/sheeppattern/nete/internal/model"
	"github.com/sheeppattern/nete/internal/store"
)

// ExploreNode represents a memo in the exploration graph.
type ExploreNode struct {
	ID      int64    `json:"id" yaml:"id"`
	Title   string   `json:"title" yaml:"title"`
	Layer   string   `json:"layer" yaml:"layer"`
	Tags    []string `json:"tags" yaml:"tags"`
	Content string   `json:"content,omitempty" yaml:"content,omitempty"`
	Summary string   `json:"summary,omitempty" yaml:"summary,omitempty"`
}

// ExploreEdge represents a directional link in the exploration graph.
type ExploreEdge struct {
	MemoID       int64   `json:"memo_id" yaml:"memo_id"`
	MemoTitle    string  `json:"memo_title" yaml:"memo_title"`
	MemoLayer    string  `json:"memo_layer" yaml:"memo_layer"`
	MemoSummary  string  `json:"memo_summary,omitempty" yaml:"memo_summary,omitempty"`
	RelationType string  `json:"relation_type" yaml:"relation_type"`
	Weight       float64 `json:"weight" yaml:"weight"`
	Direction    string  `json:"direction" yaml:"direction"`
}

// ExploreResult is the structured navigation context for a memo.
type ExploreResult struct {
	Current   ExploreNode   `json:"current" yaml:"current"`
	Outgoing  []ExploreEdge `json:"outgoing" yaml:"outgoing"`
	Incoming  []ExploreEdge `json:"incoming" yaml:"incoming"`
	Neighbors []ExploreNode `json:"neighbors,omitempty" yaml:"neighbors,omitempty"`
}

var exploreCmd = &cobra.Command{
	Use:   "explore <memoID>",
	Short: "Output structured navigation context for a memo",
	Long:  "Explore the link neighborhood of a memo, showing outgoing links, incoming backlinks, and optionally deeper neighbors via BFS.",
	Example: `  nete explore 1
  nete explore 1 --depth 2 --include-content --format json
  nete explore 1 --depth 3 --format md`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		memoID, err := strconv.ParseInt(args[0], 10, 64)
		if err != nil {
			return fmt.Errorf("invalid memo ID %q: %w", args[0], err)
		}
		depth, _ := cmd.Flags().GetInt("depth")
		includeContent, _ := cmd.Flags().GetBool("include-content")

		s, err := openStore(cmd)
		if err != nil {
			return err
		}
		defer s.Close()

		// Load starting memo.
		startMemo, err := s.GetMemo(memoID)
		if err != nil {
			return fmt.Errorf("memo %d not found: %w", memoID, err)
		}

		// Build memo map for neighbor lookup.
		allMemos, _ := s.ListAllMemos()
		memoMap := make(map[int64]*model.Memo, len(allMemos))
		for _, m := range allMemos {
			memoMap[m.ID] = m
		}

		// Build Current node.
		result := ExploreResult{}
		result.Current = makeExploreNodeFromMemo(startMemo, includeContent)

		// Outgoing and incoming links from the store.
		outgoing, incoming, _ := s.ListLinks(memoID)

		result.Outgoing = []ExploreEdge{}
		for _, link := range outgoing {
			edge := ExploreEdge{
				MemoID:       link.TargetID,
				RelationType: link.RelationType,
				Weight:       link.Weight,
				Direction:    "outgoing",
			}
			if target, ok := memoMap[link.TargetID]; ok {
				edge.MemoTitle = target.Title
				edge.MemoLayer = target.Layer
				edge.MemoSummary = target.Metadata.Summary
			}
			result.Outgoing = append(result.Outgoing, edge)
		}

		result.Incoming = []ExploreEdge{}
		for _, link := range incoming {
			edge := ExploreEdge{
				MemoID:       link.SourceID,
				RelationType: link.RelationType,
				Weight:       link.Weight,
				Direction:    "incoming",
			}
			if source, ok := memoMap[link.SourceID]; ok {
				edge.MemoTitle = source.Title
				edge.MemoLayer = source.Layer
				edge.MemoSummary = source.Metadata.Summary
			}
			result.Incoming = append(result.Incoming, edge)
		}

		// BFS for neighbors at depth > 1.
		result.Neighbors = []ExploreNode{}
		if depth > 1 {
			result.Neighbors = bfsNeighborsMemo(s, memoID, memoMap, depth, includeContent)
		}

		// Output.
		f := getFormatter()
		switch f.Format {
		case "json":
			return f.PrintJSON(result)
		case "yaml":
			return f.PrintYAML(result)
		case "md":
			return printExploreMD(result)
		default:
			return fmt.Errorf("unsupported format: %s", f.Format)
		}
	},
}

// makeExploreNodeFromMemo creates an ExploreNode from a Memo.
func makeExploreNodeFromMemo(m *model.Memo, includeContent bool) ExploreNode {
	tags := m.Tags
	if tags == nil {
		tags = []string{}
	}
	node := ExploreNode{
		ID:      m.ID,
		Title:   m.Title,
		Layer:   m.Layer,
		Tags:    tags,
		Summary: m.Metadata.Summary,
	}
	if includeContent {
		node.Content = m.Content
	}
	return node
}

// bfsNeighborsMemo performs BFS from startID up to maxDepth using the store's link API.
func bfsNeighborsMemo(s *store.Store, startID int64, memoMap map[int64]*model.Memo, maxDepth int, includeContent bool) []ExploreNode {
	bfsLinks, err := s.ListLinksBFS(startID, maxDepth)
	if err != nil {
		return []ExploreNode{}
	}

	// Collect unique neighbor IDs at depth >= 2.
	seen := map[int64]bool{}
	var neighbors []ExploreNode
	for _, l := range bfsLinks {
		if l.Depth < 2 {
			continue
		}
		// Determine which node is the neighbor.
		neighborID := l.TargetID
		if neighborID == startID {
			neighborID = l.SourceID
		}
		if seen[neighborID] {
			continue
		}
		seen[neighborID] = true
		if m, ok := memoMap[neighborID]; ok {
			neighbors = append(neighbors, makeExploreNodeFromMemo(m, includeContent))
		}
	}

	if neighbors == nil {
		neighbors = []ExploreNode{}
	}
	return neighbors
}

// printExploreMD renders the ExploreResult in a custom Markdown format.
func printExploreMD(result ExploreResult) error {
	var b strings.Builder

	fmt.Fprintf(&b, "# Exploring: %s (%d)\n\n", result.Current.Title, result.Current.ID)
	fmt.Fprintf(&b, "**Layer**: %s\n", result.Current.Layer)
	fmt.Fprintf(&b, "**Tags**: %s\n", strings.Join(result.Current.Tags, ", "))

	if result.Current.Content != "" {
		fmt.Fprintf(&b, "\n%s\n", result.Current.Content)
	}

	fmt.Fprintf(&b, "\n## Outgoing Links\n\n")
	if len(result.Outgoing) == 0 {
		fmt.Fprintln(&b, "_No outgoing links._")
	} else {
		for _, e := range result.Outgoing {
			if e.MemoSummary != "" {
				fmt.Fprintf(&b, "- %d → **%s** (%d) — %s [%s, weight: %.2f]\n",
					result.Current.ID, e.MemoTitle, e.MemoID, e.MemoSummary, e.RelationType, e.Weight)
			} else {
				fmt.Fprintf(&b, "- %d → **%s** (%d) [%s, weight: %.2f]\n",
					result.Current.ID, e.MemoTitle, e.MemoID, e.RelationType, e.Weight)
			}
		}
	}

	fmt.Fprintf(&b, "\n## Incoming Links\n\n")
	if len(result.Incoming) == 0 {
		fmt.Fprintln(&b, "_No incoming links._")
	} else {
		for _, e := range result.Incoming {
			if e.MemoSummary != "" {
				fmt.Fprintf(&b, "- **%s** (%d) — %s → %d [%s, weight: %.2f]\n",
					e.MemoTitle, e.MemoID, e.MemoSummary, result.Current.ID, e.RelationType, e.Weight)
			} else {
				fmt.Fprintf(&b, "- **%s** (%d) → %d [%s, weight: %.2f]\n",
					e.MemoTitle, e.MemoID, result.Current.ID, e.RelationType, e.Weight)
			}
		}
	}

	if len(result.Neighbors) > 0 {
		fmt.Fprintf(&b, "\n## Neighbors (depth > 1)\n\n")
		for _, n := range result.Neighbors {
			fmt.Fprintf(&b, "- **%s** (%d) — layer: %s, tags: %s\n",
				n.Title, n.ID, n.Layer, strings.Join(n.Tags, ", "))
		}
	}

	// Navigation hints.
	fmt.Fprintf(&b, "\n---\n\n")
	fmt.Fprintln(&b, "**Navigation hints:**")
	connectedIDs := map[int64]bool{}
	for _, e := range result.Outgoing {
		connectedIDs[e.MemoID] = true
	}
	for _, e := range result.Incoming {
		connectedIDs[e.MemoID] = true
	}
	for _, n := range result.Neighbors {
		connectedIDs[n.ID] = true
	}
	if len(connectedIDs) == 0 {
		fmt.Fprintln(&b, "_No connected memos to explore._")
	} else {
		for id := range connectedIDs {
			fmt.Fprintf(&b, "- `nete explore %d`\n", id)
		}
	}

	fmt.Fprintln(&b)
	_, err := fmt.Fprint(os.Stdout, b.String())
	return err
}

func init() {
	exploreCmd.Flags().Int("depth", 1, "BFS traversal depth for neighbors")
	exploreCmd.Flags().Bool("include-content", false, "include memo content in output")
	rootCmd.AddCommand(exploreCmd)
}
