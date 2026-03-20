package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
	"github.com/sheeppattern/zk/internal/output"
	"github.com/sheeppattern/zk/internal/store"
)

var (
	flagFormat  string
	flagProject string
	flagVerbose bool
	flagQuiet   bool
)

var rootCmd = &cobra.Command{
	Use:   "zk",
	Short: "Zettelkasten memory CLI for AI agents",
	Long:  "A Zettelkasten-style memory system designed for AI agents to store, link, and retrieve knowledge notes.",
	PersistentPreRun: func(cmd *cobra.Command, args []string) {
		// Apply default project from config if --project not explicitly set.
		if flagProject == "" {
			storePath := getStorePathSilent(cmd)
			if storePath != "" {
				cfgPath := filepath.Join(storePath, "config.yaml")
				if _, err := os.Stat(cfgPath); err == nil {
					s := store.NewStore(storePath)
					if cfg, err := s.LoadConfig(); err == nil && cfg.DefaultProject != "" {
						flagProject = cfg.DefaultProject
						debugf("using default project from config: %s", cfg.DefaultProject)
					}
				}
			}
		}
	},
}

func init() {
	rootCmd.PersistentFlags().StringVar(&flagFormat, "format", "json", "output format: json, yaml, md")
	rootCmd.PersistentFlags().StringVar(&flagProject, "project", "", "project scope for notes")
	rootCmd.PersistentFlags().BoolVar(&flagVerbose, "verbose", false, "enable verbose output")
	rootCmd.PersistentFlags().BoolVar(&flagQuiet, "quiet", false, "suppress stderr status messages")

	rootCmd.AddCommand(initCmd)
}

// statusf prints a status message to stderr unless --quiet is set.
func statusf(format string, args ...interface{}) {
	if !flagQuiet {
		fmt.Fprintf(os.Stderr, format+"\n", args...)
	}
}

// debugf prints a debug message to stderr only when --verbose is set.
func debugf(format string, args ...interface{}) {
	if flagVerbose {
		fmt.Fprintf(os.Stderr, "[debug] "+format+"\n", args...)
	}
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

// getStorePathSilent is like getStorePath but returns empty string instead of exiting on error.
func getStorePathSilent(cmd *cobra.Command) string {
	if p, _ := cmd.Flags().GetString("path"); p != "" {
		return p
	}
	if env := os.Getenv("ZKMEMORY_PATH"); env != "" {
		return env
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, ".zk-memory")
}

// getFormatter returns a formatter based on the --format flag.
func getFormatter() *output.Formatter {
	return output.NewFormatter(flagFormat)
}
