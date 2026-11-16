//go:build unix

package claudehistory

import (
	"errors"
	"syscall"
)

// alive says whether a process runs with pid (one of another user's
// counts).
func alive(pid int) bool {
	err := syscall.Kill(pid, 0)
	return err == nil || errors.Is(err, syscall.EPERM)
}
