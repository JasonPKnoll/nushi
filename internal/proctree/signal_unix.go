//go:build unix

package proctree

import "syscall"

// signalGroup sends process group pgid SIGTERM, or SIGKILL with hard.
func signalGroup(pgid int, hard bool) error { return syscall.Kill(-pgid, sig(hard)) }

// signalOne sends process pid SIGTERM, or SIGKILL with hard.
func signalOne(pid int, hard bool) error { return syscall.Kill(pid, sig(hard)) }

func sig(hard bool) syscall.Signal {
	if hard {
		return syscall.SIGKILL
	}
	return syscall.SIGTERM
}
