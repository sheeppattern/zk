package cmd

import (
	"fmt"
	"sort"
	"strings"

	"github.com/spf13/cobra"
	"github.com/sheeppattern/zk/internal/model"
	"github.com/sheeppattern/zk/internal/store"
)

var searchCmd = &cobra.Command{
	Use:   "search <query>",
	Short: "Search notes by query",
	Long:  "Search notes by matching a query string against title, content, and tags. Supports filtering by tags, relation type, link weight, and status.",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		query := strings.ToLower(args[0])

		tags, _ := cmd.Flags().GetStringSlice("tags")
		relation, _ := cmd.Flags().GetString("relation")
		minWeight, _ := cmd.Flags().GetFloat64("min-weight")
		status, _ := cmd.Flags().GetString("status")
		sortFlag, _ := cmd.Flags().GetString("sort")

		s := store.NewStore(getStorePath(cmd))
		notes, err := s.ListNotes(flagProject)
		if err != nil {
			return fmt.Errorf("search notes: %w", err)
		}

		type scoredNote struct {
			note  *model.Note
			score int
		}

		var results []scoredNote

		for _, n := range notes {
			// Filter by query: case-insensitive substring match in title, content, or any tag.
			titleLower := strings.ToLower(n.Title)
			contentLower := strings.ToLower(n.Content)

			score := 0
			if strings.Contains(titleLower, query) {
				score += 2
			}
			if strings.Contains(contentLower, query) {
				score++
			}
			for _, tag := range n.Tags {
				if strings.Contains(strings.ToLower(tag), query) {
					score++
				}
			}

			if score == 0 {
				continue
			}

			// Filter by --tags (AND logic: note must have ALL specified tags).
			if len(tags) > 0 {
				hasAll := true
				for _, requiredTag := range tags {
					found := false
					for _, noteTag := range n.Tags {
						if strings.EqualFold(noteTag, requiredTag) {
							found = true
							break
						}
					}
					if !found {
						hasAll = false
						break
					}
				}
				if !hasAll {
					continue
				}
			}

			// Filter by --status.
			if status != "" && !strings.EqualFold(n.Metadata.Status, status) {
				continue
			}

			// Filter by --relation: only include notes that have outgoing links with the specified relation type.
			if relation != "" {
				hasRelation := false
				for _, link := range n.Links {
					if strings.EqualFold(link.RelationType, relation) {
						hasRelation = true
						break
					}
				}
				if !hasRelation {
					continue
				}
			}

			// Filter by --min-weight: only include notes that have at least one link with weight >= min-weight.
			if minWeight > 0 {
				hasWeight := false
				for _, link := range n.Links {
					if link.Weight >= minWeight {
						hasWeight = true
						break
					}
				}
				if !hasWeight {
					continue
				}
			}

			results = append(results, scoredNote{note: n, score: score})
		}

		// Sort results.
		switch sortFlag {
		case "created":
			sort.Slice(results, func(i, j int) bool {
				return results[i].note.Metadata.CreatedAt.After(results[j].note.Metadata.CreatedAt)
			})
		case "updated":
			sort.Slice(results, func(i, j int) bool {
				return results[i].note.Metadata.UpdatedAt.After(results[j].note.Metadata.UpdatedAt)
			})
		default: // "relevance"
			sort.Slice(results, func(i, j int) bool {
				return results[i].score > results[j].score
			})
		}

		// Collect notes for output.
		matched := make([]*model.Note, len(results))
		for i, r := range results {
			matched[i] = r.note
		}

		return getFormatter().PrintNotes(matched)
	},
}

func init() {
	searchCmd.Flags().StringSlice("tags", nil, "filter by tags (AND logic: note must have ALL specified tags)")
	searchCmd.Flags().String("relation", "", "filter notes that have a link with this relation type")
	searchCmd.Flags().Float64("min-weight", 0.0, "filter links with weight >= this value")
	searchCmd.Flags().String("status", "", "filter by status (active/archived)")
	searchCmd.Flags().String("sort", "relevance", "sort results: relevance, created, updated")

	rootCmd.AddCommand(searchCmd)
}
