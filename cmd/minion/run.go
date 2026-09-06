package minion

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"
	"github.com/robfig/cron/v3"
	"github.com/spf13/cobra"

	"minion/internal/character"
	"minion/internal/config"
	"minion/internal/delivery"
	"minion/internal/engine"
	"minion/internal/store"
	"minion/internal/types"
)

var detached bool

var runCmd = &cobra.Command{
	Use:   "run [filename | key=value ...]",
	Short: "Run an inline or saved minion, or start the master daemon",
	Long: `If no filename is provided, starts the master daemon in the background or foreground.

If a filename is provided, it queues the saved minion for execution by the master daemon.
If key=value assignments are provided, it runs an ephemeral minion synchronously.`,
	Args:         validateRunArgs,
	SilenceUsage: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		if isInlineRun(args) {
			if detached {
				return fmt.Errorf("--detach cannot be used with an inline run")
			}
			return runInline(cmd, args)
		}

		config.LoadEnv()

		if len(args) == 1 {
			queueMinionRun(args[0])
			return nil
		}

		checkPIDLock()

		if detached {
			runDetached()
			return nil
		}

		runDaemon()
		return nil
	},
}

func init() {
	runCmd.Flags().BoolVarP(&detached, "detach", "d", false, "Run daemon in the background")
	rootCmd.AddCommand(runCmd)
}

func validateRunArgs(_ *cobra.Command, args []string) error {
	if len(args) <= 1 || isInlineRun(args) {
		return nil
	}
	return fmt.Errorf("accepts one filename or one or more key=value assignments")
}

func isInlineRun(args []string) bool {
	return len(args) > 0 && strings.Contains(args[0], "=")
}

func runInline(cmd *cobra.Command, assignments []string) error {
	m, err := config.ParseInline(assignments)
	if err != nil {
		return err
	}
	config.LoadExistingEnv()
	return runInlineConfig(cmd, m)
}

func runInlineConfig(cmd *cobra.Command, m *config.MinionConfig) error {
	ctx, stop := signal.NotifyContext(cmd.Context(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	ctx, cancel := context.WithTimeout(ctx, 2*time.Hour)
	defer cancel()

	var results []types.Item
	runCtx := &engine.RunContext{
		Ephemeral: true,
		OnResult: func(item types.Item) {
			results = append(results, item)
		},
		OnStep: func(step, details string, isError bool) {
			if isError {
				fmt.Fprintf(cmd.ErrOrStderr(), "%s: %s\n", step, details)
			}
		},
	}

	runErr := engine.RunMission(ctx, m, runCtx)
	if err := writeInlineResults(cmd.OutOrStdout(), results); err != nil {
		return fmt.Errorf("write results: %w", err)
	}
	if runErr != nil {
		return fmt.Errorf("inline run failed: %w", runErr)
	}
	if runCtx.Stats != nil && runCtx.Stats.Errors > 0 {
		return fmt.Errorf("inline run completed with %d error(s)", runCtx.Stats.Errors)
	}
	return nil
}

func writeInlineResults(w io.Writer, items []types.Item) error {
	for i := range items {
		data, err := delivery.MarshalFileRecordYAML(&items[i])
		if err != nil {
			return err
		}

		if i > 0 {
			if _, err := io.WriteString(w, "---\n"); err != nil {
				return err
			}
		}
		if _, err := w.Write(data); err != nil {
			return err
		}
	}
	return nil
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
	_ = dbStore.ClearAbortQueue()

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
			syncFleet(daemonCtx, dbStore, c, scheduledJobs, activeCancels)
			ranWork := processRunQueue(daemonCtx, dbStore, activeCancels)
			if !ranWork {
				processChainQueue(daemonCtx, dbStore, activeCancels)
			}
			processAbortQueue(dbStore, activeCancels)

			if !ranWork {
				activeJobs, _ := dbStore.GetActiveJobs()
				activeMinions, _ := dbStore.GetActiveMinions()
				chainCount, _ := dbStore.GetChainDataCount()
				if len(activeJobs) == 0 && len(activeMinions) == 0 && chainCount == 0 {
					return
				}
			}
		case <-sigChan:
			fmt.Println()
			fmt.Printf("%s %s\n", lipgloss.NewStyle().Foreground(lipgloss.Color("240")).Render(time.Now().Format("2006-01-02 15:04:05")), lipgloss.NewStyle().Foreground(lipgloss.Color("214")).Render("Shutting down... waiting for running tasks to finish"))
			return
		}
	}
}

