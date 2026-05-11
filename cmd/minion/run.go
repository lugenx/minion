package minion

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
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
	Use:   "run [filename]",
	Short: "Queue a minion to run immediately, or start the master daemon",
	Long: `If no filename is provided, starts the master daemon in the background or foreground.
If a filename is provided, it instantly queues the minion to be executed by the master daemon.`,
	Args:  cobra.MaximumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		config.LoadEnv()

		if len(args) == 1 {
			filename := args[0]
			queueMinionRun(filename)
			return
		}

		checkPIDLock()

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

func checkPIDLock() {
	if pidBytes, err := os.ReadFile(config.PIDPath); err == nil {
		pidStr := strings.TrimSpace(string(pidBytes))
		if pid, err := strconv.Atoi(pidStr); err == nil {
			if process, err := os.FindProcess(pid); err == nil {
				// On Unix, FindProcess always succeeds. Sending signal 0 checks if it's alive.
				if err := process.Signal(syscall.Signal(0)); err == nil {
					fmt.Println(lipgloss.NewStyle().Foreground(lipgloss.Color("9")).Render("Daemon is already running."))
					os.Exit(1)
				}
			}
		}
	}
}

func isDaemonRunning() bool {
	if pidBytes, err := os.ReadFile(config.PIDPath); err == nil {
		pidStr := strings.TrimSpace(string(pidBytes))
		if pid, err := strconv.Atoi(pidStr); err == nil {
			if process, err := os.FindProcess(pid); err == nil {
				if err := process.Signal(syscall.Signal(0)); err == nil {
					return true
				}
			}
		}
	}
	return false
}

func runDetached() {
	exe, err := os.Executable()
	if err != nil {
		fmt.Printf("Failed to get executable path: %v\n", err)
		os.Exit(1)
	}

	cmd := exec.Command(exe, "run") 
	setSysProcAttr(cmd)
	
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

	// Wait up to 2 seconds for the background process to write the PID file
	for i := 0; i < 20; i++ {
		if _, err := os.Stat(config.PIDPath); err == nil {
			break
		}
		time.Sleep(100 * time.Millisecond)
	}
}

func queueMinionRun(filename string) {
	if !strings.HasSuffix(filename, ".yaml") && !strings.HasSuffix(filename, ".yml") {
		filename += ".yaml"
	}

	m, err := config.LoadMinion(filename)
	if err != nil {
		fmt.Println(lipgloss.NewStyle().Foreground(lipgloss.Color("9")).Render(fmt.Sprintf("Error loading minion: %v", err)))
		os.Exit(1)
	}

	if m.Enabled != nil && !*m.Enabled {
		fmt.Println(lipgloss.NewStyle().Foreground(lipgloss.Color("9")).Render(fmt.Sprintf("This minion is disabled in its YAML. Remove 'enabled: false' to run it.")))
		os.Exit(1)
	}

	dbStore, err := store.InitStore(config.DBPath)
	if err != nil {
		fmt.Println(lipgloss.NewStyle().Foreground(lipgloss.Color("9")).Render(fmt.Sprintf("DB Error: %v", err)))
		os.Exit(1)
	}
	defer dbStore.Close()

	if err := dbStore.QueueRun(m.Filename); err != nil {
		fmt.Println(lipgloss.NewStyle().Foreground(lipgloss.Color("9")).Render(fmt.Sprintf("Failed to queue run: %v", err)))
		os.Exit(1)
	}

	successStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("42")).Bold(true)
	
	displayName := strings.TrimSuffix(m.Filename, ".yaml")
	displayName = strings.TrimSuffix(displayName, ".yml")
	fmt.Printf("%s\n", successStyle.Render(fmt.Sprintf("Running %s...", displayName)))
	
	if !isDaemonRunning() {
		runDetached()
	}
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
	if err := os.WriteFile(config.PIDPath, []byte(fmt.Sprintf("%d", os.Getpid())), 0644); err != nil {
		fmt.Printf("Warning: Failed to write PID file: %v\n", err)
	}
	defer os.Remove(config.PIDPath)

	timeStr := lipgloss.NewStyle().Foreground(lipgloss.Color("240")).Render(time.Now().Format("2006-01-02 15:04:05"))
	fmt.Printf("%s %s\n", timeStr, lipgloss.NewStyle().Foreground(lipgloss.Color("39")).Bold(true).Render("Starting master task engine..."))

	_ = os.MkdirAll(config.LogsDir, 0755)

	dbStore, err := store.InitStore(config.DBPath)
	if err != nil {
		fmt.Printf("Failed to initialize store: %v\n", err)
		os.Exit(1)
	}
	defer dbStore.Close()
	
	_ = dbStore.ClearActiveJobs()

	llmEval, err := llm.NewEvaluator()
	if err != nil {
		fmt.Printf("Failed to initialize LLM evaluator: %v\n", err)
		os.Exit(1)
	}

	daemonCtx, daemonCancel := context.WithCancel(context.Background())
	defer daemonCancel()

	// 1-second Loop
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// Keep track of internal cron schedules
	c := cron.New()
	c.Start()
	defer c.Stop()

	// Track what we have scheduled to avoid duplicates
	scheduledJobs := make(map[string]cron.EntryID)

	activeCancels := &sync.Map{}

	for {
		select {
		case <-ticker.C:
			syncFleet(daemonCtx, dbStore, llmEval, c, scheduledJobs, activeCancels)
			processRunQueue(daemonCtx, dbStore, llmEval, activeCancels)
			processAbortQueue(dbStore, activeCancels)
			
			if active, _ := dbStore.GetActiveMinions(); len(active) == 0 {
				return
			}
		case <-sigChan:
			fmt.Println()
			fmt.Printf("%s %s\n", lipgloss.NewStyle().Foreground(lipgloss.Color("240")).Render(time.Now().Format("2006-01-02 15:04:05")), lipgloss.NewStyle().Foreground(lipgloss.Color("214")).Render("Shutting down... waiting for running tasks to finish"))
			return
		}
	}
}

