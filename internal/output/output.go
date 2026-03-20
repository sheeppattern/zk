package output

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/sheeppattern/zk/internal/model"
)

// noteView is a JSON/YAML-serializable representation of a Note
// that includes Content (which the model excludes via "-" tags).
type noteView struct {
	ID        string         `json:"id"                    yaml:"id"`
	Title     string         `json:"title"                 yaml:"title"`
	Content   string         `json:"content"               yaml:"content"`
	Tags      []string       `json:"tags"                  yaml:"tags"`
	Links     []model.Link   `json:"links"                 yaml:"links"`
	Metadata  model.Metadata `json:"metadata"              yaml:"metadata"`
	ProjectID string         `json:"project_id,omitempty"  yaml:"project_id,omitempty"`
}

func toNoteView(n *model.Note) noteView {
	return noteView{
		ID:        n.ID,
		Title:     n.Title,
		Content:   n.Content,
		Tags:      n.Tags,
		Links:     n.Links,
		Metadata:  n.Metadata,
		ProjectID: n.ProjectID,
	}
}

// Formatter handles formatting data for stdout in different formats.
type Formatter struct {
	Format string
}

// NewFormatter creates a Formatter for the given format ("json", "yaml", "md").
func NewFormatter(format string) *Formatter {
	return &Formatter{Format: format}
}

// PrintNote formats a single note and prints it to stdout.
func (f *Formatter) PrintNote(note *model.Note) error {
	switch f.Format {
	case "json":
		return f.PrintJSON(toNoteView(note))
	case "yaml":
		return f.PrintYAML(toNoteView(note))
	case "md":
		return f.printNoteMD(note)
	default:
		return fmt.Errorf("unsupported format: %s", f.Format)
	}
}

// PrintNotes formats a list of notes and prints it to stdout.
func (f *Formatter) PrintNotes(notes []*model.Note) error {
	switch f.Format {
	case "json":
		views := make([]noteView, len(notes))
		for i, n := range notes {
			views[i] = toNoteView(n)
		}
		return f.PrintJSON(views)
	case "yaml":
		views := make([]noteView, len(notes))
		for i, n := range notes {
			views[i] = toNoteView(n)
		}
		return f.PrintYAML(views)
	case "md":
		return f.printNotesMD(notes)
	default:
		return fmt.Errorf("unsupported format: %s", f.Format)
	}
}

// PrintProject formats a single project and prints it to stdout.
func (f *Formatter) PrintProject(p *model.Project) error {
	switch f.Format {
	case "json":
		return f.PrintJSON(p)
	case "yaml":
		return f.PrintYAML(p)
	case "md":
		return f.printProjectMD(p)
	default:
		return fmt.Errorf("unsupported format: %s", f.Format)
	}
}

// PrintProjects formats a list of projects and prints it to stdout.
func (f *Formatter) PrintProjects(projects []*model.Project) error {
	switch f.Format {
	case "json":
		return f.PrintJSON(projects)
	case "yaml":
		return f.PrintYAML(projects)
	case "md":
		return f.printProjectsMD(projects)
	default:
		return fmt.Errorf("unsupported format: %s", f.Format)
	}
}

// PrintConfig formats a config and prints it to stdout.
func (f *Formatter) PrintConfig(cfg *model.Config) error {
	switch f.Format {
	case "json":
		return f.PrintJSON(cfg)
	case "yaml":
		return f.PrintYAML(cfg)
	case "md":
		// Config doesn't have a special markdown template; use YAML as fallback.
		return f.PrintYAML(cfg)
	default:
		return fmt.Errorf("unsupported format: %s", f.Format)
	}
}

// PrintSuccess prints a success message to stderr.
func (f *Formatter) PrintSuccess(msg string) {
	fmt.Fprintln(os.Stderr, msg)
}

// PrintError prints an error message to stderr.
func (f *Formatter) PrintError(msg string) {
	fmt.Fprintln(os.Stderr, "error: "+msg)
}

