package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/BG4JTS/AISwitch/core"
	"github.com/BG4JTS/AISwitch/internal/config"
	"github.com/BG4JTS/AISwitch/internal/proxy"
)

var (
	provider    string
	key         string
	model       string
	port        int
	baseURL     string
	verbose     bool
	profileName string
)

var serveCmd = &cobra.Command{
	Use:   "serve",
	Short: "Start the AI Switch server",
	Long: `Start the AI Switch proxy server.

If --key or --model are omitted, the default profile from
~/.ais/config.json is used. Set a default with:
  ais config use <name>`,

	Run: func(cmd *cobra.Command, args []string) {
		// Load saved config
		cfg, _ := config.Load()

		// Build proxy config from flags
		pcfg := proxy.Config{
			Provider: provider,
			Key:      key,
			Model:    model,
			BaseURL:  baseURL,
			Verbose:  verbose,
		}

		// Merge profile defaults for missing values
		prof := profileName
		if prof == "" && cfg != nil {
			prof = cfg.DefaultProfile
		}
		if cfg != nil && prof != "" {
			p := cfg.GetProfile(prof)
			if p == nil {
				fmt.Fprintf(os.Stderr, "Error: profile %q not found\n", prof)
				os.Exit(1)
			}
			if pcfg.Provider == "openai" && p.Provider != "" {
				pcfg.Provider = p.Provider
			}
			if pcfg.Key == "" {
				pcfg.Key = p.Key
			}
			if pcfg.Model == "" {
				pcfg.Model = p.Model
			}
			if pcfg.BaseURL == "" {
				pcfg.BaseURL = p.BaseURL
			}
		}

		// Create modular server
		srv := core.NewServer(pcfg)
		srv.Port(port)

		// Auto-register modules discovered via init()
		for name, m := range core.GetModules() {
			if err := srv.RegisterModule(m); err != nil {
				fmt.Fprintf(os.Stderr, "Warning: skipping module %s: %v\n", name, err)
			}
		}

		if err := srv.Run(); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
	},
}

func init() {
	serveCmd.Flags().StringVar(&provider, "provider", "openai", "AI provider (openai, anthropic, deepseek)")
	serveCmd.Flags().StringVar(&key, "key", "", "API key (can be loaded from config)")
	serveCmd.Flags().StringVar(&model, "model", "", "Model name (can be loaded from config)")
	serveCmd.Flags().IntVar(&port, "port", 8080, "Port to listen on (use --port, not --server.port)")
	serveCmd.Flags().StringVar(&baseURL, "base-url", "", "Custom base URL")
	serveCmd.Flags().BoolVar(&verbose, "verbose", false, "Enable verbose debug output")
	serveCmd.Flags().StringVar(&profileName, "profile", "", "Profile name from config (default: use default profile)")
}
