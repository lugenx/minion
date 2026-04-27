//go:build windows

package minion

import (
	"os/exec"
)

func setSysProcAttr(cmd *exec.Cmd) {
	// On Windows, detachment behaves differently. 
	// We don't set Setsid here.
}
