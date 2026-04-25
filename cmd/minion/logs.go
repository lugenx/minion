package minion

import (
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"syscall"

	"github.com/spf13/cobra"
	"minion/internal/config"
)

var logsCmd = &cobra.Command{
	Use:   "logs",
	Short: "Follows the live output of the background daemon",
	Run: func(cmd *cobra.Command, args []string) {
		config.LoadEnv()

		if _, err := os.Stat(config.LogPath); os.IsNotExist(err) {
			fmt.Printf("Log file not found at %s\nNo background jobs have run yet.\n", config.LogPath)
			return
		}

		fmt.Printf("Following logs at %s...\n(Press Ctrl+C to exit)\n\n", config.LogPath)

		tailCmd := exec.Command("tail", "-f", config.LogPath)
		tailCmd.Stdout = os.Stdout
		tailCmd.Stderr = os.Stderr

		if err := tailCmd.Start(); err != nil {
			fmt.Printf("Error trailing logs: %v\n", err)
			return
		}

		sigChan := make(chan os.Signal, 1)
		signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
		<-sigChan

		_ = tailCmd.Process.Kill()
		fmt.Println("\nStopped following logs.")
	},
}

func init() {
	rootCmd.AddCommand(logsCmd)
}