package cmd

import (
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"github.com/sheeppattern/zk/internal/model"
	"github.com/sheeppattern/zk/internal/store"
)

// ReflectReport holds the full reflection analysis results.
type ReflectReport struct {
	Insights []Insight    `json:"insights" yaml:"insights"`
	Stats    ReflectStats `json:"stats" yaml:"stats"`
}

// Insight represents a single reflection finding.
type Insight struct {
	Type           string   `json:"type" yaml:"type"`                                         // "tension", "hub_without_abstraction", "orphan_cluster", "low_abstraction"
	SourceNotes    []string `json:"source_notes" yaml:"source_notes"`
	Suggestion     string   `json:"suggestion" yaml:"suggestion"`
	SuggestedTitle string   `json:"suggested_title,omitempty" yaml:"suggested_title,omitempty"`
}

// ReflectStats provides abstraction-level statistics.
type ReflectStats struct {
	ConcreteCount  int     `json:"concrete_count" yaml:"concrete_count"`
	AbstractCount  int     `json:"abstract_count" yaml:"abstract_count"`
	Ratio          float64 `json:"ratio" yaml:"ratio"`
	Recommendation string  `json:"recommendation" yaml:"recommendation"`
}

var flagApply bool

var reflectCmd = &cobra.Command{
	Use:   "reflect",
	Short: "Analyze notes and suggest abstract insights",
	Long:  "Analyze a project's notes to detect tensions, hubs without abstraction, and orphan clusters, then suggest abstract insight notes.",
	Example: `  zk reflect --project P-XXXXXX
  zk reflect --format md
  zk reflect --apply`,
	RunE: runReflect,
}

func init() {
	rootCmd.AddCommand(reflectCmd)
	reflectCmd.Flags().BoolVar(&flagApply, "apply", false, "auto-create suggested abstract notes")
}

