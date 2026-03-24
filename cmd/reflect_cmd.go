package cmd

import (
	"fmt"
	"os"
	"strings"
	"unicode/utf8"

	"github.com/spf13/cobra"
	"github.com/sheeppattern/zk/internal/model"
)

// ReflectReport holds the full reflection analysis results.
type ReflectReport struct {
	Insights []Insight    `json:"insights" yaml:"insights"`
	Stats    ReflectStats `json:"stats" yaml:"stats"`
}

// Insight represents a single reflection finding.
type Insight struct {
	Type           string  `json:"type" yaml:"type"` // "tension", "hub_without_abstraction", "orphan_cluster", "low_abstraction", "bloated_note", "suggested_link"
	SourceMemos    []int64 `json:"source_memos" yaml:"source_memos"`
	Suggestion     string  `json:"suggestion" yaml:"suggestion"`
	SuggestedTitle string  `json:"suggested_title,omitempty" yaml:"suggested_title,omitempty"`
}

// ReflectStats provides abstraction-level statistics.
type ReflectStats struct {
	ConcreteCount  int     `json:"concrete_count" yaml:"concrete_count"`
	AbstractCount  int     `json:"abstract_count" yaml:"abstract_count"`
	Ratio          float64 `json:"ratio" yaml:"ratio"`
	Recommendation string  `json:"recommendation" yaml:"recommendation"`
}

var flagApply bool
var flagSuggestLinks bool

var reflectCmd = &cobra.Command{
	Use:   "reflect",
	Short: "Analyze memos and suggest abstract insights",
	Long:  "Analyze memos to detect tensions, hubs without abstraction, and orphan clusters, then suggest abstract insight memos.",
	Example: `  zk reflect
  zk reflect --format md
  zk reflect --apply`,
	RunE: runReflect,
}

func init() {
	rootCmd.AddCommand(reflectCmd)
	reflectCmd.Flags().BoolVar(&flagApply, "apply", false, "auto-create suggested abstract memos")
	reflectCmd.Flags().BoolVar(&flagSuggestLinks, "suggest-links", false, "suggest missing links between similar memos")
}

func runReflect(cmd *cobra.Command, args []string) error {
	s, err := openStore(cmd)
	if err != nil {
		return err
	}
	defer s.Close()
	f := getFormatter()

	memos, err := s.ListAllMemos()
	if err != nil {
		return fmt.Errorf("list memos: %w", err)
	}

	// Build link maps from the store.
	outgoingMap := make(map[int64][]model.Link)
	incomingMap := make(map[int64][]model.Link)
	for _, m := range memos {
		out, in, lErr := s.ListLinks(m.ID)
		if lErr != nil {
			continue
		}
		outgoingMap[m.ID] = out
		incomingMap[m.ID] = in
	}

	report := buildReflectReport(memos, outgoingMap, incomingMap)

	// --suggest-links: analyze memo content for potential missing links.
	if flagSuggestLinks {
		suggestions := suggestLinks(memos, outgoingMap)
		report.Insights = append(report.Insights, suggestions...)
	}

	// --apply: create suggested abstract memos.
	if flagApply {
		created := 0
		for _, insight := range report.Insights {
			if insight.SuggestedTitle == "" {
				continue
			}
			newMemo := &model.Memo{
				Title:   insight.SuggestedTitle,
				Content: insight.Suggestion,
				Tags:    []string{"auto-reflect"},
				Layer:   model.LayerAbstract,
				NoteID:  flagNote,
				Metadata: model.Metadata{
					Status: model.StatusActive,
				},
			}
			if err := s.CreateMemo(newMemo); err != nil {
				return fmt.Errorf("create abstract memo: %w", err)
			}
			// Add "abstracts" links from each source memo to the new memo.
			for _, srcID := range insight.SourceMemos {
				if err := s.AddLink(srcID, newMemo.ID, model.RelAbstracts, 0.7); err != nil {
					statusf("warning: could not add link from %d to %d: %v", srcID, newMemo.ID, err)
				}
			}
			created++
		}
		statusf("created %d abstract memos", created)
	}

	switch f.Format {
	case "json":
		return f.PrintJSON(report)
	case "yaml":
		return f.PrintYAML(report)
	case "md":
		printReflectMD(report)
		return nil
	default:
		return fmt.Errorf("unsupported format: %s", f.Format)
	}
}

