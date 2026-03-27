package output

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/sheeppattern/nete/internal/model"
)

// memoView is a JSON/YAML-serializable representation of a Memo.
type memoView struct {
	ID       int64          `json:"id"                 yaml:"id"`
	Title    string         `json:"title"              yaml:"title"`
	Content  string         `json:"content"            yaml:"content"`
	Layer    string         `json:"layer"              yaml:"layer"`
	Tags     []string       `json:"tags"               yaml:"tags"`
	NoteID   int64          `json:"note_id"            yaml:"note_id"`
	Metadata model.Metadata `json:"metadata"           yaml:"metadata"`
}

func toMemoView(m *model.Memo) memoView {
	layer := m.Layer
	if layer == "" {
		layer = "concrete"
	}
	return memoView{
		ID:       m.ID,
		Title:    m.Title,
		Content:  m.Content,
		Layer:    layer,
		Tags:     m.Tags,
		NoteID:   m.NoteID,
		Metadata: m.Metadata,
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

// PrintMemo formats a single memo and prints it to stdout.
func (f *Formatter) PrintMemo(memo *model.Memo) error {
	switch f.Format {
	case "json":
		return f.PrintJSON(toMemoView(memo))
	case "yaml":
		return f.PrintYAML(toMemoView(memo))
	case "md":
		return f.printMemoMD(memo)
	default:
		return fmt.Errorf("unsupported format: %s", f.Format)
	}
}

// PrintMemos formats a list of memos and prints it to stdout.
func (f *Formatter) PrintMemos(memos []*model.Memo) error {
	switch f.Format {
	case "json":
		views := make([]memoView, len(memos))
		for i, m := range memos {
			views[i] = toMemoView(m)
		}
		return f.PrintJSON(views)
	case "yaml":
		views := make([]memoView, len(memos))
		for i, m := range memos {
			views[i] = toMemoView(m)
		}
		return f.PrintYAML(views)
	case "md":
		return f.printMemosMD(memos)
	default:
		return fmt.Errorf("unsupported format: %s", f.Format)
	}
}

// PrintNote formats a single note and prints it to stdout.
func (f *Formatter) PrintNote(note *model.Note) error {
	switch f.Format {
	case "json":
		return f.PrintJSON(note)
	case "yaml":
		return f.PrintYAML(note)
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
		return f.PrintJSON(notes)
	case "yaml":
		return f.PrintYAML(notes)
	case "md":
		return f.printNotesMD(notes)
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
		return f.printConfigMD(cfg)
	default:
		return fmt.Errorf("unsupported format: %s", f.Format)
	}
}

func (f *Formatter) printConfigMD(cfg *model.Config) error {
	var b strings.Builder
	fmt.Fprintln(&b, "# Configuration")
	fmt.Fprintln(&b)
	fmt.Fprintf(&b, "| Key | Value |\n")
	fmt.Fprintf(&b, "|-----|-------|\n")
	fmt.Fprintf(&b, "| default_note | %s |\n", cfg.DefaultNote)
	fmt.Fprintf(&b, "| default_format | %s |\n", cfg.DefaultFormat)
	fmt.Fprintf(&b, "| default_author | %s |\n", cfg.DefaultAuthor)
	_, err := fmt.Fprint(os.Stdout, b.String())
	return err
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

// printMemoMD renders a single memo in Markdown and prints it to stdout.
func (f *Formatter) printMemoMD(memo *model.Memo) error {
	var b strings.Builder

	fmt.Fprintf(&b, "# %s\n\n", memo.Title)
	fmt.Fprintf(&b, "**ID**: %d\n", memo.ID)
	fmt.Fprintf(&b, "**Tags**: %s\n", strings.Join(memo.Tags, ", "))
	fmt.Fprintf(&b, "**Note**: %d\n", memo.NoteID)
	fmt.Fprintf(&b, "**Status**: %s\n", memo.Metadata.Status)
	if memo.Metadata.Summary != "" {
		fmt.Fprintf(&b, "**Summary**: %s\n", memo.Metadata.Summary)
	}
	if memo.Metadata.Author != "" {
		fmt.Fprintf(&b, "**Author**: %s\n", memo.Metadata.Author)
	}
	fmt.Fprintf(&b, "**Created**: %s\n", memo.Metadata.CreatedAt.Format("2006-01-02 15:04:05"))
	fmt.Fprintf(&b, "\n%s\n", memo.Content)
	fmt.Fprint(&b, "\n")
	_, err := fmt.Fprint(os.Stdout, b.String())
	return err
}

// printMemosMD renders a brief table of memos in Markdown and prints it to stdout.
func (f *Formatter) printMemosMD(memos []*model.Memo) error {
	var b strings.Builder

	fmt.Fprintln(&b, "| ID | Title | Tags | Status | Summary |")
	fmt.Fprintln(&b, "|----|-------|------|--------|---------|")
	for _, m := range memos {
		tags := strings.Join(m.Tags, ", ")
		summary := m.Metadata.Summary
		if len(summary) > 40 {
			summary = summary[:40] + "..."
		}
		fmt.Fprintf(&b, "| %d | %s | %s | %s | %s |\n", m.ID, m.Title, tags, m.Metadata.Status, summary)
	}

	_, err := fmt.Fprint(os.Stdout, b.String())
	return err
}

// printNoteMD renders a single note in Markdown and prints it to stdout.
func (f *Formatter) printNoteMD(n *model.Note) error {
	var b strings.Builder

	fmt.Fprintf(&b, "# %s\n\n", n.Name)
	fmt.Fprintf(&b, "**ID**: %d\n", n.ID)
	fmt.Fprintf(&b, "**Description**: %s\n", n.Description)
	fmt.Fprintf(&b, "**Created**: %s\n\n", n.CreatedAt.Format("2006-01-02 15:04:05"))

	_, err := fmt.Fprint(os.Stdout, b.String())
	return err
}

// printNotesMD renders a brief table of notes in Markdown and prints it to stdout.
func (f *Formatter) printNotesMD(notes []*model.Note) error {
	var b strings.Builder

	fmt.Fprintln(&b, "| ID | Name | Description |")
	fmt.Fprintln(&b, "|----|------|-------------|")
	for _, n := range notes {
		fmt.Fprintf(&b, "| %d | %s | %s |\n", n.ID, n.Name, n.Description)
	}

	_, err := fmt.Fprint(os.Stdout, b.String())
	return err
}
