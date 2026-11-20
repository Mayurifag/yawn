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
	flagSquashBase     string
)

func main() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

var rootCmd = &cobra.Command{
	Use:   "yawn",
	Args:  cobra.NoArgs,
	Short: "yawn 🥱 - AI Git Commiter",
	Long: "Yawn analyzes your staged Git changes, sends them to a configured AI\n" +
		"provider, and generates a conventional commit message.\n\n" +
		"It helps streamline your commit workflow by providing AI-suggested messages.\n" +
		"Configuration is loaded from ~/.config/yawn/config.toml, ./.yawn.toml, and environment\n" +
		"variables (YAWN_*)",
	Version: version,
	RunE: func(cmd *cobra.Command, args []string) error {
		if flagGenerateConfig {
			fmt.Print(string(config.GenerateConfigContent()))
			return nil
		}

		projectPath, err := os.Getwd()
		if err != nil {
			return fmt.Errorf("getting current directory: %w", err)
		}

		gitClient, err := git.NewExecGitClient()
		if err != nil {
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
			return fmt.Errorf("loading configuration: %w", err)
		}

		yawnApp := app.NewApp(cfg, gitClient)
		return yawnApp.Run(cmd.Context())
	},
}

var forcePushCmd = &cobra.Command{
	Use:   "force-push",
	Args:  cobra.NoArgs,
	Short: "Force-push current branch with --force-with-lease and print repo/PR links",
	RunE: func(cmd *cobra.Command, args []string) error {
		projectPath, err := os.Getwd()
		if err != nil {
			return fmt.Errorf("getting current directory: %w", err)
		}

		gitClient, err := git.NewExecGitClient()
		if err != nil {
			return err
		}

		flags := config.CLIFlags{}
		if cmd.Flags().Changed("auto-push") {
			flags.AutoPush = &flagAutoPush
		}

		cfg, err := config.LoadConfig(projectPath, flags)
		if err != nil {
			return fmt.Errorf("loading configuration: %w", err)
		}

		yawnApp := app.NewApp(cfg, gitClient)
		return yawnApp.RunForcePush()
	},
}

var squashCmd = &cobra.Command{
	Use:   "squash",
	Args:  cobra.NoArgs,
	Short: "Squash all commits on current branch into one AI-generated commit",
	RunE: func(cmd *cobra.Command, args []string) error {
		projectPath, err := os.Getwd()
		if err != nil {
			return fmt.Errorf("getting current directory: %w", err)
		}

		gitClient, err := git.NewExecGitClient()
		if err != nil {
			return err
		}

		cfg, err := config.LoadConfig(projectPath, config.CLIFlags{})
		if err != nil {
			return fmt.Errorf("loading configuration: %w", err)
		}

		yawnApp := app.NewApp(cfg, gitClient)
		return yawnApp.RunSquash(cmd.Context(), flagSquashBase)
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

	rootCmd.Flags().StringVar(&flagAPIKey, "api-key", "", "AI provider API key (overrides config/env)")
	rootCmd.Flags().BoolVar(&flagAutoStage, "auto-stage", false, "Automatically stage all unstaged changes without prompting")
	rootCmd.Flags().BoolVar(&flagAutoPush, "auto-push", false, "Automatically push after commit")
	rootCmd.Flags().BoolVar(&flagGenerateConfig, "generate-config", false, "Print default configuration TOML to stdout and exit")

	rootCmd.SetVersionTemplate(`{{printf "%s version %s\n" .Name .Version}}`)
	rootCmd.AddCommand(squashCmd)
	squashCmd.Flags().StringVar(&flagSquashBase, "base", "", "Squash commits since the merge-base with this branch or commit")

	forcePushCmd.Flags().BoolVar(&flagAutoPush, "auto-push", false, "Force-push without confirmation prompt")
	rootCmd.AddCommand(forcePushCmd)
}