func buildReflectReport(memos []*model.Memo, outgoingMap map[int64][]model.Link, incomingMap map[int64][]model.Link) *ReflectReport {
	report := &ReflectReport{
		Insights: []Insight{},
	}

	// Separate concrete and abstract memos.
	memoByID := make(map[int64]*model.Memo)
	var concreteMemos []*model.Memo
	var abstractMemos []*model.Memo

	for _, m := range memos {
		memoByID[m.ID] = m
		switch m.Layer {
		case model.LayerAbstract:
			abstractMemos = append(abstractMemos, m)
		default:
			concreteMemos = append(concreteMemos, m)
		}
	}

	// 1. Detect tensions: pairs connected by "contradicts" without a shared abstract memo.
	type pair struct{ a, b int64 }
	seenPairs := make(map[pair]bool)

	for _, m := range memos {
		for _, link := range outgoingMap[m.ID] {
			if link.RelationType != model.RelContradicts {
				continue
			}
			p := pair{m.ID, link.TargetID}
			if p.a > p.b {
				p = pair{p.b, p.a}
			}
			if seenPairs[p] {
				continue
			}
			seenPairs[p] = true

			// Check if any abstract memo is connected to both.
			hasSharedAbstract := false
			for _, absMemo := range abstractMemos {
				connectsA := false
				connectsB := false
				for _, al := range outgoingMap[absMemo.ID] {
					if al.RelationType == model.RelAbstracts || al.RelationType == model.RelGrounds {
						if al.TargetID == p.a {
							connectsA = true
						}
						if al.TargetID == p.b {
							connectsB = true
						}
					}
				}
				for _, il := range incomingMap[absMemo.ID] {
					if il.RelationType == model.RelAbstracts || il.RelationType == model.RelGrounds {
						if il.SourceID == p.a {
							connectsA = true
						}
						if il.SourceID == p.b {
							connectsB = true
						}
					}
				}
				if connectsA && connectsB {
					hasSharedAbstract = true
					break
				}
			}

			if !hasSharedAbstract {
				titleA := fmt.Sprintf("%d", p.a)
				titleB := fmt.Sprintf("%d", p.b)
				if na, ok := memoByID[p.a]; ok {
					titleA = na.Title
				}
				if nb, ok := memoByID[p.b]; ok {
					titleB = nb.Title
				}
				report.Insights = append(report.Insights, Insight{
					Type:           "tension",
					SourceMemos:    []int64{p.a, p.b},
					Suggestion:     fmt.Sprintf("Tension between %d(%s) and %d(%s) without a synthesizing abstract memo", p.a, titleA, p.b, titleB),
					SuggestedTitle: fmt.Sprintf("%s vs %s", titleA, titleB),
				})
			}
		}
	}

	// 2. Detect hubs without abstraction: concrete memos with 4+ outgoing links.
	for _, m := range concreteMemos {
		links := outgoingMap[m.ID]
		if len(links) < 4 {
			continue
		}
		hasAbstractLink := false
		for _, link := range links {
			if target, ok := memoByID[link.TargetID]; ok && target.Layer == model.LayerAbstract {
				hasAbstractLink = true
				break
			}
		}
		if !hasAbstractLink {
			report.Insights = append(report.Insights, Insight{
				Type:        "hub_without_abstraction",
				SourceMemos: []int64{m.ID},
				Suggestion:  fmt.Sprintf("Memo %d(%s) has %d links but no abstract connection", m.ID, m.Title, len(links)),
			})
		}
	}

	// 3. Detect orphan clusters.
	for _, m := range concreteMemos {
		hasOutgoing := len(outgoingMap[m.ID]) > 0
		hasIncoming := len(incomingMap[m.ID]) > 0
		if !hasOutgoing && !hasIncoming {
			report.Insights = append(report.Insights, Insight{
				Type:        "orphan_cluster",
				SourceMemos: []int64{m.ID},
				Suggestion:  fmt.Sprintf("Memo %d(%s) is isolated — review connections", m.ID, m.Title),
			})
		}
	}

	// 4. Detect bloated memos.
	for _, m := range concreteMemos {
		if utf8.RuneCountInString(m.Content) > 1000 {
			report.Insights = append(report.Insights, Insight{
				Type:        "bloated_note",
				SourceMemos: []int64{m.ID},
				Suggestion:  fmt.Sprintf("Memo %d(%s) is %d chars — consider splitting", m.ID, m.Title, utf8.RuneCountInString(m.Content)),
			})
		}
	}

	// 5. Calculate stats.
	concreteCount := len(concreteMemos)
	abstractCount := len(abstractMemos)
	total := concreteCount + abstractCount
	var ratio float64
	if total > 0 {
		ratio = float64(abstractCount) / float64(total)
	}

	var recommendation string
	switch {
	case ratio < 0.1:
		recommendation = "Low insight ratio — derive patterns and questions from concrete memos"
	case ratio <= 0.3:
		recommendation = "Good — consider adding abstractions for hubs and tensions"
	default:
		recommendation = "Sufficient abstraction level"
	}

	report.Stats = ReflectStats{
		ConcreteCount:  concreteCount,
		AbstractCount:  abstractCount,
		Ratio:          ratio,
		Recommendation: recommendation,
	}

	return report
}

