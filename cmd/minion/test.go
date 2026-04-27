package minion

import (
	"context"
	"fmt"
	"os"

	"github.com/charmbracelet/lipgloss"
	"github.com/spf13/cobra"

	"minion/internal/config"
	"minion/internal/engine"
	"minion/internal/llm"
	"minion/internal/store"
)

var testCmd = &cobra.Command{
	Use:   "test <filename>",
	Short: "Instantly runs a minion (Usage: minion test <filename>)",
	Long: `Tests a specific minion immediately, skipping its cron schedule.
Useful for debugging your YAML instructions and ensuring the AI finds the right matches.

Usage:
  minion test event_finder
  minion test my_custom_task`,
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		config.LoadEnv()
		filename := args[0]

		m, err := config.LoadMinion(filename)
		if err != nil {
			fmt.Println(lipgloss.NewStyle().Foreground(lipgloss.Color("9")).Render(fmt.Sprintf("Error loading minion: %v", err)))
			os.Exit(1)
		}

		runTest(m)
	},
}

func init() {
	rootCmd.AddCommand(testCmd)
}

func runTest(m *config.MinionConfig) {
	if m.Enabled != nil && !*m.Enabled {
		fmt.Printf("\n%s\nRunning test anyway...\n", lipgloss.NewStyle().Foreground(lipgloss.Color("214")).Render("Warning: This minion is marked as disabled (enabled: false)."))
	}

	fmt.Printf("\nTesting Minion: %s\n\n", lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("39")).Render(m.Name))

	dbStore, err := store.InitStore(config.DBPath)
	if err != nil {
		fmt.Println(lipgloss.NewStyle().Foreground(lipgloss.Color("9")).Render(fmt.Sprintf("DB Error: %v", err)))
		os.Exit(1)
	}
	defer dbStore.Close()

	llmEval, err := llm.NewEvaluator()
	if err != nil {
		fmt.Println(lipgloss.NewStyle().Foreground(lipgloss.Color("9")).Render(fmt.Sprintf("LLM Init Error: %v", err)))
		os.Exit(1)
	}

	stepStyle := lipgloss.NewStyle().Bold(true).Width(15).Align(lipgloss.Right).MarginRight(2)
	okColor := lipgloss.Color("42")
	warnColor := lipgloss.Color("214")
	errColor := lipgloss.Color("9")
	infoColor := lipgloss.Color("39")
	neutralColor := lipgloss.Color("240")

	runCtx := &engine.RunContext{
		Store: dbStore,
		LLM:   llmEval,
		OnStep: func(step, details string, isError bool) {
			color := neutralColor 

			switch step {
			case "DONE", "MATCH", "WEBHOOK", "ITEM":
				color = okColor 
			case "CACHED", "DEDUPE", "FILTERED", "SKIPPED":
				color = warnColor 
			case "FIREWALL", "DISCARDED", "NO MATCH":
				color = infoColor 
			case "ERROR", "SEARCH ERROR", "BROWSE ERROR", "SCRAPE ERROR", "STUDY ERROR", "STORE ERROR", "WEBHOOK ERROR":
				color = errColor 
			}

			s := stepStyle.Foreground(color).Render("[" + step + "]")
			
			detailStyle := lipgloss.NewStyle()
			if isError {
				detailStyle = detailStyle.Foreground(errColor)
			} else if color == neutralColor {
				detailStyle = detailStyle.Foreground(lipgloss.Color("245"))
			}

			fmt.Printf("%s %s\n", s, detailStyle.Render(details))
		},
	}

	ctx := context.Background()
	
	if err := engine.RunMission(ctx, m, runCtx); err != nil {
		fmt.Printf("\nPipeline Error: %v\n", err)
	}

	fmt.Println()
	boxStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("39")).
		Padding(1, 4)

	fmt.Println(boxStyle.Render(runCtx.Stats.GenerateReport(m.Name)))
	fmt.Println()
}
