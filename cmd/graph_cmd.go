package cmd

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/spf13/cobra"
	"github.com/sheeppattern/zk/internal/model"
)

var (
	flagFormatGraph string
	flagGraphLayer  string
	flagGraphType   string
)

var graphCmd = &cobra.Command{
	Use:   "graph",
	Short: "Generate a Mermaid or DOT graph of memo connections",
	Long:  "Visualize the link structure between memos as a Mermaid or DOT graph output to stdout.",
	Example: `  zk graph
  zk graph --format-graph dot
  zk graph --layer abstract
  zk graph --type supports --format-graph mermaid`,
	RunE: runGraph,
}

func init() {
	graphCmd.Flags().StringVar(&flagFormatGraph, "format-graph", "mermaid", `graph output format: "mermaid" or "dot"`)
	graphCmd.Flags().StringVar(&flagGraphLayer, "layer", "", "filter memos by layer (concrete, abstract)")
	graphCmd.Flags().StringVar(&flagGraphType, "type", "", "filter edges by link relation type")
	rootCmd.AddCommand(graphCmd)
}

func runGraph(cmd *cobra.Command, args []string) error {
	if flagFormatGraph != "mermaid" && flagFormatGraph != "dot" {
		return fmt.Errorf("unsupported graph format %q: use \"mermaid\" or \"dot\"", flagFormatGraph)
	}

	s, err := openStore(cmd)
	if err != nil {
		return err
	}
	defer s.Close()

	memos, err := s.ListAllMemos()
	if err != nil {
		return fmt.Errorf("list memos: %w", err)
	}

	// Filter by layer if specified.
	if flagGraphLayer != "" {
		var filtered []*model.Memo
		for _, m := range memos {
			if m.Layer == flagGraphLayer {
				filtered = append(filtered, m)
			}
		}
		memos = filtered
	}

	// Build memo set for ID lookup.
	memoSet := make(map[int64]bool, len(memos))
	for _, m := range memos {
		memoSet[m.ID] = true
	}

	// Collect edges with deduplication using sorted pair key.
	type edge struct {
		From     int64
		To       int64
		Relation string
		Weight   float64
	}
	seen := make(map[string]bool)
	var edges []edge

	for _, m := range memos {
		outgoing, _, _ := s.ListLinks(m.ID)
		for _, lnk := range outgoing {
			if !memoSet[lnk.TargetID] {
				continue
			}
			if flagGraphType != "" && lnk.RelationType != flagGraphType {
				continue
			}
			pair := [2]int64{m.ID, lnk.TargetID}
			if pair[0] > pair[1] {
				pair[0], pair[1] = pair[1], pair[0]
			}
			key := strconv.FormatInt(pair[0], 10) + "|" + strconv.FormatInt(pair[1], 10)
			if seen[key] {
				continue
			}
			seen[key] = true
			edges = append(edges, edge{
				From:     m.ID,
				To:       lnk.TargetID,
				Relation: lnk.RelationType,
				Weight:   lnk.Weight,
			})
		}
	}

	// Sort memos by ID for deterministic output.
	sortedMemos := make([]*model.Memo, len(memos))
	copy(sortedMemos, memos)
	sort.Slice(sortedMemos, func(i, j int) bool {
		return sortedMemos[i].ID < sortedMemos[j].ID
	})

	var b strings.Builder

	switch flagFormatGraph {
	case "mermaid":
		b.WriteString("graph LR\n")
		for _, m := range sortedMemos {
			title := truncateTitle(m.Title, 30)
			label := fmt.Sprintf("%s (%s)", title, m.Layer)
			nodeID := strconv.FormatInt(m.ID, 10)
			if m.Layer == model.LayerAbstract {
				b.WriteString(fmt.Sprintf("  %s{{\"%s\"}}\n", nodeID, label))
			} else {
				b.WriteString(fmt.Sprintf("  %s[\"%s\"]\n", nodeID, label))
			}
		}
		for _, e := range edges {
			b.WriteString(fmt.Sprintf("  %d -->|\"%s (%.1f)\"| %d\n", e.From, e.Relation, e.Weight, e.To))
		}
		for _, m := range sortedMemos {
			if m.Layer == model.LayerAbstract {
				b.WriteString(fmt.Sprintf("  style %d fill:#fde8e8\n", m.ID))
			}
		}

	case "dot":
		b.WriteString("digraph zk {\n")
		b.WriteString("  rankdir=LR;\n")
		b.WriteString("  node [shape=box, style=filled];\n")
		for _, m := range sortedMemos {
			title := truncateTitle(m.Title, 30)
			label := fmt.Sprintf("%s (%s)", title, m.Layer)
			fillColor := "#e8f4fd"
			if m.Layer == model.LayerAbstract {
				fillColor = "#fde8e8"
			}
			b.WriteString(fmt.Sprintf("  \"%d\" [label=\"%s\", fillcolor=\"%s\"];\n", m.ID, label, fillColor))
		}
		for _, e := range edges {
			edgeColor := "black"
			switch e.Relation {
			case model.RelContradicts:
				edgeColor = "red"
			case model.RelSupports:
				edgeColor = "green"
			}
			b.WriteString(fmt.Sprintf("  \"%d\" -> \"%d\" [label=\"%s (%.1f)\", color=%s];\n", e.From, e.To, e.Relation, e.Weight, edgeColor))
		}
		b.WriteString("}\n")
	}

	fmt.Print(b.String())
	statusf("graph: %d nodes, %d edges", len(memos), len(edges))
	return nil
}

// truncateTitle shortens a string to max runes, appending "..." if truncated.
func truncateTitle(s string, max int) string {
	if utf8.RuneCountInString(s) <= max {
		return s
	}
	runes := []rune(s)
	return string(runes[:max]) + "..."
}
