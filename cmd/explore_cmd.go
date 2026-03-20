package cmd

import (
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"github.com/sheeppattern/zk/internal/model"
	"github.com/sheeppattern/zk/internal/store"
)

// ExploreNode represents a note in the exploration graph.
type ExploreNode struct {
	ID      string   `json:"id" yaml:"id"`
	Title   string   `json:"title" yaml:"title"`
	Layer   string   `json:"layer" yaml:"layer"`
	Tags    []string `json:"tags" yaml:"tags"`
	Content string   `json:"content,omitempty" yaml:"content,omitempty"`
}

// ExploreEdge represents a directional link in the exploration graph.
type ExploreEdge struct {
	NoteID       string  `json:"note_id" yaml:"note_id"`
	NoteTitle    string  `json:"note_title" yaml:"note_title"`
	NoteLayer    string  `json:"note_layer" yaml:"note_layer"`
	RelationType string  `json:"relation_type" yaml:"relation_type"`
	Weight       float64 `json:"weight" yaml:"weight"`
	Direction    string  `json:"direction" yaml:"direction"`
}

// ExploreResult is the structured navigation context for a note.
type ExploreResult struct {
	Current   ExploreNode   `json:"current" yaml:"current"`
	Outgoing  []ExploreEdge `json:"outgoing" yaml:"outgoing"`
	Incoming  []ExploreEdge `json:"incoming" yaml:"incoming"`
	Neighbors []ExploreNode `json:"neighbors,omitempty" yaml:"neighbors,omitempty"`
}

var exploreCmd = &cobra.Command{
	Use:   "explore <noteID>",
	Short: "Output structured navigation context for a note",
	Long:  "Explore the link neighborhood of a note, showing outgoing links, incoming backlinks, and optionally deeper neighbors via BFS.",
	Example: `  zk explore N-AAAAAA --project P-XXXXXX
  zk explore N-AAAAAA --depth 2 --include-content --format json
  zk explore N-AAAAAA --depth 3 --format md`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		noteID := args[0]
		depth, _ := cmd.Flags().GetInt("depth")
		includeContent, _ := cmd.Flags().GetBool("include-content")

		storePath := getStorePath(cmd)
		s := store.NewStore(storePath)

		// Load starting note.
		startNote, err := s.GetNote(flagProject, noteID)
		if err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			os.Exit(1)
		}

		// Load all notes and build noteMap.
		allNotes, err := s.ListNotes(flagProject)
		if err != nil {
			fmt.Fprintf(os.Stderr, "error: listing notes: %v\n", err)
			os.Exit(1)
		}
		noteMap := make(map[string]*model.Note, len(allNotes))
		for _, n := range allNotes {
			noteMap[n.ID] = n
		}

		// Build Current node.
		result := ExploreResult{}
		result.Current = makeExploreNode(startNote, includeContent)

		// Outgoing: from note.Links.
		result.Outgoing = []ExploreEdge{}
		for _, link := range startNote.Links {
			edge := ExploreEdge{
				NoteID:       link.TargetID,
				RelationType: link.RelationType,
				Weight:       link.Weight,
				Direction:    "outgoing",
			}
			if target, ok := noteMap[link.TargetID]; ok {
				edge.NoteTitle = target.Title
				edge.NoteLayer = target.Layer
			}
			result.Outgoing = append(result.Outgoing, edge)
		}

		// Incoming: scan all notes for links targeting noteID.
		result.Incoming = []ExploreEdge{}
		for _, n := range allNotes {
			if n.ID == noteID {
				continue
			}
			for _, link := range n.Links {
				if link.TargetID == noteID {
					result.Incoming = append(result.Incoming, ExploreEdge{
						NoteID:       n.ID,
						NoteTitle:    n.Title,
						NoteLayer:    n.Layer,
						RelationType: link.RelationType,
						Weight:       link.Weight,
						Direction:    "incoming",
					})
				}
			}
		}

		// BFS for neighbors at depth > 1.
		result.Neighbors = []ExploreNode{}
		if depth > 1 {
			result.Neighbors = bfsNeighbors(noteID, noteMap, depth, includeContent)
		}

		// Output.
		f := getFormatter()
		switch f.Format {
		case "json":
			return f.PrintJSON(result)
		case "yaml":
			return f.PrintYAML(result)
		case "md":
			return printExploreMD(result, flagProject)
		default:
			return fmt.Errorf("unsupported format: %s", f.Format)
		}
	},
}

// makeExploreNode creates an ExploreNode from a Note.
func makeExploreNode(n *model.Note, includeContent bool) ExploreNode {
	tags := n.Tags
	if tags == nil {
		tags = []string{}
	}
	node := ExploreNode{
		ID:    n.ID,
		Title: n.Title,
		Layer: n.Layer,
		Tags:  tags,
	}
	if includeContent {
		node.Content = n.Content
	}
	return node
}

