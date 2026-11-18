//go:build unix

package proctree

import (
	"syscall"
	"time"
)

// signalGroup sends process group pgid SIGTERM, or SIGKILL with hard.
func signalGroup(pgid int, hard bool) error { return syscall.Kill(-pgid, sig(hard)) }

// signalOne sends process pid SIGTERM, or SIGKILL with hard. A Unix
// signal goes by pid alone, so started (which Windows checks) isn't used.
func signalOne(pid int, _ time.Time, hard bool) error { return syscall.Kill(pid, sig(hard)) }

func sig(hard bool) syscall.Signal {
	if hard {
		return syscall.SIGKILL
	}
	return syscall.SIGTERM
}