func syncFleet(ctx context.Context, dbStore *store.Store, llmEval *llm.Evaluator, c *cron.Cron, scheduledJobs map[string]cron.EntryID, activeCancels *sync.Map) {
	minions, err := config.LoadAllMinions()
	if err != nil {
		return
	}

	// Build map of currently desired active minions
	desired := make(map[string]*config.MinionConfig)
	for _, m := range minions {
		if m.Enabled != nil && !*m.Enabled {
			continue // Drafts are ignored completely
		}
		if dbStore.GetMinionStatus(m.Filename) {
			desired[m.Filename] = m
		}
	}

	// 1. Remove jobs that are no longer desired or got disabled
	for filename, entryID := range scheduledJobs {
		if _, ok := desired[filename]; !ok {
			c.Remove(entryID)
			delete(scheduledJobs, filename)
			logMessage("INFO", filename, "Unscheduled minion.")
		}
	}

	// 2. Add jobs that are desired but not scheduled yet
	for filename, m := range desired {
		if _, ok := scheduledJobs[filename]; !ok {
			sched := engine.ExtractSchedule(m)
			cronExpr, err := engine.ParseToCron(sched)
			if err != nil || cronExpr == "" {
				continue
			}

			targetMinion := m // capture for closure
			entryID, err := c.AddFunc(cronExpr, func() {
				executeMinion(ctx, dbStore, llmEval, targetMinion, "cron", activeCancels)
			})
			if err == nil {
				scheduledJobs[filename] = entryID
				logMessage("INFO", targetMinion.Name, fmt.Sprintf("Scheduled -> %s", cronExpr))
			}
		}
	}
}

func processRunQueue(ctx context.Context, dbStore *store.Store, llmEval *llm.Evaluator, activeCancels *sync.Map) {
	queue, err := dbStore.GetRunQueue()
	if err != nil || len(queue) == 0 {
		return
	}

	for _, filename := range queue {
		m, err := config.LoadMinion(filename)
		if err == nil {
			if !dbStore.GetMinionStatus(m.Filename) {
				logMessage("WARN", m.Name, "Cannot trigger minion because it is unscheduled/inactive.")
				_ = dbStore.DequeueRun(filename)
				continue
			}

			logMessage("INFO", m.Name, "Triggering immediate manual execution from queue.")
			// Execute in background routine so we don't block the 1s loop
			go executeMinion(ctx, dbStore, llmEval, m, "manual", activeCancels)
		}
		_ = dbStore.DequeueRun(filename)
	}
}

