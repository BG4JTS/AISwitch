package cmd

import (
	"fmt"
	"net/http"
	"os"

	"github.com/spf13/cobra"
	"github.com/yourusername/ais/internal/config"
	"github.com/yourusername/ais/internal/proxy"
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
		// ── Try loading defaults from config file ──
		cfg, cfgErr := config.Load()
		if cfgErr != nil && key == "" {
			fmt.Fprintf(os.Stderr, "Warning: could not load config: %v\n", cfgErr)
		}

		// Determine which profile to use
		prof := profileName
		if prof == "" && cfg != nil {
			prof = cfg.DefaultProfile
		}

		// Apply config defaults for missing flags
		if cfg != nil && prof != "" {
			p := cfg.GetProfile(prof)
			if p == nil {
				fmt.Fprintf(os.Stderr, "Error: profile %q not found in config\n", prof)
				os.Exit(1)
			}
			if provider == "openai" && p.Provider != "" {
				provider = p.Provider
			}
			if key == "" {
				key = p.Key
			}
			if model == "" {
				model = p.Model
			}
			if baseURL == "" {
				baseURL = p.BaseURL
			}
		}

		// Final validation
		if key == "" {
			fmt.Fprintln(os.Stderr, "Error: --key is required (or set a default profile with `ais config`)")
			os.Exit(1)
		}
		if model == "" {
			fmt.Fprintln(os.Stderr, "Error: --model is required (or set a default profile with `ais config`)")
			os.Exit(1)
		}

		fmt.Printf("AI Switch started on port %d\n", port)
		if verbose {
			fmt.Println("[VERBOSE] Debug mode enabled")
		}

		proxyConfig := proxy.Config{
			Provider: provider,
			Key:      key,
			Model:    model,
			BaseURL:  baseURL,
			Verbose:  verbose,
		}

		http.HandleFunc("/v1/chat/completions", proxy.Handler(proxyConfig))
		http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
			http.NotFound(w, r)
		})

		if err := http.ListenAndServe(fmt.Sprintf(":%d", port), nil); err != nil {
			fmt.Fprintf(os.Stderr, "Error starting server: %v\n", err)
			os.Exit(1)
		}
	},
}

func init() {
	serveCmd.Flags().StringVar(&provider, "provider", "openai", "AI provider (openai, anthropic, deepseek)")
	serveCmd.Flags().StringVar(&key, "key", "", "API key (can be loaded from config)")
	serveCmd.Flags().StringVar(&model, "model", "", "Model name (can be loaded from config)")
	serveCmd.Flags().IntVar(&port, "port", 8080, "Port to listen on")
	serveCmd.Flags().StringVar(&baseURL, "base-url", "", "Custom base URL")
	serveCmd.Flags().BoolVar(&verbose, "verbose", false, "Enable verbose debug output")
	serveCmd.Flags().StringVar(&profileName, "profile", "", "Profile name from config (default: use default profile)")
}
