// Package procgroup bounds a command run under a context to its process
// group: when the context ends, what the command started goes with it,
// and a process left holding its output can't keep the run waiting.
package procgroup

import (
	"os/exec"
	"time"
)

// Bound has cmd, made by exec.CommandContext, start so that what it starts
// can be ended with it (procgroup_unix.go, procgroup_windows.go), all of it
// killed when its context ends; Wait then gives what it left running a
// second to let go of its stdout and stderr.
func Bound(cmd *exec.Cmd) {
	bound(cmd)
	cmd.WaitDelay = time.Second
}
