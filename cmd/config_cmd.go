package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

var configCmd = &cobra.Command{
	Use:   "config",
	Short: "Manage nete configuration",
}

var configShowCmd = &cobra.Command{
	Use:   "show",
	Short: "Show current configuration",
	Example: `  nete config show
  nete config show --format yaml`,
	Args: cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		s, err := openStore(cmd)
		if err != nil {
			return err
		}
		defer s.Close()

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
	Example: `  nete config set default_note 1
  nete config set default_format yaml`,
	Args: cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		key, value := args[0], args[1]

		s, err := openStore(cmd)
		if err != nil {
			return err
		}
		defer s.Close()

		cfg, err := s.LoadConfig()
		if err != nil {
			return fmt.Errorf("load config: %w", err)
		}

		switch key {
		case "store_path":
			cfg.StorePath = value
		case "default_note":
			cfg.DefaultNote = value
		case "default_format":
			if value != "json" && value != "yaml" && value != "md" {
				return fmt.Errorf("invalid format %q: must be one of json, yaml, md", value)
			}
			cfg.DefaultFormat = value
		case "default_author":
			cfg.DefaultAuthor = value
		default:
			return fmt.Errorf("unknown config key %q; valid keys: store_path, default_note, default_format, default_author", key)
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
