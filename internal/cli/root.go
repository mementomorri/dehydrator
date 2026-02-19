package cli

import (
	"fmt"
	"os"

	"github.com/alexkarsten/dehydrate/internal/config"
	"github.com/alexkarsten/dehydrate/internal/git"
	"github.com/alexkarsten/dehydrate/internal/reporter"
	"github.com/alexkarsten/dehydrate/internal/sidecar"
	"github.com/alexkarsten/dehydrate/pkg/models"
	"github.com/spf13/cobra"
)

var (
	cfgFile    string
	cfg        *models.Config
	sidecarSvc *sidecar.Manager
)

var rootCmd = &cobra.Command{
	Use:   "dehydrate",
	Short: "Semantic Code Compression Engine",
	Long: `DeHydrator is an autonomous code compression utility that optimizes 
and compresses codebases while maintaining 100% functional parity.

It identifies repeating patterns, suggests idiomatic improvements, 
and applies design patterns to reduce cognitive load.`,
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		var err error
		cfg, err = config.Load(cfgFile)
		if err != nil {
			return fmt.Errorf("failed to load config: %w", err)
		}

		if cmd.Name() != "version" && cmd.Name() != "help" {
			sidecarSvc = sidecar.NewManager(cfg)
			if err := sidecarSvc.Start(); err != nil {
				return fmt.Errorf("failed to start AI sidecar: %w", err)
			}
		}

		return nil
	},
	PersistentPostRun: func(cmd *cobra.Command, args []string) {
		if sidecarSvc != nil {
			sidecarSvc.Stop()
		}
	},
}

