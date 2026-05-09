package minion

import (
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"

	"github.com/spf13/cobra"
	"minion/internal/config"
)

var logCmd = &cobra.Command{
	Use:   "log [filename]",
	Short: "Follows the live output of a specific minion, or the master daemon log",
	Long: `Follows the live output logs. 
If you provide a minion filename (e.g. "minion log event_finder"), it follows the detailed step-by-step log for that specific minion.
If you provide no arguments, it follows the master daemon log.`,
	Args:  cobra.MaximumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		config.LoadEnv()

		targetLog := config.LogPath
		if len(args) == 1 {
			filename := args[0]
			displayFile := strings.TrimSuffix(filename, ".yaml")
			displayFile = strings.TrimSuffix(displayFile, ".yml")
			targetLog = filepath.Join(config.LogsDir, displayFile+".log")
		}

		if _, err := os.Stat(targetLog); os.IsNotExist(err) {
			if len(args) == 1 {
				fmt.Printf("Log file not found at %s\nThis minion has not run yet.\n", targetLog)
			} else {
				fmt.Printf("Master log file not found at %s\nThe daemon has not been started yet.\n", config.LogPath)
			}
			return
		}

		fmt.Printf("Following log at %s...\n(Press Ctrl+C to exit)\n\n", targetLog)

		tailCmd := exec.Command("tail", "-n", "+1", "-f", targetLog)
		tailCmd.Stdout = os.Stdout
		tailCmd.Stderr = os.Stderr

		if err := tailCmd.Start(); err != nil {
			fmt.Printf("Error trailing log: %v\n", err)
			return
		}

		sigChan := make(chan os.Signal, 1)
		signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
		<-sigChan

		_ = tailCmd.Process.Kill()
		fmt.Println("\nStopped following log.")
	},
}

func init() {
	rootCmd.AddCommand(logCmd)
}