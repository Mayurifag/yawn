package main

import (
	"fmt"
	"os"

	"github.com/Mayurifag/yawn/internal/app"
	"github.com/Mayurifag/yawn/internal/config"
	"github.com/Mayurifag/yawn/internal/git"
	"github.com/Mayurifag/yawn/internal/ui"
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
				ui.PrintError(fmt.Sprintf("Error generating default config: %v", err))
				return err
			}
			fmt.Println(defaultToml)
			return nil
		}

		// Determine project path (current directory)
		projectPath, err := os.Getwd()
		if err != nil {
			ui.PrintError(fmt.Sprintf("Error getting current directory: %v", err))
			return err
		}

		// Load configuration with flag overrides
		flagsSpecified := []string{}

		// Check which flags were explicitly set by the user
		if cmd.Flags().Changed("verbose") {
			flagsSpecified = append(flagsSpecified, "verbose")
		}
		if cmd.Flags().Changed("api-key") {
			flagsSpecified = append(flagsSpecified, "api-key")
		}
		if cmd.Flags().Changed("auto-stage") {
			flagsSpecified = append(flagsSpecified, "stage")
		}
		if cmd.Flags().Changed("auto-push") {
			flagsSpecified = append(flagsSpecified, "push")
		}

		cfg, err := config.LoadConfig(projectPath, flagVerbose, flagAPIKey, flagAutoStage, flagAutoPush, flagsSpecified...)
		if err != nil {
			ui.PrintError(fmt.Sprintf("Error loading configuration: %v", err))
			return err
		}

		// Setup git client
		gitClient, err := git.NewExecGitClient(cfg.Verbose)
		if err != nil {
			ui.PrintError(fmt.Sprintf("Failed to create git client: %v", err))
			return err
		}

		// Create and run the application
		yawnApp := app.NewApp(cfg, gitClient)
		if err := yawnApp.Run(); err != nil {
			ui.PrintError(err.Error())
			os.Exit(1)
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