func init() {
	rootCmd.PersistentFlags().StringVarP(&cfgFile, "config", "c", "", "config file (default is $HOME/.dehydrate.yaml)")
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func checkGitState(path string) error {
	gitMgr := git.NewManager(path)
	clean, err := gitMgr.IsClean()
	if err != nil {
		return fmt.Errorf("failed to check git state: %w", err)
	}

	if !clean {
		fmt.Println("Warning: You have uncommitted changes.")
		fmt.Println("It is recommended to commit or stash your changes before refactoring.")

		if cfg.PreApprove {
			fmt.Println("Proceeding anyway (--yes flag set)...")
			return nil
		}

		fmt.Print("Continue anyway? [y/N]: ")
		var response string
		fmt.Scanln(&response)
		if response != "y" && response != "Y" {
			return fmt.Errorf("aborted by user")
		}
	}

	return nil
}

func runAnalyze(path string) error {
	fmt.Printf("Analyzing repository: %s\n", path)

	if err := checkGitState(path); err != nil {
		return err
	}

	client := sidecarSvc.Client()
	result, err := client.Analyze(path)
	if err != nil {
		return fmt.Errorf("analysis failed: %w", err)
	}

	fmt.Printf("\n=== Analysis Results ===\n")
	fmt.Printf("Total files: %d\n", result.TotalFiles)
	fmt.Printf("Total symbols: %d\n", result.TotalSymbols)
	fmt.Printf("Complexity hotspots: %d\n", len(result.Hotspots))
	fmt.Printf("Potential duplicates: %d\n", len(result.Duplicates))

	if len(result.Hotspots) > 0 {
		fmt.Printf("\n--- Complexity Hotspots ---\n")
		for _, hs := range result.Hotspots {
			fmt.Printf("  %s:%d - %s (CC: %d)\n", hs.File, hs.Line, hs.Symbol, hs.CyclomaticComplexity)
		}
	}

	if len(result.Duplicates) > 0 {
		fmt.Printf("\n--- Potential Duplicates ---\n")
		for _, dup := range result.Duplicates {
			fmt.Printf("  Similarity: %.2f%%\n", dup.Similarity*100)
			for _, block := range dup.Blocks {
				fmt.Printf("    - %s:%d-%d\n", block.File, block.StartLine, block.EndLine)
			}
		}
	}

	return nil
}

func runDeduplicate(path string, commitChanges bool) error {
	fmt.Printf("Running deduplication: %s\n", path)

	if err := checkGitState(path); err != nil {
		return err
	}

	client := sidecarSvc.Client()
	plan, err := client.Deduplicate(path)
	if err != nil {
		return fmt.Errorf("deduplication planning failed: %w", err)
	}

	fmt.Printf("\n=== Refactoring Plan ===\n")
	fmt.Printf("%s\n", plan.Description)
	fmt.Printf("\nChanges to be made:\n")
	for i, change := range plan.Changes {
		fmt.Printf("\n%d. %s\n", i+1, change.Path)
		fmt.Printf("   %s\n", change.Description)
	}

	if !cfg.PreApprove {
		fmt.Printf("\nApply these changes? [y/N]: ")
		var response string
		fmt.Scanln(&response)
		if response != "y" && response != "Y" {
			return fmt.Errorf("aborted by user")
		}
	}

	result, err := client.ApplyPlan(plan.SessionID)
	if err != nil {
		return fmt.Errorf("failed to apply changes: %w", err)
	}

	if !result.TestsPassed {
		fmt.Println("\nWarning: Tests failed after refactoring!")
		fmt.Println("Rolling back changes...")

		gitMgr := git.NewManager(path)
		if err := gitMgr.Rollback(); err != nil {
			return fmt.Errorf("rollback failed: %w", err)
		}

		return fmt.Errorf("refactoring aborted due to test failures")
	}

	fmt.Println("\nRefactoring completed successfully!")
	fmt.Printf("LOC reduced: %d\n", result.MetricsBefore.LinesOfCode-result.MetricsAfter.LinesOfCode)

	if commitChanges {
		gitMgr := git.NewManager(path)
		if err := gitMgr.Commit("refactor: deduplicate code", result.Changes); err != nil {
			return fmt.Errorf("failed to commit changes: %w", err)
		}
		fmt.Println("Changes committed to git.")
	}

	if cfg.Report {
		rep := reporter.New(cfg)
		if err := rep.Generate(result); err != nil {
			return fmt.Errorf("failed to generate report: %w", err)
		}
		fmt.Println("Report generated: dehydrate-report.md")
	}

	return nil
}

func runIdiomatize(path string) error {
	fmt.Printf("Running idiomatization: %s\n", path)

	if err := checkGitState(path); err != nil {
		return err
	}

	client := sidecarSvc.Client()
	plan, err := client.Idiomatize(path)
	if err != nil {
		return fmt.Errorf("idiomatization planning failed: %w", err)
	}

	fmt.Printf("\n=== Idiomatization Plan ===\n")
	fmt.Printf("%s\n", plan.Description)
	fmt.Printf("\nChanges to be made:\n")
	for i, change := range plan.Changes {
		fmt.Printf("\n%d. %s\n", i+1, change.Path)
		fmt.Printf("   %s\n", change.Description)
	}

	if !cfg.PreApprove {
		fmt.Printf("\nApply these changes? [y/N]: ")
		var response string
		fmt.Scanln(&response)
		if response != "y" && response != "Y" {
			return fmt.Errorf("aborted by user")
		}
	}

	_, err = client.ApplyPlan(plan.SessionID)
	if err != nil {
		return fmt.Errorf("failed to apply changes: %w", err)
	}

	fmt.Println("\nIdiomatization completed successfully!")
	return nil
}

func runPattern(pattern, path string) error {
	fmt.Printf("Applying design pattern: %s\n", pattern)
	fmt.Printf("Path: %s\n", path)

	if err := checkGitState(path); err != nil {
		return err
	}

	client := sidecarSvc.Client()
	plan, err := client.ApplyPattern(pattern, path)
	if err != nil {
		return fmt.Errorf("pattern injection failed: %w", err)
	}

	fmt.Printf("\n=== Pattern Injection Plan ===\n")
	fmt.Printf("Pattern: %s\n", plan.Pattern)
	fmt.Printf("%s\n", plan.Description)
	fmt.Printf("\nChanges to be made:\n")
	for i, change := range plan.Changes {
		fmt.Printf("\n%d. %s\n", i+1, change.Path)
		fmt.Printf("   %s\n", change.Description)
	}

	if !cfg.PreApprove {
		fmt.Printf("\nApply these changes? [y/N]: ")
		var response string
		fmt.Scanln(&response)
		if response != "y" && response != "Y" {
			return fmt.Errorf("aborted by user")
		}
	}

	_, err = client.ApplyPlan(plan.SessionID)
	if err != nil {
		return fmt.Errorf("failed to apply changes: %w", err)
	}

	fmt.Println("\nPattern injection completed successfully!")
	return nil
}

func runReport(sessionID string) error {
	rep := reporter.New(cfg)
	return rep.Load(sessionID)
}

var analyzeCmd = &cobra.Command{
	Use:   "analyze [path]",
	Short: "Analyze repository for compression opportunities",
	Long: `Scans the repository to identify:
- Code duplication patterns
- Complexity hotspots
- Non-idiomatic code sections
- Opportunities for design pattern injection`,
	Args: cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		path := "."
		if len(args) > 0 {
			path = args[0]
		}

		return runAnalyze(path)
	},
}

