package cmd

import (
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"
	"github.com/yourusername/ais/internal/config"
)

var configCmd = &cobra.Command{
	Use:   "config",
	Short: "Manage saved provider configurations",
	Long: `Store and manage AI provider profiles in ~/.ais/config.json.

Profiles can be used as defaults so you don't need to pass --key --model every time.`,
}

// ── config set ──────────────────────────────────────────────

var configSetCmd = &cobra.Command{
	Use:   "set <name> --provider <p> --key <k> --model <m> [--base-url <url>]",
	Short: "Add or update a named profile",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		name := args[0]
		provider, _ := cmd.Flags().GetString("provider")
		key, _ := cmd.Flags().GetString("key")
		model, _ := cmd.Flags().GetString("model")
		baseURL, _ := cmd.Flags().GetString("base-url")

		if key == "" || model == "" {
			return fmt.Errorf("--key and --model are required")
		}

		cfg, err := config.Load()
		if err != nil {
			return err
		}

		if err := cfg.SetProfile(config.Profile{
			Name:     name,
			Provider: provider,
			Key:      key,
			Model:    model,
			BaseURL:  baseURL,
		}); err != nil {
			return err
		}

		fmt.Printf("✅ Profile %q saved.\n", name)
		return nil
	},
}

// ── config use ──────────────────────────────────────────────

var configUseCmd = &cobra.Command{
	Use:   "use <name>",
	Short: "Set a profile as the default",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		name := args[0]
		cfg, err := config.Load()
		if err != nil {
			return err
		}
		if err := cfg.SetDefault(name); err != nil {
			return err
		}
		fmt.Printf("✅ Default profile set to %q.\n", name)
		return nil
	},
}

// ── config show ─────────────────────────────────────────────

var configShowCmd = &cobra.Command{
	Use:   "show [name]",
	Short: "Show the current config or a specific profile",
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := config.Load()
		if err != nil {
			return err
		}

		if len(args) > 0 {
			name := args[0]
			p := cfg.GetProfile(name)
			if p == nil {
				return fmt.Errorf("profile %q not found", name)
			}
			printProfile(name, p, name == cfg.DefaultProfile)
			return nil
		}

		// Show full config
		data, _ := json.MarshalIndent(cfg, "", "  ")
		fmt.Println(string(data))
		return nil
	},
}

// ── config list ─────────────────────────────────────────────

var configListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all saved profiles",
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := config.Load()
		if err != nil {
			return err
		}

		if len(cfg.Profiles) == 0 {
			fmt.Println("No profiles saved yet. Use `ais config set <name> ...` to add one.")
			return nil
		}

		tableFormat, _ := cmd.Flags().GetBool("table")
		if tableFormat {
			fmt.Printf("%-4s %-16s %-12s %-30s %-20s %s\n", "DEF", "NAME", "PROVIDER", "MODEL", "BASE_URL", "KEY(hidden)")
			fmt.Println("──── ──────────────── ──────────── ────────────────────────────── ──────────────────── ───────────")
		} else {
			fmt.Println("Profiles:")
		}
		for _, p := range cfg.Profiles {
			isDef := p.Name == cfg.DefaultProfile
			if tableFormat {
				defFlag := " "
				if isDef {
					defFlag = " ✓ "
				}
				keyHint := "***"
				if len(p.Key) > 0 {
					keyHint = p.Key[:min(8, len(p.Key))] + "***"
				}
				baseURL := p.BaseURL
				if baseURL == "" {
					baseURL = "(default)"
				}
				fmt.Printf("%-4s %-16s %-12s %-30s %-20s %s\n",
					defFlag, p.Name, p.Provider, p.Model, baseURL, keyHint)
			} else {
				defTag := ""
				if isDef {
					defTag = " [DEFAULT]"
				}
				fmt.Printf("  %s%s\n    provider=%s  model=%s  base_url=%s\n",
					p.Name, defTag, p.Provider, p.Model, p.BaseURL)
			}
		}

		if !tableFormat {
			fmt.Println("\nUse `ais config use <name>` to set a default.")
		}
		return nil
	},
}

// ── config delete ───────────────────────────────────────────

var configDeleteCmd = &cobra.Command{
	Use:   "delete <name>",
	Short: "Delete a saved profile",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		name := args[0]
		cfg, err := config.Load()
		if err != nil {
			return err
		}
		if err := cfg.DeleteProfile(name); err != nil {
			return err
		}
		fmt.Printf("✅ Profile %q deleted.\n", name)
		return nil
	},
}

func printProfile(name string, p *config.Profile, isDefault bool) {
	defTag := ""
	if isDefault {
		defTag = " (default)"
	}
	fmt.Printf("Profile: %s%s\n", name, defTag)
	fmt.Printf("  Provider : %s\n", p.Provider)
	fmt.Printf("  Model    : %s\n", p.Model)
	if p.BaseURL != "" {
		fmt.Printf("  Base URL : %s\n", p.BaseURL)
	}
	fmt.Printf("  Key      : %s***\n", p.Key[:min(8, len(p.Key))])
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func init() {
	// config set flags
	configSetCmd.Flags().String("provider", "deepseek", "Provider name")
	configSetCmd.Flags().String("key", "", "API key (required)")
	configSetCmd.Flags().String("model", "", "Model name (required)")
	configSetCmd.Flags().String("base-url", "", "Custom base URL")
	configSetCmd.MarkFlagRequired("key")
	configSetCmd.MarkFlagRequired("model")

	// config list flags
	configListCmd.Flags().BoolP("table", "t", false, "Output as a compact table")

	// register subcommands
	configCmd.AddCommand(configSetCmd)
	configCmd.AddCommand(configUseCmd)
	configCmd.AddCommand(configShowCmd)
	configCmd.AddCommand(configListCmd)
	configCmd.AddCommand(configDeleteCmd)

	rootCmd.AddCommand(configCmd)
}
