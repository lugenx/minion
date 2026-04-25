package minion

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"syscall"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"
	"github.com/robfig/cron/v3"
	"github.com/spf13/cobra"

	"minion/internal/config"
	"minion/internal/engine"
	"minion/internal/llm"
	"minion/internal/store"
)

var detached bool

var runCmd = &cobra.Command{
	Use:   "run",
	Short: "Starts the daemon and runs minions on their schedules",
	Long: `Starts the daemon and runs all active minions according to their cron schedules.
By default, it runs in the foreground. Use the -d or --detach flag to run it silently in the background.

Usage:
  minion run
  minion run -d`,
	Args:  cobra.NoArgs,
	Run: func(cmd *cobra.Command, args []string) {
		config.LoadEnv()

		if detached {
			runDetached()
			return
		}

		runDaemon()
	},
}

func init() {
	runCmd.Flags().BoolVarP(&detached, "detach", "d", false, "Run daemon in the background")
	rootCmd.AddCommand(runCmd)
}

func runDetached() {
	if _, err := os.Stat(config.PIDPath); err == nil {
		fmt.Println(lipgloss.NewStyle().Foreground(lipgloss.Color("9")).Render("Daemon is already running. Use 'minion stop' to stop it first."))
		os.Exit(1)
	}

	exe, err := os.Executable()
	if err != nil {
		fmt.Printf("Failed to get executable path: %v\n", err)
		os.Exit(1)
	}

	cmd := exec.Command(exe, "run") 
	
	logFile, err := os.OpenFile(config.LogPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
	if err != nil {
		fmt.Printf("Failed to open log file: %v\n", err)
		os.Exit(1)
	}
	cmd.Stdout = logFile
	cmd.Stderr = logFile

	if err := cmd.Start(); err != nil {
		fmt.Printf("Failed to start daemon: %v\n", err)
		os.Exit(1)
	}

	if err := os.WriteFile(config.PIDPath, []byte(fmt.Sprintf("%d", cmd.Process.Pid)), 0644); err != nil {
		fmt.Printf("Warning: Failed to write PID file: %v\n", err)
	}

	activeCount := 0
	minions, _ := config.LoadAllMinions()
	for _, m := range minions {
		if m.Enabled == nil || *m.Enabled {
			activeCount++
		}
	}

	successStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("42")).Bold(true)
	word := "minion"
	if activeCount != 1 {
		word = "minions"
	}
	fmt.Printf("%s\n", successStyle.Render(fmt.Sprintf("Successfully deployed %d running %s in the background.", activeCount, word)))
	fmt.Printf("Logs: tail -f %s\n", config.LogPath)
	fmt.Println("To stop: minion stop")
}

func logMessage(level, minionName, msg string) {
	colorRenderer := lipgloss.NewRenderer(os.Stdout)
	colorRenderer.SetColorProfile(termenv.TrueColor)

	timeStr := colorRenderer.NewStyle().Foreground(lipgloss.Color("240")).Render(time.Now().Format("2006-01-02 15:04:05"))
	
	levelStyle := colorRenderer.NewStyle().Bold(true)
	switch level {
	case "INFO":
		levelStyle = levelStyle.Foreground(lipgloss.Color("39"))
	case "WARN":
		levelStyle = levelStyle.Foreground(lipgloss.Color("214"))
	case "ERROR":
		levelStyle = levelStyle.Foreground(lipgloss.Color("9"))
	case "SUCCESS":
		levelStyle = levelStyle.Foreground(lipgloss.Color("42"))
	}

	nameStyle := colorRenderer.NewStyle().Foreground(lipgloss.Color("250"))
	
	fmt.Printf("%s %s %s: %s\n", 
		timeStr, 
		levelStyle.Render(fmt.Sprintf("[%s]", level)), 
		nameStyle.Render(fmt.Sprintf("(%s)", minionName)), 
		msg,
	)
}

func runDaemon() {
	timeStr := lipgloss.NewStyle().Foreground(lipgloss.Color("240")).Render(time.Now().Format("2006-01-02 15:04:05"))
	fmt.Printf("%s %s\n", timeStr, lipgloss.NewStyle().Foreground(lipgloss.Color("39")).Bold(true).Render("Starting task engine..."))

	minions, err := config.LoadAllMinions()
	if err != nil {
		fmt.Printf("Failed to load minions: %v\n", err)
		os.Exit(1)
	}

	if len(minions) == 0 {
		fmt.Printf("No minions found in %s\n", config.MinionsDir)
		return
	}

	dbStore, err := store.InitStore(config.DBPath)
	if err != nil {
		fmt.Printf("Failed to initialize store: %v\n", err)
		os.Exit(1)
	}
	defer dbStore.Close()
	
	// Clear any ghost jobs left over from a previous crash
	_ = dbStore.ClearActiveJobs()

	llmEval, err := llm.NewEvaluator()
	if err != nil {
		fmt.Printf("Failed to initialize LLM evaluator: %v\n", err)
		os.Exit(1)
	}

	c := cron.New()

	for _, m := range minions {
		if m.Enabled != nil && !*m.Enabled {
			logMessage("WARN", m.Name, "Skipping (marked disabled)")
			continue
		}

		sched := engine.ExtractSchedule(m)
		cronExpr, err := engine.ParseToCron(sched)
		if err != nil {
			logMessage("ERROR", m.Name, fmt.Sprintf("Failed to parse schedule: %v", err))
			continue
		}

		currentMinion := m
		
		_, err = c.AddFunc(cronExpr, func() {
			logMessage("INFO", currentMinion.Name, "Running scheduled task...")
			
			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Hour)
			defer cancel()
			
			runCtx := &engine.RunContext{
				Store: dbStore,
				LLM:   llmEval,
				OnStep: func(step, details string, isError bool) {
					if isError {
						logMessage("ERROR", currentMinion.Name, fmt.Sprintf("%s: %s", step, details))
					} else if step == "MATCH" || step == "WEBHOOK" {
						logMessage("SUCCESS", currentMinion.Name, details)
					}
				}, 
			}

			if err := engine.RunMission(ctx, currentMinion, runCtx); err != nil {
				logMessage("ERROR", currentMinion.Name, fmt.Sprintf("Failed: %v", err))
			} else {
				statsMsg := fmt.Sprintf("Finished in %s (Scraped: %d, Cached: %d, Found: %d, Delivered: %d, Errors: %d)",
					runCtx.Stats.Duration().Round(time.Millisecond*100),
					runCtx.Stats.PagesScraped,
					runCtx.Stats.PagesCached,
					runCtx.Stats.ItemsFound,
					runCtx.Stats.ItemsDelivered,
					runCtx.Stats.Errors,
				)
				logMessage("INFO", currentMinion.Name, statsMsg)
			}
		})

		if err != nil {
			logMessage("ERROR", m.Name, fmt.Sprintf("Failed to schedule: %v", err))
		} else {
			logMessage("INFO", m.Name, fmt.Sprintf("Scheduled for %s -> %s", sched, cronExpr))
		}
	}

	c.Start()
	fmt.Printf("%s %s\n", timeStr, lipgloss.NewStyle().Foreground(lipgloss.Color("42")).Bold(true).Render("Daemon is running. Press Ctrl+C to stop."))

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	<-sigChan

	fmt.Println()
	fmt.Printf("%s %s\n", lipgloss.NewStyle().Foreground(lipgloss.Color("240")).Render(time.Now().Format("2006-01-02 15:04:05")), lipgloss.NewStyle().Foreground(lipgloss.Color("214")).Render("Shutting down..."))
	c.Stop()
	
	if !detached {
		_ = os.Remove(config.PIDPath)
	}
}
