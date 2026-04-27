package minion

import (
	"fmt"
	"os"

	"github.com/charmbracelet/lipgloss"
	"github.com/spf13/cobra"

	"minion/internal/config"
	"minion/internal/store"
)

var clearAll bool

var clearCmd = &cobra.Command{
	Use:   "clear <minion_name> | --all",
	Short: "Clears DB memory (Usage: minion clear <name> OR minion clear --all)",
	Run: func(cmd *cobra.Command, args []string) {
		config.LoadEnv()

		if !clearAll && len(args) != 1 {
			fmt.Println(lipgloss.NewStyle().Foreground(lipgloss.Color("9")).Render("Error: You must provide a minion name or use the --all flag."))
			cmd.Usage()
			os.Exit(1)
		}

		dbStore, err := store.InitStore(config.DBPath)
		if err != nil {
			fmt.Println(lipgloss.NewStyle().Foreground(lipgloss.Color("9")).Render(fmt.Sprintf("DB Error: %v", err)))
			os.Exit(1)
		}
		defer dbStore.Close()

		if clearAll {
			deleted, err := dbStore.ClearAllState()
			if err != nil {
				fmt.Println(lipgloss.NewStyle().Foreground(lipgloss.Color("9")).Render(fmt.Sprintf("Failed to clear database: %v", err)))
				os.Exit(1)
			}
			fmt.Printf("%s\n", lipgloss.NewStyle().Foreground(lipgloss.Color("42")).Render(fmt.Sprintf("Cleared all memory. Deleted %d records across all minions.", deleted)))
			return
		}

		minionName := args[0]
		deleted, err := dbStore.ClearMinionState(minionName)
		if err != nil {
			fmt.Println(lipgloss.NewStyle().Foreground(lipgloss.Color("9")).Render(fmt.Sprintf("Failed to clear state for %s: %v", minionName, err)))
			os.Exit(1)
		}
		fmt.Printf("%s\n", lipgloss.NewStyle().Foreground(lipgloss.Color("42")).Render(fmt.Sprintf("Cleared memory for '%s'. Deleted %d records.", minionName, deleted)))
	},
}

func init() {
	clearCmd.Flags().BoolVarP(&clearAll, "all", "a", false, "Clear memory for ALL minions")
	rootCmd.AddCommand(clearCmd)
}