var deduplicateCmd = &cobra.Command{
	Use:   "deduplicate [path]",
	Short: "Find and eliminate code duplication",
	Long: `Identifies semantically similar code blocks across the repository
and suggests or applies refactoring to eliminate duplication.`,
	Args: cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		path := "."
		if len(args) > 0 {
			path = args[0]
		}

		preApprove, _ := cmd.Flags().GetBool("yes")
		cfg.PreApprove = preApprove

		commitChanges, _ := cmd.Flags().GetBool("commit")

		return runDeduplicate(path, commitChanges)
	},
}

var idiomatizeCmd = &cobra.Command{
	Use:   "idiomatize [path]",
	Short: "Transform code to idiomatic patterns",
	Long: `Identifies non-idiomatic code patterns and suggests
or applies transformations to make code more idiomatic.`,
	Args: cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		path := "."
		if len(args) > 0 {
			path = args[0]
		}

		preApprove, _ := cmd.Flags().GetBool("yes")
		cfg.PreApprove = preApprove

		return runIdiomatize(path)
	},
}

var patternCmd = &cobra.Command{
	Use:   "pattern [pattern] [path]",
	Short: "Apply a design pattern to simplify code",
	Long: `Identifies opportunities for design pattern injection
(such as Factory, Strategy, Observer) and applies refactoring.`,
	Args: cobra.MaximumNArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		pattern := ""
		path := "."

		if len(args) > 0 {
			pattern = args[0]
		}
		if len(args) > 1 {
			path = args[1]
		}

		preApprove, _ := cmd.Flags().GetBool("yes")
		cfg.PreApprove = preApprove

		return runPattern(pattern, path)
	},
}

var reportCmd = &cobra.Command{
	Use:   "report",
	Short: "Generate a compression report",
	Long:  `Generates a detailed report of the last compression session.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		sessionID, _ := cmd.Flags().GetString("session")
		return runReport(sessionID)
	},
}

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print the version number",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("DeHydrator v0.1.0")
	},
}

func initCommands() {
	analyzeCmd.Flags().Bool("report", false, "generate report after analysis")
	deduplicateCmd.Flags().BoolP("yes", "y", false, "skip approval and apply changes automatically")
	deduplicateCmd.Flags().Bool("commit", false, "commit changes to git after successful refactoring")
	idiomatizeCmd.Flags().BoolP("yes", "y", false, "skip approval and apply changes automatically")
	patternCmd.Flags().BoolP("yes", "y", false, "skip approval and apply changes automatically")
	reportCmd.Flags().StringP("session", "s", "", "session ID to report (default: last session)")

	rootCmd.AddCommand(analyzeCmd)
	rootCmd.AddCommand(deduplicateCmd)
	rootCmd.AddCommand(idiomatizeCmd)
	rootCmd.AddCommand(patternCmd)
	rootCmd.AddCommand(reportCmd)
	rootCmd.AddCommand(versionCmd)
}

func init() {
	initCommands()
}