// charTrigrams extracts a set of character trigrams from text.
func charTrigrams(text string) map[string]bool {
	runes := []rune(text)
	set := make(map[string]bool)
	for i := 0; i+3 <= len(runes); i++ {
		tri := string(runes[i : i+3])
		hasContent := false
		for _, r := range tri {
			if r > ' ' && r != '.' && r != ',' && r != '!' && r != '?' {
				hasContent = true
				break
			}
		}
		if hasContent {
			set[tri] = true
		}
	}
	return set
}

// suggestLinks analyzes memo content to find pairs that share significant
// keyword overlap but have no existing link.
func suggestLinks(memos []*model.Memo, outgoingMap map[int64][]model.Link) []Insight {
	type candidate struct {
		a, b       int64
		aTitle     string
		bTitle     string
		similarity float64
	}

	keywords := make(map[int64]map[string]bool)
	for _, m := range memos {
		text := strings.ToLower(m.Title + " " + m.Content)
		keywords[m.ID] = charTrigrams(text)
	}

	// Build existing link set.
	type pairKey struct{ a, b int64 }
	existingLinks := make(map[pairKey]bool)
	for _, m := range memos {
		for _, link := range outgoingMap[m.ID] {
			p := pairKey{m.ID, link.TargetID}
			if p.a > p.b {
				p = pairKey{p.b, p.a}
			}
			existingLinks[p] = true
		}
	}

	var candidates []candidate
	for i := 0; i < len(memos); i++ {
		for j := i + 1; j < len(memos); j++ {
			a, b := memos[i], memos[j]
			p := pairKey{a.ID, b.ID}
			if p.a > p.b {
				p = pairKey{p.b, p.a}
			}
			if existingLinks[p] {
				continue
			}

			wordsA := keywords[a.ID]
			wordsB := keywords[b.ID]
			if len(wordsA) == 0 || len(wordsB) == 0 {
				continue
			}
			intersection := 0
			for w := range wordsA {
				if wordsB[w] {
					intersection++
				}
			}
			union := len(wordsA) + len(wordsB) - intersection
			if union == 0 {
				continue
			}
			sim := float64(intersection) / float64(union)
			if sim > 0.08 {
				candidates = append(candidates, candidate{
					a: a.ID, b: b.ID,
					aTitle: a.Title, bTitle: b.Title,
					similarity: sim,
				})
			}
		}
	}

	for i := 0; i < len(candidates); i++ {
		for j := i + 1; j < len(candidates); j++ {
			if candidates[j].similarity > candidates[i].similarity {
				candidates[i], candidates[j] = candidates[j], candidates[i]
			}
		}
	}
	if len(candidates) > 10 {
		candidates = candidates[:10]
	}

	var insights []Insight
	for _, c := range candidates {
		insights = append(insights, Insight{
			Type:        "suggested_link",
			SourceMemos: []int64{c.a, c.b},
			Suggestion:  fmt.Sprintf("%d(%s) and %d(%s) — %.0f%% similarity, consider linking", c.a, c.aTitle, c.b, c.bTitle, c.similarity*100),
		})
	}
	return insights
}

