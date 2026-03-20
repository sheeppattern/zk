package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/sheeppattern/zk/internal/store"
)

var configCmd = &cobra.Command{
	Use:   "config",
	Short: "Manage zk configuration",
}

var configShowCmd = &cobra.Command{
	Use:   "show",
	Short: "Show current configuration",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		s := store.NewStore(getStorePath(cmd))
		cfg, err := s.LoadConfig()
		if err != nil {
			return fmt.Errorf("load config: %w", err)
		}
		return getFormatter().PrintConfig(cfg)
	},
}

var configSetCmd = &cobra.Command{
	Use:   "set <key> <value>",
	Short: "Set a configuration value",
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		key, value := args[0], args[1]

		s := store.NewStore(getStorePath(cmd))
		cfg, err := s.LoadConfig()
		if err != nil {
			return fmt.Errorf("load config: %w", err)
		}

		switch key {
		case "store_path":
			cfg.StorePath = value
		case "default_project":
			cfg.DefaultProject = value
		case "default_format":
			if value != "json" && value != "yaml" && value != "md" {
				return fmt.Errorf("invalid format %q: must be one of json, yaml, md", value)
			}
			cfg.DefaultFormat = value
		default:
			return fmt.Errorf("unknown config key %q: valid keys are store_path, default_project, default_format", key)
		}

		if err := s.SaveConfig(cfg); err != nil {
			return fmt.Errorf("save config: %w", err)
		}

		statusf("config %s set to %s", key, value)
		return nil
	},
}

func init() {
	configCmd.AddCommand(configShowCmd)
	configCmd.AddCommand(configSetCmd)
	rootCmd.AddCommand(configCmd)
}
