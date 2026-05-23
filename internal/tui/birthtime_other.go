//go:build !darwin

package tui

import (
	"os"
	"time"
)

func getBirthTime(fi os.FileInfo) time.Time {
	return fi.ModTime()
}