func processAbortQueue(dbStore *store.Store, activeCancels *sync.Map) {
	queue, err := dbStore.GetAbortQueue()
	if err != nil || len(queue) == 0 {
		return
	}

	for _, filename := range queue {
		if cancel, ok := activeCancels.Load(filename); ok {
			logMessage("WARN", filename, "Aborting active execution due to user request.")
			cancel.(context.CancelFunc)()
			activeCancels.Delete(filename)
		}
		_ = dbStore.DequeueAbort(filename)
	}
}

func executeMinion(ctx context.Context, dbStore *store.Store, llmEval *llm.Evaluator, m *config.MinionConfig, triggerType string, activeCancels *sync.Map) {
	activeJobs, _ := dbStore.GetActiveJobs()
	if activeJobs[m.Filename] {
		logMessage("WARN", m.Name, "Skipping execution: minion is already actively running.")
		return
	}

	displayFile := strings.TrimSuffix(m.Filename, ".yaml")
	displayFile = strings.TrimSuffix(displayFile, ".yml")
	minionLogPath := filepath.Join(config.LogsDir, displayFile+".log")

	minionLogFile, err := os.OpenFile(minionLogPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0666)
	if err != nil {
		logMessage("ERROR", m.Name, fmt.Sprintf("Failed to open log file: %v", err))
		return
	}
	defer minionLogFile.Close()

	fileRenderer := lipgloss.NewRenderer(minionLogFile)
	fileRenderer.SetColorProfile(termenv.TrueColor)
	
	stepStyle := fileRenderer.NewStyle().Bold(true).Width(15).Align(lipgloss.Right).MarginRight(2)
	okColor := lipgloss.Color("42")
	warnColor := lipgloss.Color("214")
	errColor := lipgloss.Color("9")
	infoColor := lipgloss.Color("39")
	neutralColor := lipgloss.Color("240")

	// Set a reasonable absolute max timeout for any minion execution
	runCtxTimeout, cancel := context.WithTimeout(ctx, 2*time.Hour)
	defer cancel()
	
	activeCancels.Store(m.Filename, cancel)
	defer activeCancels.Delete(m.Filename)
	
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
			
			detailStyle := fileRenderer.NewStyle()
			if isError {
				detailStyle = detailStyle.Foreground(errColor)
			} else if color == neutralColor {
				detailStyle = detailStyle.Foreground(lipgloss.Color("245"))
			}

			fmt.Fprintf(minionLogFile, "%s %s\n", s, detailStyle.Render(details))

			if isError {
				logMessage("ERROR", m.Name, fmt.Sprintf("%s: %s", step, details))
			}
		}, 
	}

	if err := engine.RunMission(runCtxTimeout, m, runCtx); err != nil {
		logMessage("ERROR", m.Name, fmt.Sprintf("Failed: %v", err))
	} else {
		statsMsg := fmt.Sprintf("Finished in %s (Scraped: %d, Cached: %d, Studied: %d, Discarded: %d, Skipped: %d, Found: %d, Delivered: %d, Errors: %d)",
			runCtx.Stats.Duration().Round(time.Millisecond*100),
			runCtx.Stats.PagesScraped,
			runCtx.Stats.PagesCached,
			runCtx.Stats.PagesStudied,
			runCtx.Stats.PagesDiscarded,
			runCtx.Stats.PagesSkipped,
			runCtx.Stats.ItemsFound,
			runCtx.Stats.ItemsDelivered,
			runCtx.Stats.Errors,
		)
		logMessage("INFO", m.Name, statsMsg)

		fmt.Fprintln(minionLogFile)
		boxStyle := fileRenderer.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("39")).
			Padding(1, 4)
		fmt.Fprintln(minionLogFile, boxStyle.Render(runCtx.Stats.GenerateReport(m.Name)))
		fmt.Fprintln(minionLogFile)
	}
}