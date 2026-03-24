package cmd

import (
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"github.com/sheeppattern/zk/internal/model"
)

// DiagnosticReport holds the full diagnosis results.
type DiagnosticReport struct {
	TotalMemos int               `json:"total_memos" yaml:"total_memos"`
	TotalLinks int               `json:"total_links" yaml:"total_links"`
	Errors     []DiagnosticItem  `json:"errors" yaml:"errors"`
	Warnings   []DiagnosticItem  `json:"warnings" yaml:"warnings"`
	Summary    DiagnosticSummary `json:"summary" yaml:"summary"`
}

// DiagnosticItem represents a single diagnostic finding.
type DiagnosticItem struct {
	Severity string `json:"severity" yaml:"severity"`
	MemoID   int64  `json:"memo_id" yaml:"memo_id"`
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
	Short: "Diagnose storage for orphan memos, invalid data in links",
	Long:  "Run diagnostic checks on the store to find orphan memos, invalid relation types, and invalid weights in the links table.",
	Example: `  zk diagnose
  zk diagnose --format md`,
	RunE: runDiagnose,
}

func init() {
	rootCmd.AddCommand(diagnoseCmd)
}

func runDiagnose(cmd *cobra.Command, args []string) error {
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

	report := &DiagnosticReport{
		TotalMemos: len(memos),
		Errors:     []DiagnosticItem{},
		Warnings:   []DiagnosticItem{},
	}

	totalLinks := 0

	for _, m := range memos {
		outgoing, incoming, err := s.ListLinks(m.ID)
		if err != nil {
			continue
		}
		allLinks := append(outgoing, incoming...)
		totalLinks += len(outgoing)

		// Check for orphan memos (no links at all).
		if len(allLinks) == 0 {
			report.Warnings = append(report.Warnings, DiagnosticItem{
				Severity: "warning",
				MemoID:   m.ID,
				Message:  "orphan memo: no incoming or outgoing links",
			})
		}

		// Check invalid relation types and weights in outgoing links.
		for _, link := range outgoing {
			if !model.IsValidRelationType(link.RelationType) {
				report.Warnings = append(report.Warnings, DiagnosticItem{
					Severity: "warning",
					MemoID:   m.ID,
					Message:  fmt.Sprintf("invalid relation type %q on link to %d", link.RelationType, link.TargetID),
				})
			}
			if link.Weight < 0.0 || link.Weight > 1.0 {
				report.Errors = append(report.Errors, DiagnosticItem{
					Severity: "error",
					MemoID:   m.ID,
					Message:  fmt.Sprintf("invalid weight %.4f on link to %d (must be 0.0-1.0)", link.Weight, link.TargetID),
				})
			}
		}
	}

	report.TotalLinks = totalLinks

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

func printDiagnosticMD(report *DiagnosticReport) {
	var b strings.Builder

	fmt.Fprintf(&b, "# Diagnostic Report\n\n")
	fmt.Fprintf(&b, "**Total Memos**: %d\n", report.TotalMemos)
	fmt.Fprintf(&b, "**Total Links**: %d\n", report.TotalLinks)
	fmt.Fprintf(&b, "**Health Score**: %s\n", report.Summary.HealthScore)
	fmt.Fprintf(&b, "**Errors**: %d\n", report.Summary.ErrorCount)
	fmt.Fprintf(&b, "**Warnings**: %d\n", report.Summary.WarningCount)
	fmt.Fprintf(&b, "\n")

	if len(report.Errors) > 0 {
		fmt.Fprintf(&b, "## Errors\n\n")
		for _, item := range report.Errors {
			fmt.Fprintf(&b, "- **[%d]** %s\n", item.MemoID, item.Message)
		}
		fmt.Fprintf(&b, "\n")
	}

	if len(report.Warnings) > 0 {
		fmt.Fprintf(&b, "## Warnings\n\n")
		for _, item := range report.Warnings {
			fmt.Fprintf(&b, "- **[%d]** %s\n", item.MemoID, item.Message)
		}
		fmt.Fprintf(&b, "\n")
	}

	if len(report.Errors) == 0 && len(report.Warnings) == 0 {
		fmt.Fprintf(&b, "No issues found. Storage is healthy.\n")
	}

	fmt.Fprint(os.Stdout, b.String())
}
