package cmd

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/sheeppattern/zk/internal/model"
	"github.com/sheeppattern/zk/internal/store"
)

// ProjectDetail wraps a Project with computed statistics.
type ProjectDetail struct {
	*model.Project
	NoteCount    int        `json:"note_count" yaml:"note_count"`
	LinkCount    int        `json:"link_count" yaml:"link_count"`
	LastActivity *time.Time `json:"last_activity,omitempty" yaml:"last_activity,omitempty"`
}

var projectCmd = &cobra.Command{
	Use:   "project",
	Short: "Manage projects",
	Long:  "Create, list, get, and delete projects in the Zettelkasten store.",
}

var projectCreateCmd = &cobra.Command{
	Use:   "create <name>",
	Short: "Create a new project",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		name := args[0]
		description, _ := cmd.Flags().GetString("description")

		s := store.NewStore(getStorePath(cmd))
		p := model.NewProject(name, description)

		if err := s.CreateProject(p); err != nil {
			return fmt.Errorf("create project: %w", err)
		}

		return getFormatter().PrintProject(p)
	},
}

var projectListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all projects",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		s := store.NewStore(getStorePath(cmd))

		projects, err := s.ListProjects()
		if err != nil {
			return fmt.Errorf("list projects: %w", err)
		}

		return getFormatter().PrintProjects(projects)
	},
}

var projectGetCmd = &cobra.Command{
	Use:   "get <id>",
	Short: "Get a project by ID",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		id := args[0]
		s := store.NewStore(getStorePath(cmd))

		p, err := s.GetProject(id)
		if err != nil {
			return fmt.Errorf("get project: %w", err)
		}

		// Gather note statistics.
		notes, err := s.ListNotes(id)
		if err != nil {
			return fmt.Errorf("list notes for project: %w", err)
		}

		noteCount := len(notes)
		linkCount := 0
		var lastActivity *time.Time
		for _, n := range notes {
			linkCount += len(n.Links)
			t := n.Metadata.UpdatedAt
			if lastActivity == nil || t.After(*lastActivity) {
				copied := t
				lastActivity = &copied
			}
		}

		detail := ProjectDetail{
			Project:      p,
			NoteCount:    noteCount,
			LinkCount:    linkCount,
			LastActivity: lastActivity,
		}

		f := getFormatter()
		switch f.Format {
		case "json":
			return f.PrintJSON(detail)
		case "yaml":
			return f.PrintYAML(detail)
		case "md":
			var b strings.Builder
			fmt.Fprintf(&b, "# %s\n\n", p.Name)
			fmt.Fprintf(&b, "**ID**: %s\n", p.ID)
			fmt.Fprintf(&b, "**Description**: %s\n", p.Description)
			fmt.Fprintf(&b, "**Created**: %s\n", p.CreatedAt.Format("2006-01-02 15:04:05"))
			fmt.Fprintf(&b, "**Notes**: %d\n", noteCount)
			fmt.Fprintf(&b, "**Links**: %d\n", linkCount)
			if lastActivity != nil {
				fmt.Fprintf(&b, "**Last Activity**: %s\n", lastActivity.Format("2006-01-02 15:04:05"))
			}
			fmt.Fprintln(&b)
			_, err := fmt.Fprint(os.Stdout, b.String())
			return err
		default:
			return fmt.Errorf("unsupported format: %s", f.Format)
		}
	},
}

var projectDeleteCmd = &cobra.Command{
	Use:   "delete <id>",
	Short: "Delete a project by ID",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		id := args[0]
		s := store.NewStore(getStorePath(cmd))

		if err := s.DeleteProject(id); err != nil {
			return fmt.Errorf("delete project: %w", err)
		}

		statusf("deleted project %s", id)
		return nil
	},
}

func init() {
	projectCreateCmd.Flags().String("description", "", "project description")

	projectCmd.AddCommand(projectCreateCmd)
	projectCmd.AddCommand(projectListCmd)
	projectCmd.AddCommand(projectGetCmd)
	projectCmd.AddCommand(projectDeleteCmd)

	rootCmd.AddCommand(projectCmd)
}