func runReflect(cmd *cobra.Command, args []string) error {
	s := store.NewStore(getStorePath(cmd))
	f := getFormatter()

	notes, err := s.ListNotes(flagProject)
	if err != nil {
		return fmt.Errorf("list notes: %w", err)
	}

	report := buildReflectReport(notes)

	// --apply: create suggested abstract notes.
	if flagApply {
		created := 0
		for _, insight := range report.Insights {
			if insight.SuggestedTitle == "" {
				continue
			}
			newNote := model.NewNote(insight.SuggestedTitle, insight.Suggestion, []string{"auto-reflect"})
			newNote.Layer = "abstract"
			newNote.ProjectID = flagProject
			if err := s.CreateNote(newNote); err != nil {
				return fmt.Errorf("create abstract note: %w", err)
			}
			// Add "abstracts" links from each source note to the new note.
			for _, srcID := range insight.SourceNotes {
				srcNote, err := s.GetNote(flagProject, srcID)
				if err != nil {
					statusf("warning: could not read source note %s: %v", srcID, err)
					continue
				}
				srcNote.Links = append(srcNote.Links, model.Link{
					TargetID:     newNote.ID,
					RelationType: model.RelAbstracts,
					Weight:       0.7,
				})
				if err := s.UpdateNote(srcNote); err != nil {
					statusf("warning: could not update source note %s: %v", srcID, err)
				}
			}
			created++
		}
		statusf("created %d abstract notes", created)
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

func buildReflectReport(notes []*model.Note) *ReflectReport {
	report := &ReflectReport{
		Insights: []Insight{},
	}

	// Separate concrete and abstract notes.
	noteByID := make(map[string]*model.Note)
	var concreteNotes []*model.Note
	var abstractNotes []*model.Note

	for _, n := range notes {
		noteByID[n.ID] = n
		switch n.Layer {
		case model.LayerAbstract:
			abstractNotes = append(abstractNotes, n)
		default: // "" or "concrete"
			concreteNotes = append(concreteNotes, n)
		}
	}

	// Build adjacency: noteID -> list of links (outgoing).
	outgoing := make(map[string][]model.Link)
	incoming := make(map[string][]model.Link)
	for _, n := range notes {
		outgoing[n.ID] = n.Links
		for _, link := range n.Links {
			incoming[link.TargetID] = append(incoming[link.TargetID], model.Link{
				TargetID:     n.ID,
				RelationType: link.RelationType,
				Weight:       link.Weight,
			})
		}
	}

	// 1. Detect tensions: pairs connected by "contradicts" without a shared abstract note.
	type pair struct{ a, b string }
	seenPairs := make(map[pair]bool)

	for _, n := range notes {
		for _, link := range n.Links {
			if link.RelationType != model.RelContradicts {
				continue
			}
			p := pair{n.ID, link.TargetID}
			if p.a > p.b {
				p = pair{p.b, p.a}
			}
			if seenPairs[p] {
				continue
			}
			seenPairs[p] = true

			// Check if any abstract note is connected to both via "abstracts" or "grounds".
			hasSharedAbstract := false
			for _, absNote := range abstractNotes {
				connectsA := false
				connectsB := false
				for _, al := range absNote.Links {
					if al.RelationType == model.RelAbstracts || al.RelationType == model.RelGrounds {
						if al.TargetID == p.a {
							connectsA = true
						}
						if al.TargetID == p.b {
							connectsB = true
						}
					}
				}
				// Also check incoming links to the abstract note.
				for _, il := range incoming[absNote.ID] {
					if il.RelationType == model.RelAbstracts || il.RelationType == model.RelGrounds {
						if il.TargetID == p.a {
							connectsA = true
						}
						if il.TargetID == p.b {
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
				titleA := p.a
				titleB := p.b
				if na, ok := noteByID[p.a]; ok {
					titleA = na.Title
				}
				if nb, ok := noteByID[p.b]; ok {
					titleB = nb.Title
				}
				report.Insights = append(report.Insights, Insight{
					Type:           "tension",
					SourceNotes:    []string{p.a, p.b},
					Suggestion:     fmt.Sprintf("%s(%s)과 %s(%s) 사이에 긴장이 있지만 이를 종합하는 추상 노트가 없습니다", p.a, titleA, p.b, titleB),
					SuggestedTitle: fmt.Sprintf("%s vs %s — 어떤 판단이 필요한가?", titleA, titleB),
				})
			}
		}
	}

	// 2. Detect hubs without abstraction: concrete notes with 4+ outgoing links,
	//    none leading to an abstract note.
	for _, n := range concreteNotes {
		if len(n.Links) < 4 {
			continue
		}
		hasAbstractLink := false
		for _, link := range n.Links {
			if target, ok := noteByID[link.TargetID]; ok && target.Layer == model.LayerAbstract {
				hasAbstractLink = true
				break
			}
		}
		if !hasAbstractLink {
			report.Insights = append(report.Insights, Insight{
				Type:        "hub_without_abstraction",
				SourceNotes: []string{n.ID},
				Suggestion:  fmt.Sprintf("%s(%s)이 %d개 노트와 연결되어 있지만 상위 추상화가 없습니다", n.ID, n.Title, len(n.Links)),
			})
		}
	}

	// 3. Detect orphan clusters: concrete notes with 0 outgoing AND 0 incoming links.
	for _, n := range concreteNotes {
		hasOutgoing := len(n.Links) > 0
		hasIncoming := len(incoming[n.ID]) > 0
		if !hasOutgoing && !hasIncoming {
			report.Insights = append(report.Insights, Insight{
				Type:        "orphan_cluster",
				SourceNotes: []string{n.ID},
				Suggestion:  fmt.Sprintf("%s(%s)이 고립되어 있습니다 — 다른 노트와의 관계를 검토하세요", n.ID, n.Title),
			})
		}
	}

	// 4. Calculate stats.
	concreteCount := len(concreteNotes)
	abstractCount := len(abstractNotes)
	total := concreteCount + abstractCount
	var ratio float64
	if total > 0 {
		ratio = float64(abstractCount) / float64(total)
	}

	var recommendation string
	switch {
	case ratio < 0.1:
		recommendation = "인사이트 부족 — concrete 노트에서 패턴과 질문을 도출하세요"
	case ratio <= 0.3:
		recommendation = "양호 — 허브 노트와 긴장 관계에 추가 추상화를 고려하세요"
	default:
		recommendation = "충분한 추상화 수준"
	}

	report.Stats = ReflectStats{
		ConcreteCount:  concreteCount,
		AbstractCount:  abstractCount,
		Ratio:          ratio,
		Recommendation: recommendation,
	}

	return report
}

func printReflectMD(report *ReflectReport) {
	var b strings.Builder

	fmt.Fprintf(&b, "# Reflect Report\n\n")
	fmt.Fprintf(&b, "## Stats\n\n")
	fmt.Fprintf(&b, "- Concrete: %d, Abstract: %d, Ratio: %.1f%%\n", report.Stats.ConcreteCount, report.Stats.AbstractCount, report.Stats.Ratio*100)
	fmt.Fprintf(&b, "- %s\n\n", report.Stats.Recommendation)

	fmt.Fprintf(&b, "## Insights\n\n")

	// Group insights by type.
	tensions := filterInsights(report.Insights, "tension")
	hubs := filterInsights(report.Insights, "hub_without_abstraction")
	orphans := filterInsights(report.Insights, "orphan_cluster")

	if len(tensions) > 0 {
		fmt.Fprintf(&b, "### Tensions\n\n")
		for _, ins := range tensions {
			fmt.Fprintf(&b, "- [%s] %s\n", strings.Join(ins.SourceNotes, ", "), ins.Suggestion)
		}
		fmt.Fprintf(&b, "\n")
	}

	if len(hubs) > 0 {
		fmt.Fprintf(&b, "### Hub Without Abstraction\n\n")
		for _, ins := range hubs {
			fmt.Fprintf(&b, "- [%s] %s\n", strings.Join(ins.SourceNotes, ", "), ins.Suggestion)
		}
		fmt.Fprintf(&b, "\n")
	}

	if len(orphans) > 0 {
		fmt.Fprintf(&b, "### Orphan Notes\n\n")
		for _, ins := range orphans {
			fmt.Fprintf(&b, "- [%s] %s\n", strings.Join(ins.SourceNotes, ", "), ins.Suggestion)
		}
		fmt.Fprintf(&b, "\n")
	}

	if len(tensions) == 0 && len(hubs) == 0 && len(orphans) == 0 {
		fmt.Fprintf(&b, "No insights found. Notes are well-structured.\n")
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
