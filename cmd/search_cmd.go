package cmd

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/sheeppattern/zk/internal/model"
	"github.com/sheeppattern/zk/internal/store"
)

var searchCmd = &cobra.Command{
	Use:   "search <query>",
	Short: "Search notes by query",
	Long: "Search notes by matching a query string against title, content, and tags. Supports filtering by tags, relation type, link weight, and status.",
	Example: `  zk search "Redis" --project P-XXXXXX
  zk search "auth" --tags "security" --relation contradicts --min-weight 0.5
  zk search "data" --created-after 2026-01-01 --sort created`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		query := strings.ToLower(args[0])

		tags, _ := cmd.Flags().GetStringSlice("tags")
		relation, _ := cmd.Flags().GetString("relation")
		minWeight, _ := cmd.Flags().GetFloat64("min-weight")
		status, _ := cmd.Flags().GetString("status")
		sortFlag, _ := cmd.Flags().GetString("sort")
		createdAfterStr, _ := cmd.Flags().GetString("created-after")
		createdBeforeStr, _ := cmd.Flags().GetString("created-before")
		layer, _ := cmd.Flags().GetString("layer")
		authorFilter, _ := cmd.Flags().GetString("author")

		var createdAfter, createdBefore time.Time
		if createdAfterStr != "" {
			var err error
			createdAfter, err = time.Parse("2006-01-02", createdAfterStr)
			if err != nil {
				return fmt.Errorf("invalid --created-after date %q: expected format YYYY-MM-DD", createdAfterStr)
			}
		}
		if createdBeforeStr != "" {
			var err error
			createdBefore, err = time.Parse("2006-01-02", createdBeforeStr)
			if err != nil {
				return fmt.Errorf("invalid --created-before date %q: expected format YYYY-MM-DD", createdBeforeStr)
			}
		}

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

			// Filter by --created-after.
			if !createdAfter.IsZero() && n.Metadata.CreatedAt.Before(createdAfter) {
				continue
			}

			// Filter by --created-before.
			if !createdBefore.IsZero() && n.Metadata.CreatedAt.After(createdBefore) {
				continue
			}

			// Filter by --layer.
			if layer != "" && n.Layer != layer {
				continue
			}

			// Filter by --author.
			if authorFilter != "" && !strings.EqualFold(n.Metadata.Author, authorFilter) {
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
	searchCmd.Flags().String("created-after", "", "filter notes created on or after this date (YYYY-MM-DD)")
	searchCmd.Flags().String("created-before", "", "filter notes created on or before this date (YYYY-MM-DD)")
	searchCmd.Flags().String("layer", "", "filter by layer (concrete, abstract)")
	searchCmd.Flags().String("author", "", "filter by note author")

	rootCmd.AddCommand(searchCmd)
}
