package minion

import (
	"fmt"
	"os"

	"github.com/charmbracelet/lipgloss"
	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:     "minion",
	Short:   "Minion is an AI web monitoring agent",
	Version: "2.1.5",
}

func init() {
	titleStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("39"))
	subStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("214"))
	textStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("250"))
	pathStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("42"))
	highlightStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("13")).Bold(true)

	rootCmd.Long = fmt.Sprintf(`%s

%s
It scrapes targeted websites or performs web searches, filters out junk using fast string matching,
evaluates the survivors using an LLM via OpenRouter, and sends matches to a webhook topic.

%s
  Config Folder: %s
  Secrets File:  %s
  Task Files:    %s

%s
  1. Add your API key:    %s
  2. Create a task:       %s
  3. Test it instantly:   %s
  4. Run the daemon:      %s`,
		titleStyle.Render("Minion: AI Web Monitoring Agent"),
		textStyle.Render(`Minion reads simple YAML files (representing "minions").`),
		subStyle.Render("Directory Structure"),
		pathStyle.Render("~/.config/minion/"),
		pathStyle.Render("~/.config/minion/.env"),
		pathStyle.Render("~/.config/minion/minions/*.yaml"),
		subStyle.Render("Quick Start"),
		highlightStyle.Render("nano ~/.config/minion/.env"),
		highlightStyle.Render("nano ~/.config/minion/minions/my_task.yaml"),
		highlightStyle.Render("minion test my_task"),
		highlightStyle.Render("minion run -d"),
	)
}

func Execute() {
	rootCmd.CompletionOptions.DisableDefaultCmd = true

	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
