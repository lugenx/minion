package minion

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/robfig/cron/v3"
	"github.com/spf13/cobra"

	"minion/internal/config"
	"minion/internal/engine"
	"minion/internal/store"
)

var showAll bool

var listCmd = &cobra.Command{
	Use:     "list",
	Aliases: []string{"ls"},
	Short:   "Prints all configured minions, their state, and next scheduled run times",
	Args:    cobra.NoArgs,
	Run: func(cmd *cobra.Command, args []string) {
		config.LoadEnv()
		runList()
	},
}

func init() {
	listCmd.Flags().BoolVarP(&showAll, "all", "a", false, "Show all minions, including disabled ones")
	rootCmd.AddCommand(listCmd)
}

func runList() {
	allMinions, err := config.LoadAllMinions()
	if err != nil {
		fmt.Printf("Error loading minions: %v\n", err)
		os.Exit(1)
	}

	var minions []*config.MinionConfig
	for _, m := range allMinions {
		isDraft := m.Enabled != nil && !*m.Enabled
		if !isDraft || showAll {
			minions = append(minions, m)
		}
	}

	if len(minions) == 0 {
		if !showAll && len(allMinions) > 0 {
			fmt.Printf("No active minions found. (Use `minion ls -a` to see %d disabled minions)\n", len(allMinions))
		} else {
			fmt.Printf("No minions found in %s\n", config.MinionsDir)
		}
		return
	}

	parser := cron.NewParser(cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow | cron.Descriptor)
	now := time.Now()

	dbStore, _ := store.InitStore(config.DBPath)
	activeJobs := make(map[string]bool)
	queueMap := make(map[string]bool)
	if dbStore != nil {
		activeJobs, _ = dbStore.GetActiveJobs()
		queue, _ := dbStore.GetRunQueue()
		for _, q := range queue {
			queueMap[q] = true
		}
	}

	headerStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("250"))
	executingStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("214")) // Orange for scraping/queued
	scheduledStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("42")) // Green for UP
	stoppedStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("240")) // Grey for DOWN
	idleStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("240")) // Grey for Draft
	nameStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("39"))

	nameWidth := 6
	stateWidth := 5
	schedWidth := 8
	nextRunWidth := 8
	fileWidth := 4

	for _, m := range minions {
		if len(m.Name) > nameWidth {
			nameWidth = len(m.Name)
		}
		
		sched := engine.ExtractSchedule(m)
		if len(sched) > schedWidth {
			schedWidth = len(sched)
		}
		
		isDraft := m.Enabled != nil && !*m.Enabled
		isActive := false
		if dbStore != nil {
			isActive = dbStore.GetMinionStatus(m.Filename)
		}
		isRunning := activeJobs[m.Filename]
		isQueued := queueMap[m.Filename]

		if isRunning {
			if len("Running") > stateWidth {
				stateWidth = len("Running")
			}
		} else if isQueued {
			if len("Queued") > stateWidth {
				stateWidth = len("Queued")
			}
		} else if isDraft {
			if len("Disabled") > stateWidth {
				stateWidth = len("Disabled")
			}
		} else if isActive {
			if len("Up") > stateWidth {
				stateWidth = len("Up")
			}
		} else {
			if len("Down") > stateWidth {
				stateWidth = len("Down")
			}
		}

		if !isDraft && isActive {
			cronExpr, err := engine.ParseToCron(sched)
			if err == nil {
				s, parseErr := parser.Parse(cronExpr)
				if parseErr == nil {
					nextStr := s.Next(now).Format(time.RFC822)
					if len(nextStr) > nextRunWidth {
						nextRunWidth = len(nextStr)
					}
				}
			}
		} else {
			if len("-") > nextRunWidth {
				nextRunWidth = len("-")
			}
		}

		displayFile := strings.TrimSuffix(m.Filename, ".yaml")
		displayFile = strings.TrimSuffix(displayFile, ".yml")
		if len(displayFile) > fileWidth {
			fileWidth = len(displayFile)
		}
	}

	nameWidth += 4
	stateWidth += 4
	schedWidth += 4
	nextRunWidth += 4

	pad := func(s string, width int) string {
		padding := width - len(s)
		if padding < 0 {
			padding = 0
		}
		return s + strings.Repeat(" ", padding)
	}

	fmt.Println()
	fmt.Printf("  %s%s%s%s%s\n",
		headerStyle.Render(pad("MINION", nameWidth)),
		headerStyle.Render(pad("STATE", stateWidth)),
		headerStyle.Render(pad("SCHEDULE", schedWidth)),
		headerStyle.Render(pad("NEXT RUN", nextRunWidth)),
		headerStyle.Render("FILE"),
	)
	
	fmt.Printf("  %s%s%s%s%s\n",
		headerStyle.Render(pad(strings.Repeat("-", 6), nameWidth)),
		headerStyle.Render(pad(strings.Repeat("-", 5), stateWidth)),
		headerStyle.Render(pad(strings.Repeat("-", 8), schedWidth)),
		headerStyle.Render(pad(strings.Repeat("-", 8), nextRunWidth)),
		headerStyle.Render(strings.Repeat("-", 4)),
	)

	for _, m := range minions {
		isDraft := m.Enabled != nil && !*m.Enabled
		isActive := false
		if dbStore != nil {
			isActive = dbStore.GetMinionStatus(m.Filename)
		}
		isRunning := activeJobs[m.Filename]
		isQueued := queueMap[m.Filename]

		stateText := ""
		var stateRender string

		if isDraft {
			stateText = "Disabled"
			stateRender = idleStyle.Render(stateText)
		} else if isRunning {
			stateText = "Running"
			stateRender = executingStyle.Render(stateText)
		} else if isQueued {
			stateText = "Queued"
			stateRender = executingStyle.Render(stateText)
		} else if isActive {
			stateText = "Up"
			stateRender = scheduledStyle.Render(stateText)
		} else {
			stateText = "Down"
			stateRender = stoppedStyle.Render(stateText)
		}
		
		// Unstyled length calculation for perfect padding
		// Fix because "▶" is 3 bytes but 1 rune width in some terminals, but let's use runes
		runePadding := stateWidth - len([]rune(stateText))
		if runePadding < 0 { runePadding = 0 }
		statePadded := stateRender + strings.Repeat(" ", runePadding)

		sched := engine.ExtractSchedule(m)
		cronExpr, err := engine.ParseToCron(sched)
		nextRunText := ""
		nextRunRender := ""
		
		if isDraft {
			nextRunText = "-"
			nextRunRender = idleStyle.Render(nextRunText)
		} else if !isActive {
			nextRunText = "-"
			nextRunRender = stoppedStyle.Render(nextRunText)
		} else if err == nil {
			s, parseErr := parser.Parse(cronExpr)
			if parseErr == nil {
				nextRunText = s.Next(now).Format(time.RFC822)
				nextRunRender = nextRunText
			} else {
				nextRunText = "Cron Parse Error"
				nextRunRender = stoppedStyle.Render(nextRunText)
			}
		} else {
			nextRunText = "Invalid Schedule"
			nextRunRender = stoppedStyle.Render(nextRunText)
		}

		nextPadding := nextRunWidth - len(nextRunText)
		if nextPadding < 0 {
			nextPadding = 0
		}
		nextRunPadded := nextRunRender + strings.Repeat(" ", nextPadding)

		displayFile := strings.TrimSuffix(m.Filename, ".yaml")
		displayFile = strings.TrimSuffix(displayFile, ".yml")

		fmt.Printf("  %s%s%s%s%s\n",
			nameStyle.Render(pad(m.Name, nameWidth)),
			statePadded,
			pad(sched, schedWidth),
			nextRunPadded,
			nameStyle.Render(displayFile),
		)
	}
	
	if dbStore != nil {
		dbStore.Close()
	}
	fmt.Println()
}