// PrintJSON marshals v as indented JSON and prints it to stdout.
func (f *Formatter) PrintJSON(v interface{}) error {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return fmt.Errorf("json marshal: %w", err)
	}
	_, err = fmt.Fprintln(os.Stdout, string(data))
	return err
}

// PrintYAML marshals v as YAML and prints it to stdout.
func (f *Formatter) PrintYAML(v interface{}) error {
	data, err := yaml.Marshal(v)
	if err != nil {
		return fmt.Errorf("yaml marshal: %w", err)
	}
	_, err = fmt.Fprint(os.Stdout, string(data))
	return err
}

// printNoteMD renders a single note in Markdown and prints it to stdout.
func (f *Formatter) printNoteMD(note *model.Note) error {
	var b strings.Builder

	fmt.Fprintf(&b, "# %s\n\n", note.Title)
	fmt.Fprintf(&b, "**ID**: %s\n", note.ID)
	fmt.Fprintf(&b, "**Tags**: %s\n", strings.Join(note.Tags, ", "))
	fmt.Fprintf(&b, "**Project**: %s\n", note.ProjectID)
	fmt.Fprintf(&b, "**Status**: %s\n", note.Metadata.Status)
	if note.Metadata.Summary != "" {
		fmt.Fprintf(&b, "**Summary**: %s\n", note.Metadata.Summary)
	}
	if note.Metadata.Author != "" {
		fmt.Fprintf(&b, "**Author**: %s\n", note.Metadata.Author)
	}
	fmt.Fprintf(&b, "**Created**: %s\n", note.Metadata.CreatedAt.Format("2006-01-02 15:04:05"))
	fmt.Fprintf(&b, "\n%s\n", note.Content)

	if len(note.Links) > 0 {
		fmt.Fprintf(&b, "\n## Links\n")
		for _, l := range note.Links {
			fmt.Fprintf(&b, "- %s → %s (weight: %.2f)\n", l.RelationType, l.TargetID, l.Weight)
		}
	}

	fmt.Fprint(&b, "\n")
	_, err := fmt.Fprint(os.Stdout, b.String())
	return err
}

// printNotesMD renders a brief table of notes in Markdown and prints it to stdout.
func (f *Formatter) printNotesMD(notes []*model.Note) error {
	var b strings.Builder

	fmt.Fprintln(&b, "| ID | Title | Tags | Status | Summary |")
	fmt.Fprintln(&b, "|----|-------|------|--------|---------|")
	for _, n := range notes {
		tags := strings.Join(n.Tags, ", ")
		summary := n.Metadata.Summary
		if len(summary) > 40 {
			summary = summary[:40] + "..."
		}
		fmt.Fprintf(&b, "| %s | %s | %s | %s | %s |\n", n.ID, n.Title, tags, n.Metadata.Status, summary)
	}

	_, err := fmt.Fprint(os.Stdout, b.String())
	return err
}

// printProjectMD renders a single project in Markdown and prints it to stdout.
func (f *Formatter) printProjectMD(p *model.Project) error {
	var b strings.Builder

	fmt.Fprintf(&b, "# %s\n\n", p.Name)
	fmt.Fprintf(&b, "**ID**: %s\n", p.ID)
	fmt.Fprintf(&b, "**Description**: %s\n", p.Description)
	fmt.Fprintf(&b, "**Created**: %s\n\n", p.CreatedAt.Format("2006-01-02 15:04:05"))

	_, err := fmt.Fprint(os.Stdout, b.String())
	return err
}

// printProjectsMD renders a brief table of projects in Markdown and prints it to stdout.
func (f *Formatter) printProjectsMD(projects []*model.Project) error {
	var b strings.Builder

	fmt.Fprintln(&b, "| ID | Name | Description |")
	fmt.Fprintln(&b, "|----|------|-------------|")
	for _, p := range projects {
		fmt.Fprintf(&b, "| %s | %s | %s |\n", p.ID, p.Name, p.Description)
	}

	_, err := fmt.Fprint(os.Stdout, b.String())
	return err
}
