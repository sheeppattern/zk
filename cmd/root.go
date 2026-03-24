package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"

	"github.com/spf13/cobra"
	"github.com/sheeppattern/zk/internal/output"
	"github.com/sheeppattern/zk/internal/store"
)

// Version is set at build time via -ldflags.
var Version = "dev"

var (
	flagFormat  string
	flagNote    int64
	flagVerbose bool
	flagQuiet   bool
)

var rootCmd = &cobra.Command{
	Use:   "zk",
	Short: "Zettelkasten memory CLI for AI agents",
	Long:  "A Zettelkasten-style memory system designed for AI agents to store, link, and retrieve knowledge memos.",
	PersistentPreRun: func(cmd *cobra.Command, args []string) {},
}

func init() {
	rootCmd.PersistentFlags().StringVar(&flagFormat, "format", "json", "output format: json, yaml, md")
	rootCmd.PersistentFlags().Int64Var(&flagNote, "note", 0, "note scope for memos (0 = global)")
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

// getStorePath returns the store directory path by checking:
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

// getDBPath returns the path to store.db file.
func getDBPath(cmd *cobra.Command) string {
	storePath := getStorePath(cmd)
	return filepath.Join(storePath, "store.db")
}

// openStore opens the SQLite database, ensures schema exists, and returns a Store.
// It also applies the default note from config if --note was not explicitly set.
// Caller must defer Close().
func openStore(cmd *cobra.Command) (*store.Store, error) {
	dbPath := getDBPath(cmd)
	s, err := store.NewStore(dbPath)
	if err != nil {
		return nil, fmt.Errorf("open store at %s: %w", dbPath, err)
	}
	if err := s.Init(); err != nil {
		s.Close()
		return nil, fmt.Errorf("init store: %w", err)
	}
	// Apply default note from config if --note not explicitly set.
	if !cmd.Flags().Changed("note") {
		if cfg, err := s.LoadConfig(); err == nil && cfg.DefaultNote != "" {
			if parsed, err := strconv.ParseInt(cfg.DefaultNote, 10, 64); err == nil {
				flagNote = parsed
				debugf("using default note from config: %d", flagNote)
			}
		}
	}
	return s, nil
}

// getFormatter returns a formatter based on the --format flag.
func getFormatter() *output.Formatter {
	return output.NewFormatter(flagFormat)
}
