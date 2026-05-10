package minion

import (
	"fmt"
	"os"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/spf13/cobra"

	"minion/internal/config"
	"minion/internal/store"
)

var upCmd = &cobra.Command{
	Use:   "up [filename|all]",
	Short: "Schedules a minion (or all non-disabled minions) to run in the background",
	Long: `Schedules a minion.
If you pass "all", it will schedule all minions that are not explicitly marked enabled: false in their YAML.

Usage:
  minion up event_finder
  minion up all`,
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		config.LoadEnv()
		target := args[0]
		
		if target != "all" && !strings.HasSuffix(target, ".yaml") && !strings.HasSuffix(target, ".yml") {
			target += ".yaml"
		}

		dbStore, err := store.InitStore(config.DBPath)
		if err != nil {
			fmt.Println(lipgloss.NewStyle().Foreground(lipgloss.Color("9")).Render(fmt.Sprintf("DB Error: %v", err)))
			os.Exit(1)
		}
		defer dbStore.Close()

		minions, err := config.LoadAllMinions()
		if err != nil {
			fmt.Println(lipgloss.NewStyle().Foreground(lipgloss.Color("9")).Render(fmt.Sprintf("Failed to load minions: %v", err)))
			os.Exit(1)
		}

		startedCount := 0
		for _, m := range minions {
			if target == "all" || m.Filename == target || m.Name == target {
				if m.Enabled != nil && !*m.Enabled {
					if target != "all" {
						fmt.Println(lipgloss.NewStyle().Foreground(lipgloss.Color("9")).Render(fmt.Sprintf("This minion is disabled in its YAML. Remove 'enabled: false' to start it.")))
						os.Exit(1)
					}
					continue
				}

					if err := dbStore.SetMinionStatus(m.Filename, true); err != nil {
						fmt.Printf("Error starting %s: %v\n", m.Name, err)
						continue
					}
					startedCount++
					
					displayName := strings.TrimSuffix(m.Filename, ".yaml")
					displayName = strings.TrimSuffix(displayName, ".yml")
					fmt.Println(lipgloss.NewStyle().Foreground(lipgloss.Color("42")).Render(fmt.Sprintf("%s is Up", displayName)))
				}
			}

			if startedCount == 0 {
				if target == "all" {
					fmt.Println(lipgloss.NewStyle().Foreground(lipgloss.Color("240")).Render("No minions found to start."))
				} else {
					displayName := strings.TrimSuffix(target, ".yaml")
					fmt.Println(lipgloss.NewStyle().Foreground(lipgloss.Color("9")).Render(fmt.Sprintf("Minion %s not found.", displayName)))
				}
				os.Exit(1)
			}

			if !isDaemonRunning() {
				runDetached()
			}
		},
}

func init() {
	rootCmd.AddCommand(upCmd)
}