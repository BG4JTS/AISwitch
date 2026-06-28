package cmd

import (
	"fmt"
	"net/http"
	"os"

	"github.com/spf13/cobra"
	"github.com/yourusername/ais/internal/proxy"
)

var (
	provider string
	key      string
	model    string
	port     int
	baseURL  string
	verbose  bool
)

var serveCmd = &cobra.Command{
	Use:   "serve",
	Short: "Start the AI Switch server",
	Run: func(cmd *cobra.Command, args []string) {
		if key == "" {
			fmt.Fprintln(os.Stderr, "Error: --key is required")
			os.Exit(1)
		}
		if model == "" {
			fmt.Fprintln(os.Stderr, "Error: --model is required")
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
	serveCmd.Flags().StringVar(&key, "key", "", "API key (required)")
	serveCmd.Flags().StringVar(&model, "model", "", "Model name (required)")
	serveCmd.Flags().IntVar(&port, "port", 8080, "Port to listen on")
	serveCmd.Flags().StringVar(&baseURL, "base-url", "", "Custom base URL")
	serveCmd.Flags().BoolVar(&verbose, "verbose", false, "Enable verbose debug output")

	serveCmd.MarkFlagRequired("key")
	serveCmd.MarkFlagRequired("model")
}
