package main

import (
	"fmt"
	"os"
	"runtime/debug"

	"github.com/Mayurifag/yawn/internal/app"
	"github.com/Mayurifag/yawn/internal/config"
	"github.com/Mayurifag/yawn/internal/git"
	"github.com/Mayurifag/yawn/internal/ui"
	"github.com/spf13/cobra"
)

var (
	version = "dev"
	commit  = ""
	builtBy = ""

	flagAPIKey         string
	flagAutoStage      bool
	flagAutoPush       bool
	flagGenerateConfig bool
)

func main() {
	if err := rootCmd.Execute(); err != nil {
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
	RunE: func(cmd *cobra.Command, args []string) error {
		if flagGenerateConfig {
			content, err := config.GenerateConfigContent("")
			if err != nil {
				ui.PrintError(fmt.Sprintf("Error generating default config: %v", err))
				return err
			}
			fmt.Print(string(content))
			return nil
		}

		projectPath, err := os.Getwd()
		if err != nil {
			ui.PrintError(fmt.Sprintf("Error getting current directory: %v", err))
			return err
		}

		flags := config.CLIFlags{}
		if cmd.Flags().Changed("api-key") {
			flags.APIKey = &flagAPIKey
		}
		if cmd.Flags().Changed("auto-stage") {
			flags.AutoStage = &flagAutoStage
		}
		if cmd.Flags().Changed("auto-push") {
			flags.AutoPush = &flagAutoPush
		}

		cfg, err := config.LoadConfig(projectPath, flags)
		if err != nil {
			ui.PrintError(fmt.Sprintf("Error loading configuration: %v", err))
			return err
		}

		gitClient, err := git.NewExecGitClient()
		if err != nil {
			ui.PrintError(fmt.Sprintf("Failed to create git client: %v", err))
			return err
		}

		yawnApp := app.NewApp(cfg, gitClient)
		if err := yawnApp.Run(cmd.Context()); err != nil {
			ui.PrintError(err.Error())
			os.Exit(1)
		}
		return nil
	},
}

var squashCmd = &cobra.Command{
	Use:   "squash",
	Short: "Squash all commits on current branch into one AI-generated commit",
	RunE: func(cmd *cobra.Command, args []string) error {
		projectPath, err := os.Getwd()
		if err != nil {
			ui.PrintError(fmt.Sprintf("Error getting current directory: %v", err))
			return err
		}

		cfg, err := config.LoadConfig(projectPath, config.CLIFlags{})
		if err != nil {
			ui.PrintError(fmt.Sprintf("Error loading configuration: %v", err))
			return err
		}

		gitClient, err := git.NewExecGitClient()
		if err != nil {
			ui.PrintError(fmt.Sprintf("Failed to create git client: %v", err))
			return err
		}

		yawnApp := app.NewApp(cfg, gitClient)
		if err := yawnApp.RunSquash(cmd.Context()); err != nil {
			ui.PrintError(err.Error())
			os.Exit(1)
		}
		return nil
	},
}

func init() {
	if builtBy == "goreleaser" {
		ui.Version = version
	} else {
		sha := commit
		if sha == "" {
			if info, ok := debug.ReadBuildInfo(); ok {
				for _, s := range info.Settings {
					if s.Key == "vcs.revision" {
						sha = s.Value
						break
					}
				}
			}
		}
		if len(sha) > 7 {
			sha = sha[:7]
		}
		if sha != "" {
			ui.Version = version + " (" + sha + ")"
		} else {
			ui.Version = version
		}
	}

	rootCmd.Flags().StringVar(&flagAPIKey, "api-key", "", "Gemini API key (overrides config/env)")
	rootCmd.Flags().BoolVar(&flagAutoStage, "auto-stage", false, "Automatically stage all unstaged changes without prompting")
	rootCmd.Flags().BoolVar(&flagAutoPush, "auto-push", false, "Automatically push after commit")
	rootCmd.Flags().BoolVar(&flagGenerateConfig, "generate-config", false, "Print default configuration TOML to stdout and exit")

	rootCmd.SetVersionTemplate(`{{printf "%s version %s\n" .Name .Version}}`)
	rootCmd.AddCommand(squashCmd)
}
