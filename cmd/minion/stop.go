package minion

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/spf13/cobra"

	"minion/internal/config"
	"minion/internal/store"
)

var stopCmd = &cobra.Command{
	Use:   "stop <filename>",
	Short: "Aborts a currently running minion mid-execution",
	Long: `If a minion is actively running (scraping) and you want to cancel it immediately, use this command.
It will safely terminate the minion's browser and stop its execution pipeline.

Usage:
  minion stop event_finder`,
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		config.LoadEnv()

		target := args[0]
		if !strings.HasSuffix(target, ".yaml") && !strings.HasSuffix(target, ".yml") {
			target += ".yaml"
		}

		dbStore, err := store.InitStore(config.DBPath)
		if err != nil {
			fmt.Println(lipgloss.NewStyle().Foreground(lipgloss.Color("9")).Render(fmt.Sprintf("DB Error: %v", err)))
			os.Exit(1)
		}
		defer dbStore.Close()

		m, err := config.LoadMinion(target)
		if err != nil {
			fmt.Println(lipgloss.NewStyle().Foreground(lipgloss.Color("9")).Render(fmt.Sprintf("Minion %s not found.", target)))
			os.Exit(1)
		}

		activeJobs, _ := dbStore.GetActiveJobs()
		if !activeJobs[m.Filename] {
			displayName := strings.TrimSuffix(m.Filename, ".yaml")
			displayName = strings.TrimSuffix(displayName, ".yml")
			fmt.Println(lipgloss.NewStyle().Foreground(lipgloss.Color("240")).Render(fmt.Sprintf("%s is not currently running.", displayName)))
			os.Exit(0)
		}

		if err := dbStore.QueueAbort(m.Filename); err != nil {
			fmt.Println(lipgloss.NewStyle().Foreground(lipgloss.Color("9")).Render(fmt.Sprintf("Failed to stop: %v", err)))
			os.Exit(1)
		}

		displayName := strings.TrimSuffix(m.Filename, ".yaml")
		displayName = strings.TrimSuffix(displayName, ".yml")
		fmt.Print(lipgloss.NewStyle().Foreground(lipgloss.Color("214")).Render(fmt.Sprintf("Stopping %s... ", displayName)))

		// Wait for the job to actually die (max 30s)
		deadline := time.Now().Add(30 * time.Second)
		for time.Now().Before(deadline) {
			jobs, _ := dbStore.GetActiveJobs()
			if !jobs[m.Filename] {
				fmt.Println(lipgloss.NewStyle().Foreground(lipgloss.Color("42")).Render("Stopped."))
				os.Exit(0)
			}
			time.Sleep(200 * time.Millisecond)
		}

		fmt.Println(lipgloss.NewStyle().Foreground(lipgloss.Color("9")).Render("Timed out waiting for minion to stop."))
	},
}

func init() {
	rootCmd.AddCommand(stopCmd)
}