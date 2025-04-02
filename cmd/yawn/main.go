package main

import (
	"fmt"
	"log"
	"os"

	"github.com/Mayurifag/yawn/internal/app"
	"github.com/Mayurifag/yawn/internal/config"
	"github.com/Mayurifag/yawn/internal/gemini"
	"github.com/Mayurifag/yawn/internal/git"
	"github.com/spf13/cobra"
)

var (
	// Version is set during build
	version = "dev"

	// Flags
	flagConfigPath     string // Allow specifying a config file (though layering handles most cases)
	flagAPIKey         string
	flagAutoStage      bool
	flagAutoPush       bool
	flagVerbose        bool
	flagGenerateConfig bool
)

func main() {
	if err := rootCmd.Execute(); err != nil {
		// Cobra already prints the error, but we might want specific exit codes
		os.Exit(1)
	}
}

var rootCmd = &cobra.Command{
	Use:   "yawn",
	Short: "yawn 🥱 - AI Git Commiter using Google Gemini",
	Long: `Yawn analyzes your staged Git changes, sends them to a configured Google Gemini
model, and generates a conventional commit message.

It helps streamline your commit workflow by providing AI-suggested messages.
Configuration is loaded from ~/.config/yawn/config.toml, ./.yawn.toml, and environment
variables (YAWN_*)`,
	Version: version,
	// SilenceUsage: true, // Prevent usage printing on error, Cobra handles it well
	RunE: func(cmd *cobra.Command, args []string) error {
		// Handle --generate-config flag first
		if flagGenerateConfig {
			defaultToml, err := config.GenerateDefaultConfig()
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error generating default config: %v\n", err)
				return err
			}
			fmt.Println(defaultToml)
			return nil
		}

		// Determine project path (current directory)
		projectPath, err := os.Getwd()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error getting current directory: %v\n", err)
			return err
		}

		// Load configuration with flag overrides
		cfg, err := config.LoadConfig(projectPath, flagVerbose, flagAPIKey, flagAutoStage, flagAutoPush)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error loading configuration: %v\n", err)
			return err
		}

		// Ensure API Key exists before creating Gemini Client
		finalAPIKey := cfg.GeminiAPIKey // Get potentially overridden key
		if finalAPIKey == "" {
			// If still empty after load, prompt here or let App handle it?
			// Let App handle the interactive prompt for better flow control.
			// However, we need *a* client instance. Create it, App will check key inside Run.
			if cfg.Verbose {
				fmt.Fprintln(os.Stderr, "[MAIN] Gemini API key not found in config/env/flags, will prompt if needed.")
			}
		}

		// Setup dependencies
		gitClient, err := git.NewExecGitClient(cfg.Verbose)
		if err != nil {
			return fmt.Errorf("failed to create git client: %w", err)
		}

		geminiClient, err := gemini.NewClient(finalAPIKey)
		if err != nil {
			return fmt.Errorf("failed to create Gemini client: %w", err)
		}

		// Create and run the application
		yawnApp := app.NewApp(cfg, gitClient, geminiClient)
		if err := yawnApp.Run(); err != nil {
			log.Fatal(err)
		}
		return nil
	},
}

func init() {
	// Define flags
	rootCmd.PersistentFlags().BoolVarP(&flagVerbose, "verbose", "v", false, "Enable verbose logging output")
	rootCmd.Flags().StringVar(&flagConfigPath, "config", "", "Path to a specific config file (overrides project/user discovery)") // Less common due to layering
	rootCmd.Flags().StringVar(&flagAPIKey, "api-key", "", "Gemini API key (overrides config/env)")
	rootCmd.Flags().BoolVar(&flagAutoStage, "auto-stage", false, "Automatically stage all unstaged changes without prompting")
	rootCmd.Flags().BoolVar(&flagAutoPush, "auto-push", false, "Automatically push after commit")
	rootCmd.Flags().BoolVar(&flagGenerateConfig, "generate-config", false, "Print default configuration TOML to stdout and exit")

	// Hide the less common --config flag unless needed
	_ = rootCmd.Flags().MarkHidden("config")

	// Set version template
	rootCmd.SetVersionTemplate(`{{printf "%s version %s\n" .Name .Version}}`)
}
