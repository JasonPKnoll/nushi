//go:build unix

package procgroup

import (
	"os/exec"
	"syscall"
)

// Own has cmd start in a process group of its own, unless it asked for a
// session of its own with Setsid, whose group is its own too.
func Own(cmd *exec.Cmd) {
	if cmd.SysProcAttr == nil {
		cmd.SysProcAttr = &syscall.SysProcAttr{}
	}
	if !cmd.SysProcAttr.Setsid {
		cmd.SysProcAttr.Setpgid = true
	}
}

// Kill kills the process group pid leads, as Own started it.
func Kill(pid int) error { return syscall.Kill(-pid, syscall.SIGKILL) }
