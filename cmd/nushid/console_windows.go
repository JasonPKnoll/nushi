package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"syscall"

	"golang.org/x/sys/windows"

	"github.com/afrase/nushi/internal/logs"
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
// One with a console (started from a terminal) goes on as it is, and so
// does one whose data folder can't be made the user's (ownDataFolder):
// run stops on the same check, and says why on this process's stderr,
// which the app reads, rather than in a started daemon's no one sees.
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
	if dir := startFolder(); dir != "" {
		if os.MkdirAll(dir, 0o700) != nil || ownDataFolder(dir) != nil {
			return false, nil // run stops on it, and says why
		}
		if log, err := logs.OpenAppend(filepath.Join(dir, "nushid.log")); err == nil {
			defer log.Close() //nolint:errcheck // the started daemon has its own handle
			cmd.Stderr = log
		}
	}
	if err := cmd.Start(); err != nil {
		return false, err
	}
	return true, cmd.Process.Release()
}

// startFolder is the data folder, "" when there's none, for the
// relaunched daemon's stderr: nushid.log there, so what it says before it
// logs there itself (a crash) lands where the app points, not in NUL, as
// a launcher's `2>>` has it on Unix. The folder is made and given its
// DACL first (ownDataFolder), so the log is never opened in a folder
// another account controls.
func startFolder() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return dataFolder(home)
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
