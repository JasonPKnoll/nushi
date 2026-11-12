//go:build unix

package procgroup

import (
	"os/exec"
	"syscall"
)

// bound starts cmd in a process group of its own (or in the session it
// asked for with Setsid, whose group is its own too), and kills the whole
// group on cancel.
func bound(cmd *exec.Cmd) {
	if cmd.SysProcAttr == nil {
		cmd.SysProcAttr = &syscall.SysProcAttr{}
	}
	if !cmd.SysProcAttr.Setsid {
		cmd.SysProcAttr.Setpgid = true
	}
	cmd.Cancel = func() error { return syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL) }
}