// bfsNeighbors performs BFS from startID up to maxDepth and returns unique
// neighbor nodes, excluding the start node and direct link targets (depth 1).
func bfsNeighbors(startID string, noteMap map[string]*model.Note, maxDepth int, includeContent bool) []ExploreNode {
	type bfsEntry struct {
		id    string
		depth int
	}

	visited := map[string]bool{startID: true}
	queue := []bfsEntry{}

	// Seed BFS with depth-1 neighbors (outgoing + incoming).
	if startNote, ok := noteMap[startID]; ok {
		for _, link := range startNote.Links {
			if !visited[link.TargetID] {
				visited[link.TargetID] = true
				queue = append(queue, bfsEntry{id: link.TargetID, depth: 1})
			}
		}
	}
	// Incoming at depth 1.
	for _, n := range noteMap {
		if n.ID == startID {
			continue
		}
		for _, link := range n.Links {
			if link.TargetID == startID && !visited[n.ID] {
				visited[n.ID] = true
				queue = append(queue, bfsEntry{id: n.ID, depth: 1})
			}
		}
	}

	// BFS from depth-1 nodes outward.
	var neighbors []ExploreNode
	for i := 0; i < len(queue); i++ {
		entry := queue[i]
		// Only collect nodes at depth >= 2 as neighbors.
		if entry.depth >= 2 {
			if n, ok := noteMap[entry.id]; ok {
				neighbors = append(neighbors, makeExploreNode(n, includeContent))
			}
		}
		// Expand if not at max depth.
		if entry.depth < maxDepth {
			if n, ok := noteMap[entry.id]; ok {
				for _, link := range n.Links {
					if !visited[link.TargetID] {
						visited[link.TargetID] = true
						queue = append(queue, bfsEntry{id: link.TargetID, depth: entry.depth + 1})
					}
				}
				// Also check incoming links for this node.
				for _, other := range noteMap {
					if other.ID == entry.id {
						continue
					}
					for _, link := range other.Links {
						if link.TargetID == entry.id && !visited[other.ID] {
							visited[other.ID] = true
							queue = append(queue, bfsEntry{id: other.ID, depth: entry.depth + 1})
						}
					}
				}
			}
		}
	}

	if neighbors == nil {
		neighbors = []ExploreNode{}
	}
	return neighbors
}

// printExploreMD renders the ExploreResult in a custom Markdown format.
func printExploreMD(result ExploreResult, projectID string) error {
	var b strings.Builder

	fmt.Fprintf(&b, "# Exploring: %s (%s)\n\n", result.Current.Title, result.Current.ID)
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
			fmt.Fprintf(&b, "- %s → **%s** (%s) [%s, weight: %.2f]\n",
				result.Current.ID, e.NoteTitle, e.NoteID, e.RelationType, e.Weight)
		}
	}

	fmt.Fprintf(&b, "\n## Incoming Links\n\n")
	if len(result.Incoming) == 0 {
		fmt.Fprintln(&b, "_No incoming links._")
	} else {
		for _, e := range result.Incoming {
			fmt.Fprintf(&b, "- **%s** (%s) → %s [%s, weight: %.2f]\n",
				e.NoteTitle, e.NoteID, result.Current.ID, e.RelationType, e.Weight)
		}
	}

	if len(result.Neighbors) > 0 {
		fmt.Fprintf(&b, "\n## Neighbors (depth > 1)\n\n")
		for _, n := range result.Neighbors {
			fmt.Fprintf(&b, "- **%s** (%s) — layer: %s, tags: %s\n",
				n.Title, n.ID, n.Layer, strings.Join(n.Tags, ", "))
		}
	}

	// Navigation hints.
	fmt.Fprintf(&b, "\n---\n\n")
	fmt.Fprintln(&b, "**Navigation hints:**")
	connectedIDs := map[string]bool{}
	for _, e := range result.Outgoing {
		connectedIDs[e.NoteID] = true
	}
	for _, e := range result.Incoming {
		connectedIDs[e.NoteID] = true
	}
	for _, n := range result.Neighbors {
		connectedIDs[n.ID] = true
	}
	if len(connectedIDs) == 0 {
		fmt.Fprintln(&b, "_No connected notes to explore._")
	} else {
		for id := range connectedIDs {
			if projectID != "" {
				fmt.Fprintf(&b, "- `zk explore %s --project %s`\n", id, projectID)
			} else {
				fmt.Fprintf(&b, "- `zk explore %s`\n", id)
			}
		}
	}

	fmt.Fprintln(&b)
	_, err := fmt.Fprint(os.Stdout, b.String())
	return err
}

func init() {
	exploreCmd.Flags().Int("depth", 1, "BFS traversal depth for neighbors")
	exploreCmd.Flags().Bool("include-content", false, "include note content in output")
	rootCmd.AddCommand(exploreCmd)
}
