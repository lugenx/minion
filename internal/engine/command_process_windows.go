//go:build windows

package engine

import (
	"errors"
	"os"
	"os/exec"
	"strconv"
)

func configureCommandProcess(cmd *exec.Cmd) {
	cmd.Cancel = func() error { return terminateCommandProcess(cmd) }
}

func terminateCommandProcess(cmd *exec.Cmd) error {
	if cmd.Process == nil {
		return os.ErrProcessDone
	}

	pid := strconv.Itoa(cmd.Process.Pid)
	if err := exec.Command("taskkill", "/T", "/F", "/PID", pid).Run(); err == nil {
		return nil
	}

	if err := cmd.Process.Kill(); err != nil && !errors.Is(err, os.ErrProcessDone) {
		return err
	}
	return nil
}
