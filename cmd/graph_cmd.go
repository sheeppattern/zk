package cmd

import (
	"fmt"
	"sort"
	"strings"
	"unicode/utf8"

	"github.com/spf13/cobra"
	"github.com/sheeppattern/zk/internal/model"
	"github.com/sheeppattern/zk/internal/store"
)

var (
	flagFormatGraph string
	flagGraphLayer  string
	flagGraphType   string
)

var graphCmd = &cobra.Command{
	Use:   "graph",
	Short: "Generate a Mermaid or DOT graph of note connections",
	Long:  "Visualize the link structure between notes as a Mermaid or DOT graph output to stdout.",
	Example: `  zk graph
  zk graph --format-graph dot
  zk graph --project P-ABC123 --layer abstract
  zk graph --type supports --format-graph mermaid`,
	RunE: runGraph,
}

func init() {
	graphCmd.Flags().StringVar(&flagFormatGraph, "format-graph", "mermaid", `graph output format: "mermaid" or "dot"`)
	graphCmd.Flags().StringVar(&flagGraphLayer, "layer", "", "filter notes by layer (concrete, abstract)")
	graphCmd.Flags().StringVar(&flagGraphType, "type", "", "filter edges by link relation type")
	rootCmd.AddCommand(graphCmd)
}

func runGraph(cmd *cobra.Command, args []string) error {
	if flagFormatGraph != "mermaid" && flagFormatGraph != "dot" {
		return fmt.Errorf("unsupported graph format %q: use \"mermaid\" or \"dot\"", flagFormatGraph)
	}

	s := store.NewStore(getStorePath(cmd))

	notes, err := s.ListNotes(flagProject)
	if err != nil {
		return fmt.Errorf("list notes: %w", err)
	}

	// Filter by layer if specified.
	if flagGraphLayer != "" {
		var filtered []*model.Note
		for _, n := range notes {
			if n.Layer == flagGraphLayer {
				filtered = append(filtered, n)
			}
		}
		notes = filtered
	}

	// Build noteMap for ID lookup.
	noteMap := make(map[string]*model.Note, len(notes))
	for _, n := range notes {
		noteMap[n.ID] = n
	}

	// Collect edges with deduplication using sorted pair key.
	type edge struct {
		From     string
		To       string
		Relation string
		Weight   float64
	}
	seen := make(map[string]bool)
	var edges []edge

	for _, n := range notes {
		for _, lnk := range n.Links {
			// Skip if target is not in the note set.
			if _, ok := noteMap[lnk.TargetID]; !ok {
				continue
			}
			// Filter by relation type if specified.
			if flagGraphType != "" && lnk.RelationType != flagGraphType {
				continue
			}
			// Dedup with sorted pair key.
			pair := [2]string{n.ID, lnk.TargetID}
			if pair[0] > pair[1] {
				pair[0], pair[1] = pair[1], pair[0]
			}
			key := pair[0] + "|" + pair[1]
			if seen[key] {
				continue
			}
			seen[key] = true
			edges = append(edges, edge{
				From:     n.ID,
				To:       lnk.TargetID,
				Relation: lnk.RelationType,
				Weight:   lnk.Weight,
			})
		}
	}

	// Sort notes by ID for deterministic output.
	sortedNotes := make([]*model.Note, len(notes))
	copy(sortedNotes, notes)
	sort.Slice(sortedNotes, func(i, j int) bool {
		return sortedNotes[i].ID < sortedNotes[j].ID
	})

	var b strings.Builder

	switch flagFormatGraph {
	case "mermaid":
		b.WriteString("graph LR\n")
		// Node declarations.
		for _, n := range sortedNotes {
			title := truncateTitle(n.Title, 30)
			label := fmt.Sprintf("%s (%s)", title, n.Layer)
			nodeID := strings.ReplaceAll(n.ID, "-", "-")
			if n.Layer == model.LayerAbstract {
				b.WriteString(fmt.Sprintf("  %s{{\"%s\"}}\n", nodeID, label))
			} else {
				b.WriteString(fmt.Sprintf("  %s[\"%s\"]\n", nodeID, label))
			}
		}
		// Edges.
		for _, e := range edges {
			fromID := strings.ReplaceAll(e.From, "-", "-")
			toID := strings.ReplaceAll(e.To, "-", "-")
			b.WriteString(fmt.Sprintf("  %s -->|\"%s (%.1f)\"| %s\n", fromID, e.Relation, e.Weight, toID))
		}
		// Style abstract nodes.
		for _, n := range sortedNotes {
			if n.Layer == model.LayerAbstract {
				nodeID := strings.ReplaceAll(n.ID, "-", "-")
				b.WriteString(fmt.Sprintf("  style %s fill:#fde8e8\n", nodeID))
			}
		}

	case "dot":
		b.WriteString("digraph zk {\n")
		b.WriteString("  rankdir=LR;\n")
		b.WriteString("  node [shape=box, style=filled];\n")
		// Node declarations.
		for _, n := range sortedNotes {
			title := truncateTitle(n.Title, 30)
			label := fmt.Sprintf("%s (%s)", title, n.Layer)
			fillColor := "#e8f4fd"
			if n.Layer == model.LayerAbstract {
				fillColor = "#fde8e8"
			}
			b.WriteString(fmt.Sprintf("  \"%s\" [label=\"%s\", fillcolor=\"%s\"];\n", n.ID, label, fillColor))
		}
		// Edges.
		for _, e := range edges {
			edgeColor := "black"
			switch e.Relation {
			case model.RelContradicts:
				edgeColor = "red"
			case model.RelSupports:
				edgeColor = "green"
			}
			b.WriteString(fmt.Sprintf("  \"%s\" -> \"%s\" [label=\"%s (%.1f)\", color=%s];\n", e.From, e.To, e.Relation, e.Weight, edgeColor))
		}
		b.WriteString("}\n")
	}

	fmt.Print(b.String())
	statusf("graph: %d nodes, %d edges", len(notes), len(edges))
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
