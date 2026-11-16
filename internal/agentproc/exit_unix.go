//go:build unix

package agentproc

import (
	"os"
	"syscall"

	"golang.org/x/sys/unix"
)

// killedBy is the signal that ended ps's process, by name ("SIGKILL"),
// if one did.
func killedBy(ps *os.ProcessState) (string, bool) {
	if ws, ok := ps.Sys().(syscall.WaitStatus); ok && ws.Signaled() {
		return unix.SignalName(ws.Signal()), true
	}
	return "", false
}
