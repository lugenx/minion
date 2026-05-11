package minion

import (
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/spf13/cobra"

	"minion/internal/config"
	"minion/internal/store"
)

var downCmd = &cobra.Command{
	Use:   "down [filename|all]",
	Short: "Unschedules a minion, or stops the entire background daemon",
	Long: `If no arguments are provided, it completely shuts down the master background daemon and all running tasks.
If a filename (or "all") is provided, it marks the minion as Inactive (unscheduled). 

If you unschedule the last active minion, it will automatically shut down the master daemon to save resources.

Usage:
  minion down                 (Kills master daemon completely)
  minion down event_finder    (Unschedules event_finder)
  minion down all             (Unschedules all minions)`,
	Args:  cobra.MaximumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		config.LoadEnv()

		if len(args) == 0 {
			killDaemon()
			return
		}

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

		stoppedCount := 0
		for _, m := range minions {
			if target == "all" || m.Filename == target || m.Name == target {
				if err := dbStore.SetMinionStatus(m.Filename, false); err != nil {
					fmt.Printf("Error stopping %s: %v\n", m.Name, err)
					continue
				}
				stoppedCount++
				
					displayName := strings.TrimSuffix(m.Filename, ".yaml")
					displayName = strings.TrimSuffix(displayName, ".yml")
					fmt.Println(lipgloss.NewStyle().Foreground(lipgloss.Color("240")).Render(fmt.Sprintf("%s is Down", displayName)))
				}
			}

			if stoppedCount == 0 {
				if target != "all" {
				displayName := strings.TrimSuffix(target, ".yaml")
				fmt.Println(lipgloss.NewStyle().Foreground(lipgloss.Color("9")).Render(fmt.Sprintf("Minion %s not found.", displayName)))
			}
			os.Exit(1)
		}
	},
}

func init() {
	rootCmd.AddCommand(downCmd)
}

func killDaemon() {
	pidBytes, err := os.ReadFile(config.PIDPath)
	if err != nil {
		return
	}

	pidStr := strings.TrimSpace(string(pidBytes))
	pid, err := strconv.Atoi(pidStr)
	if err != nil {
		_ = os.Remove(config.PIDPath)
		return
	}

	process, err := os.FindProcess(pid)
	if err != nil {
		_ = os.Remove(config.PIDPath)
		return
	}
	
	// Check if it's already dead
	if err := process.Signal(syscall.Signal(0)); err != nil {
		_ = os.Remove(config.PIDPath)
		return
	}

	if runtime.GOOS == "windows" {
		exec.Command("taskkill", "/PID", pidStr).Run()
	} else {
		err = process.Signal(syscall.SIGTERM)
	}

	if err != nil && !strings.Contains(err.Error(), "process already finished") {
		return
	} 
	
	// Wait for process to actually exit
	for {
		if err := process.Signal(syscall.Signal(0)); err != nil {
			break
		}
		time.Sleep(500 * time.Millisecond)
	}
}