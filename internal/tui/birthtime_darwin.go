//go:build darwin

package tui

import (
	"os"
	"syscall"
	"time"
)

func getBirthTime(fi os.FileInfo) time.Time {
	if stat, ok := fi.Sys().(*syscall.Stat_t); ok {
		return time.Unix(stat.Birthtimespec.Sec, stat.Birthtimespec.Nsec)
	}
	return fi.ModTime()
}
