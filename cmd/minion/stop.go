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
)

var stopCmd = &cobra.Command{
	Use:   "stop",
	Short: "Stops the background daemon",
	Args:  cobra.NoArgs,
	Run: func(cmd *cobra.Command, args []string) {
		config.LoadEnv()

		pidBytes, err := os.ReadFile(config.PIDPath)
		if err != nil {
			fmt.Println(lipgloss.NewStyle().Foreground(lipgloss.Color("240")).Render("Daemon is not running (no PID file found)."))
			return
		}

		pidStr := strings.TrimSpace(string(pidBytes))
		pid, err := strconv.Atoi(pidStr)
		if err != nil {
			fmt.Printf("Invalid PID in file: %v\n", err)
			_ = os.Remove(config.PIDPath)
			return
		}

		process, err := os.FindProcess(pid)
		if err != nil {
			fmt.Printf("Failed to find process: %v\n", err)
			_ = os.Remove(config.PIDPath)
			return
		}
		
		// Check if it's already dead
		if err := process.Signal(syscall.Signal(0)); err != nil {
			fmt.Println(lipgloss.NewStyle().Foreground(lipgloss.Color("240")).Render("Daemon is not running (stale PID file)."))
			_ = os.Remove(config.PIDPath)
			return
		}
		
		fmt.Printf("%s\n", lipgloss.NewStyle().Foreground(lipgloss.Color("214")).Render("Sending stop signal... waiting for current tasks to finish."))

		if runtime.GOOS == "windows" {
			exec.Command("taskkill", "/PID", pidStr).Run()
		} else {
			err = process.Signal(syscall.SIGTERM)
		}

		if err != nil && !strings.Contains(err.Error(), "process already finished") {
			fmt.Printf("Failed to send stop signal: %v\n", err)
			return
		} 
		
		// Wait for process to actually exit
		for {
			if err := process.Signal(syscall.Signal(0)); err != nil {
				// Process has exited
				break
			}
			time.Sleep(500 * time.Millisecond)
		}

		fmt.Println(lipgloss.NewStyle().Foreground(lipgloss.Color("42")).Render("Stopped Minion daemon successfully."))
	},
}

func init() {
	rootCmd.AddCommand(stopCmd)
}