func printReflectMD(report *ReflectReport) {
	var b strings.Builder

	fmt.Fprintf(&b, "# Reflect Report\n\n")
	fmt.Fprintf(&b, "## Stats\n\n")
	fmt.Fprintf(&b, "- Concrete: %d, Abstract: %d, Ratio: %.1f%%\n", report.Stats.ConcreteCount, report.Stats.AbstractCount, report.Stats.Ratio*100)
	fmt.Fprintf(&b, "- %s\n\n", report.Stats.Recommendation)

	fmt.Fprintf(&b, "## Insights\n\n")

	tensions := filterInsights(report.Insights, "tension")
	hubs := filterInsights(report.Insights, "hub_without_abstraction")
	orphans := filterInsights(report.Insights, "orphan_cluster")
	bloated := filterInsights(report.Insights, "bloated_note")
	suggested := filterInsights(report.Insights, "suggested_link")

	if len(tensions) > 0 {
		fmt.Fprintf(&b, "### Tensions\n\n")
		for _, ins := range tensions {
			fmt.Fprintf(&b, "- [%v] %s\n", ins.SourceMemos, ins.Suggestion)
		}
		fmt.Fprintf(&b, "\n")
	}

	if len(hubs) > 0 {
		fmt.Fprintf(&b, "### Hub Without Abstraction\n\n")
		for _, ins := range hubs {
			fmt.Fprintf(&b, "- [%v] %s\n", ins.SourceMemos, ins.Suggestion)
		}
		fmt.Fprintf(&b, "\n")
	}

	if len(orphans) > 0 {
		fmt.Fprintf(&b, "### Orphan Memos\n\n")
		for _, ins := range orphans {
			fmt.Fprintf(&b, "- [%v] %s\n", ins.SourceMemos, ins.Suggestion)
		}
		fmt.Fprintf(&b, "\n")
	}

	if len(bloated) > 0 {
		fmt.Fprintf(&b, "### Bloated Memos\n\n")
		for _, ins := range bloated {
			fmt.Fprintf(&b, "- [%v] %s\n", ins.SourceMemos, ins.Suggestion)
		}
		fmt.Fprintf(&b, "\n")
	}

	if len(suggested) > 0 {
		fmt.Fprintf(&b, "### Suggested Links\n\n")
		for _, ins := range suggested {
			if len(ins.SourceMemos) >= 2 {
				fmt.Fprintf(&b, "- [%d <-> %d] %s\n", ins.SourceMemos[0], ins.SourceMemos[1], ins.Suggestion)
			}
		}
		fmt.Fprintf(&b, "\n")
	}

	if len(tensions) == 0 && len(hubs) == 0 && len(orphans) == 0 && len(bloated) == 0 && len(suggested) == 0 {
		fmt.Fprintf(&b, "No insights found. Memos are well-structured.\n")
	}

	fmt.Fprint(os.Stdout, b.String())
}

func filterInsights(insights []Insight, insightType string) []Insight {
	var result []Insight
	for _, ins := range insights {
		if ins.Type == insightType {
			result = append(result, ins)
		}
	}
	return result
}
