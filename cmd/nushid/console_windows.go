package main

import (
	"os"
	"os/exec"
	"syscall"

	"golang.org/x/sys/windows"
)

// relaunchedVar marks the daemon started again by hideConsole, so it never
// starts itself twice.
const relaunchedVar = "NUSHID_NO_WINDOW"

// hideConsole gives the daemon a console with no window. A command-line
// program (an agent, its bridge, git, gh) started by a process that has
// no console opens a console window of its own, so a daemon started
// detached, as the app starts it, would flash one for each. A console
// started with CREATE_NO_WINDOW has none, and what the daemon starts
// shares it. So with no console, the daemon starts itself again that way
// and says so: the caller exits, and the one started takes the socket.
// One with a console (started from a terminal) goes on as it is.
func hideConsole() (relaunched bool, err error) {
	if os.Getenv(relaunchedVar) != "" {
		_ = os.Unsetenv(relaunchedVar) // nothing the daemon starts needs it
		return false, nil
	}
	if hasConsole() {
		return false, nil
	}
	exe, err := os.Executable()
	if err != nil {
		return false, err
	}
	cmd := exec.Command(exe, os.Args[1:]...) //nolint:gosec,noctx // this daemon itself, its own arguments; it isn't waited on
	cmd.Env = append(os.Environ(), relaunchedVar+"=1")
	cmd.SysProcAttr = &syscall.SysProcAttr{CreationFlags: windows.CREATE_NO_WINDOW}
	if err := cmd.Start(); err != nil {
		return false, err
	}
	return true, cmd.Process.Release()
}

// hasConsole says the process is attached to a console, shown or not.
func hasConsole() bool {
	f, err := os.OpenFile("CONOUT$", os.O_WRONLY, 0)
	if err != nil {
		return false
	}
	_ = f.Close()
	return true
}