func syncFleet(ctx context.Context, dbStore *store.Store, c *cron.Cron, scheduledJobs map[string]cron.EntryID, activeCancels *sync.Map) {
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
				executeMinion(ctx, dbStore, targetMinion, "cron", activeCancels)
			})
			if err == nil {
				scheduledJobs[filename] = entryID
				logMessage("INFO", targetMinion.Name, fmt.Sprintf("Scheduled -> %s", cronExpr))
			}
		}
	}
}

func processRunQueue(ctx context.Context, dbStore *store.Store, activeCancels *sync.Map) bool {
	queue, err := dbStore.GetRunQueue()
	if err != nil || len(queue) == 0 {
		return false
	}

	spawned := false
	for _, filename := range queue {
		m, err := config.LoadMinion(filename)
		if err == nil {
			logMessage("INFO", m.Name, "Triggering immediate manual execution from queue.")
			// Execute in background routine so we don't block the 1s loop
			go executeMinion(ctx, dbStore, m, "manual", activeCancels)
			spawned = true
		}
		_ = dbStore.DequeueRun(filename)
	}
	return spawned
}

func processChainQueue(ctx context.Context, dbStore *store.Store, activeCancels *sync.Map) {
	minions, err := dbStore.GetChainDataMinions()
	if err != nil || len(minions) == 0 {
		return
	}
	activeJobs, _ := dbStore.GetActiveJobs()
	for _, fn := range minions {
		if activeJobs[fn] {
			continue
		}
		m, err := config.LoadMinion(fn)
		if err != nil {
			continue
		}
		go executeMinion(ctx, dbStore, m, "chain", activeCancels)
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

func executeMinion(ctx context.Context, dbStore *store.Store, m *config.MinionConfig, triggerType string, activeCancels *sync.Map) {
	activeJobs, _ := dbStore.GetActiveJobs()
	if activeJobs[m.Filename] {
		logMessage("WARN", m.Name, "Skipping execution: minion is already actively running.")
		return
	}

	displayFile := strings.TrimSuffix(m.Filename, ".yaml")
	displayFile = strings.TrimSuffix(displayFile, ".yml")
	minionLogPath := filepath.Join(config.LogsDir, displayFile+".log")

	os.Remove(minionLogPath)
	minionLogFile, err := os.OpenFile(minionLogPath, os.O_CREATE|os.O_WRONLY, 0666)
	if err != nil {
		logMessage("ERROR", m.Name, fmt.Sprintf("Failed to open log file: %v", err))
		return
	}
	defer minionLogFile.Close()

	fileRenderer := lipgloss.NewRenderer(minionLogFile)
	fileRenderer.SetColorProfile(termenv.TrueColor)

	stepStyle := fileRenderer.NewStyle().Bold(true).Width(15).Align(lipgloss.Right).MarginRight(2)
	neutralColor := lipgloss.Color("240")
	urlRe := regexp.MustCompile(`https?://[^\s` + "`" + `]+`)
	btRe := regexp.MustCompile("`([^`]*)`")

	// Set a reasonable absolute max timeout for any minion execution
	runCtxTimeout, cancel := context.WithTimeout(ctx, 2*time.Hour)
	defer cancel()

	activeCancels.Store(m.Filename, cancel)
	defer activeCancels.Delete(m.Filename)

	runCtx := &engine.RunContext{
		Store: dbStore,
		OnStep: func(step, details string, isError bool) {
			if step == "" {
				fmt.Fprintln(minionLogFile)
				return
			}
			color := neutralColor
			switch step {
			case "start", "from":
				color = lipgloss.Color("39")
			case "do":
				color = lipgloss.Color("141")
			case "done", "result", "tell", "report", "unchanged":
				color = lipgloss.Color("42")
			case "skip", "keep", "ignore":
				color = lipgloss.Color("214")
			case "discard", "discarded":
				color = lipgloss.Color("75")
			}

			if isError {
				color = lipgloss.Color("9")
			}

			s := stepStyle.Foreground(color).Render("[" + step + "]")

			dataStyle := fileRenderer.NewStyle().Foreground(lipgloss.Color("67"))
			textStyle := fileRenderer.NewStyle().Foreground(lipgloss.Color("252"))
			dimStyle := fileRenderer.NewStyle().Foreground(lipgloss.Color("243"))

			var renderedDetail string
			if isError {
				renderedDetail = fileRenderer.NewStyle().Foreground(lipgloss.Color("9")).Render(details)
			} else if step == "tell" && !strings.HasPrefix(details, "→ minion") && !strings.Contains(details, ":") {
				parts := strings.SplitN(details, "→ ", 2)
				if len(parts) == 2 {
					renderedDetail = dimStyle.Render(parts[0]+"→ ") + textStyle.Render(parts[1])
				}
			}

			if renderedDetail == "" {
				var buf strings.Builder
				lastEnd := 0
				for _, loc := range btRe.FindAllStringIndex(details, -1) {
					before := details[lastEnd:loc[0]]
					for _, urlLoc := range urlRe.FindAllStringIndex(before, -1) {
						buf.WriteString(textStyle.Render(before[:urlLoc[0]]))
						buf.WriteString(dataStyle.Render(before[urlLoc[0]:urlLoc[1]]))
						before = before[urlLoc[1]:]
					}
					buf.WriteString(textStyle.Render(before))
					buf.WriteString(dataStyle.Render(details[loc[0]+1 : loc[1]-1]))
					lastEnd = loc[1]
				}
				remaining := details[lastEnd:]
				for _, urlLoc := range urlRe.FindAllStringIndex(remaining, -1) {
					buf.WriteString(textStyle.Render(remaining[:urlLoc[0]]))
					buf.WriteString(dataStyle.Render(remaining[urlLoc[0]:urlLoc[1]]))
					remaining = remaining[urlLoc[1]:]
				}
				buf.WriteString(textStyle.Render(remaining))
				renderedDetail = buf.String()
			}

			fmt.Fprintf(minionLogFile, "%s %s\n", s, renderedDetail)

			if isError {
				logMessage("ERROR", m.Name, fmt.Sprintf("%s: %s", step, details))
			}
		},
	}

	hasChain, _ := dbStore.HasChainData(m.Filename)
	var runErr error
	if hasChain || triggerType == "chain" {
		runErr = engine.ProcessChainTrigger(runCtxTimeout, m, runCtx)
	} else {
		runErr = engine.RunMission(runCtxTimeout, m, runCtx)
	}

	if runCtx.Stats != nil && character.Enabled() {
		_ = dbStore.UpdateCharacterState(m.Filename, runCtx.Stats.Results, runCtx.Stats.Errors)
	}

	if runErr != nil {
		logMessage("ERROR", m.Name, fmt.Sprintf("Failed: %v", runErr))
	} else {
		statsMsg := fmt.Sprintf("Finished in %s (Fetched: %d, Unchanged: %d, Analyzed: %d, Discarded: %d, Skipped: %d, Results: %d, Sent: %d, Errors: %d)",
			runCtx.Stats.Duration().Round(time.Millisecond*100),
			runCtx.Stats.Fetched,
			runCtx.Stats.Unchanged,
			runCtx.Stats.Analyzed,
			runCtx.Stats.Discarded,
			runCtx.Stats.Skipped,
			runCtx.Stats.Results,
			runCtx.Stats.Sent,
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
