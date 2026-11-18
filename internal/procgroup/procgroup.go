// Package procgroup bounds a command run under a context to its process
// group: when the context ends, what the command started goes with it,
// and a process left holding its output can't keep the run waiting.
package procgroup

import (
	"os/exec"
	"time"
)

// Bound has cmd, made by exec.CommandContext, start in a group of its own
// (Own), killed whole when its context ends (Kill; on Windows the group
// its tracker keeps, bind); Wait then gives what it left running a second
// to let go of its stdout and stderr.
func Bound(cmd *exec.Cmd) {
	Own(cmd)
	cmd.Cancel = bind(cmd)
	cmd.WaitDelay = time.Second
}

// Start starts cmd in a group of its own (Own) and holds that group, so
// that the Group's Kill ends what cmd started too.
func Start(cmd *exec.Cmd) (*Group, error) {
	Own(cmd)
	if err := cmd.Start(); err != nil {
		return nil, err
	}
	return hold(cmd.Process), nil
}
