package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/ymh/zk/internal/model"
	"github.com/ymh/zk/internal/store"
)

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

		return getFormatter().PrintProject(p)
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

		fmt.Fprintln(os.Stderr, "deleted project", id)
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
