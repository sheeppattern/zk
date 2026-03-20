package cmd

import (
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"github.com/sheeppattern/zk/internal/model"
	"github.com/sheeppattern/zk/internal/store"
)

// DiagnosticReport holds the full diagnosis results.
type DiagnosticReport struct {
	TotalNotes int               `json:"total_notes" yaml:"total_notes"`
	TotalLinks int               `json:"total_links" yaml:"total_links"`
	Errors     []DiagnosticItem  `json:"errors" yaml:"errors"`
	Warnings   []DiagnosticItem  `json:"warnings" yaml:"warnings"`
	Summary    DiagnosticSummary `json:"summary" yaml:"summary"`
}

// DiagnosticItem represents a single diagnostic finding.
type DiagnosticItem struct {
	Severity string `json:"severity" yaml:"severity"`
	NoteID   string `json:"note_id" yaml:"note_id"`
	Message  string `json:"message" yaml:"message"`
}

// DiagnosticSummary provides an overview of the diagnosis.
type DiagnosticSummary struct {
	ErrorCount   int    `json:"error_count" yaml:"error_count"`
	WarningCount int    `json:"warning_count" yaml:"warning_count"`
	HealthScore  string `json:"health_score" yaml:"health_score"`
}

var diagnoseCmd = &cobra.Command{
	Use:   "diagnose",
	Short: "Diagnose storage for broken links, orphans, and invalid data",
	Long:  "Run diagnostic checks on the note store to find broken links, orphan notes, invalid relation types, invalid weights, and duplicate IDs.",
	RunE:  runDiagnose,
}

func init() {
	rootCmd.AddCommand(diagnoseCmd)
}

func runDiagnose(cmd *cobra.Command, args []string) error {
	s := store.NewStore(getStorePath(cmd))
	f := getFormatter()

	notes, err := s.ListNotes(flagProject)
	if err != nil {
		return fmt.Errorf("load notes: %w", err)
	}

	report := buildDiagnosticReport(notes)

	switch f.Format {
	case "json":
		return f.PrintJSON(report)
	case "yaml":
		return f.PrintYAML(report)
	case "md":
		printDiagnosticMD(report)
		return nil
	default:
		return fmt.Errorf("unsupported format: %s", f.Format)
	}
}

func buildDiagnosticReport(notes []*model.Note) *DiagnosticReport {
	report := &DiagnosticReport{
		Errors:   []DiagnosticItem{},
		Warnings: []DiagnosticItem{},
	}

	// Build index of all note IDs for quick lookup.
	noteIDs := make(map[string]int) // id -> count
	for _, n := range notes {
		noteIDs[n.ID]++
	}

	report.TotalNotes = len(notes)

	// Track which notes are link targets (incoming links).
	hasIncoming := make(map[string]bool)

	// Count total links and check each one.
	totalLinks := 0
	for _, n := range notes {
		totalLinks += len(n.Links)
		for _, link := range n.Links {
			hasIncoming[link.TargetID] = true

			// Check broken links.
			if _, exists := noteIDs[link.TargetID]; !exists {
				report.Errors = append(report.Errors, DiagnosticItem{
					Severity: "error",
					NoteID:   n.ID,
					Message:  fmt.Sprintf("broken link: target note %q does not exist", link.TargetID),
				})
			}

			// Check invalid relation types.
			if !model.IsValidRelationType(link.RelationType) {
				report.Warnings = append(report.Warnings, DiagnosticItem{
					Severity: "warning",
					NoteID:   n.ID,
					Message:  fmt.Sprintf("invalid relation type %q on link to %s", link.RelationType, link.TargetID),
				})
			}

			// Check invalid weights.
			if link.Weight < 0.0 || link.Weight > 1.0 {
				report.Errors = append(report.Errors, DiagnosticItem{
					Severity: "error",
					NoteID:   n.ID,
					Message:  fmt.Sprintf("invalid weight %.4f on link to %s (must be 0.0-1.0)", link.Weight, link.TargetID),
				})
			}
		}
	}
	report.TotalLinks = totalLinks

	// Check for orphan notes: no outgoing links AND no incoming links.
	for _, n := range notes {
		hasOutgoing := len(n.Links) > 0
		if !hasOutgoing && !hasIncoming[n.ID] {
			report.Warnings = append(report.Warnings, DiagnosticItem{
				Severity: "warning",
				NoteID:   n.ID,
				Message:  "orphan note: no incoming or outgoing links",
			})
		}
	}

	// Check for duplicate IDs.
	for id, count := range noteIDs {
		if count > 1 {
			report.Errors = append(report.Errors, DiagnosticItem{
				Severity: "error",
				NoteID:   id,
				Message:  fmt.Sprintf("duplicate note ID found %d times", count),
			})
		}
	}

	// Build summary.
	report.Summary = DiagnosticSummary{
		ErrorCount:   len(report.Errors),
		WarningCount: len(report.Warnings),
	}
	switch {
	case report.Summary.ErrorCount > 0:
		report.Summary.HealthScore = "issues"
	case report.Summary.WarningCount > 0:
		report.Summary.HealthScore = "warnings"
	default:
		report.Summary.HealthScore = "healthy"
	}

	return report
}

func printDiagnosticMD(report *DiagnosticReport) {
	var b strings.Builder

	fmt.Fprintf(&b, "# Diagnostic Report\n\n")
	fmt.Fprintf(&b, "**Total Notes**: %d\n", report.TotalNotes)
	fmt.Fprintf(&b, "**Total Links**: %d\n", report.TotalLinks)
	fmt.Fprintf(&b, "**Health Score**: %s\n", report.Summary.HealthScore)
	fmt.Fprintf(&b, "**Errors**: %d\n", report.Summary.ErrorCount)
	fmt.Fprintf(&b, "**Warnings**: %d\n\n", report.Summary.WarningCount)

	if len(report.Errors) > 0 {
		fmt.Fprintf(&b, "## Errors\n\n")
		for _, item := range report.Errors {
			fmt.Fprintf(&b, "- **[%s]** %s\n", item.NoteID, item.Message)
		}
		fmt.Fprintf(&b, "\n")
	}

	if len(report.Warnings) > 0 {
		fmt.Fprintf(&b, "## Warnings\n\n")
		for _, item := range report.Warnings {
			fmt.Fprintf(&b, "- **[%s]** %s\n", item.NoteID, item.Message)
		}
		fmt.Fprintf(&b, "\n")
	}

	if len(report.Errors) == 0 && len(report.Warnings) == 0 {
		fmt.Fprintf(&b, "No issues found. Storage is healthy.\n")
	}

	fmt.Fprint(os.Stdout, b.String())
}
