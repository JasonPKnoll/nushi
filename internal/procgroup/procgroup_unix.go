//go:build unix

package procgroup

import (
	"os"
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

// Kill kills the process group p leads, as Own started it.
func Kill(p *os.Process) error { return syscall.Kill(-p.Pid, syscall.SIGKILL) }

// bind is what Bound's cmd does when its context ends: kill its group.
func bind(cmd *exec.Cmd) func() error {
	return func() error { return Kill(cmd.Process) }
}

// Group is the process group a command Start started leads.
type Group struct{ p *os.Process }

func hold(p *os.Process) *Group { return &Group{p} }

// Kill kills the group (Kill): what is left of it once its leader has
// exited too.
func (g *Group) Kill() error { return Kill(g.p) }

// Session has cmd start in a session of its own, with no terminal to take
// or wait on; its group is then its own too, which Own keeps.
func Session(cmd *exec.Cmd) {
	if cmd.SysProcAttr == nil {
		cmd.SysProcAttr = &syscall.SysProcAttr{}
	}
	cmd.SysProcAttr.Setsid = true
}
