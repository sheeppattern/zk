package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
	"github.com/sheeppattern/zk/internal/output"
)

var (
	flagFormat  string
	flagProject string
	flagVerbose bool
)

var rootCmd = &cobra.Command{
	Use:   "zk",
	Short: "Zettelkasten memory CLI for AI agents",
	Long:  "A Zettelkasten-style memory system designed for AI agents to store, link, and retrieve knowledge notes.",
}

func init() {
	rootCmd.PersistentFlags().StringVar(&flagFormat, "format", "json", "output format: json, yaml, md")
	rootCmd.PersistentFlags().StringVar(&flagProject, "project", "", "project scope for notes")
	rootCmd.PersistentFlags().BoolVar(&flagVerbose, "verbose", false, "enable verbose output")

	rootCmd.AddCommand(initCmd)
}

// Execute runs the root command.
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

// getStorePath returns the store path by checking:
// 1) --path flag value, 2) ZKMEMORY_PATH env var, 3) default ~/.zk-memory.
func getStorePath(cmd *cobra.Command) string {
	if p, _ := cmd.Flags().GetString("path"); p != "" {
		return p
	}
	if env := os.Getenv("ZKMEMORY_PATH"); env != "" {
		return env
	}
	home, err := os.UserHomeDir()
	if err != nil {
		fmt.Fprintln(os.Stderr, "cannot determine home directory:", err)
		os.Exit(1)
	}
	return filepath.Join(home, ".zk-memory")
}

// getFormatter returns a formatter based on the --format flag.
func getFormatter() *output.Formatter {
	return output.NewFormatter(flagFormat)
}
