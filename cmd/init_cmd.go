package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/sheeppattern/zk/internal/store"
)

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Initialize the Zettelkasten storage",
	Long:  "Creates the root directory structure for the Zettelkasten memory store.",
	RunE: func(cmd *cobra.Command, args []string) error {
		storePath := getStorePath(cmd)

		s := store.NewStore(storePath)
		if err := s.Init(); err != nil {
			return fmt.Errorf("failed to initialize store: %w", err)
		}

		fmt.Fprintln(os.Stderr, "initialized zk store at", storePath)
		return nil
	},
}

func init() {
	initCmd.Flags().String("path", "", "path for the zk store (overrides default)")
}
