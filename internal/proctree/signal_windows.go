package proctree

import (
	"errors"
	"fmt"
	"time"

	"golang.org/x/sys/windows"
)

// signalGroup sends nothing: Windows has no process groups (Pgid is 0, so
// none is ever asked for, ADR 0132).
func signalGroup(int, bool) error {
	return fmt.Errorf("signalling a process group on windows: %w", errors.ErrUnsupported)
}

// signalOne ends process pid with hard (TerminateProcess), if it is still
// the one that started at started. Without hard it sends nothing: Windows
// has no SIGTERM for a program it didn't start on a console of its own,
// so Stop ends it at once rather than wait out a grace. The process is
// opened once, and its start time read and the process ended through that
// one handle: while it is open the pid can't be given to another process,
// so one that ended and had its pid used again since Stop listed it is
// left alone.
func signalOne(pid int, started time.Time, hard bool) error {
	if !hard {
		return fmt.Errorf("asking a process to end on windows: %w", errors.ErrUnsupported)
	}
	h, err := windows.OpenProcess(windows.PROCESS_TERMINATE|windows.PROCESS_QUERY_LIMITED_INFORMATION, false, uint32(pid)) //nolint:gosec // a pid fits
	if err != nil {
		return err
	}
	defer windows.CloseHandle(h) //nolint:errcheck // done with it
	var created, exited, kernel, user windows.Filetime
	if err := windows.GetProcessTimes(h, &created, &exited, &kernel, &user); err != nil {
		return err
	}
	if !time.Unix(0, created.Nanoseconds()).Equal(started) { // as fileTime reads the process list's
		return fmt.Errorf("process %d is no longer the one that started at %v", pid, started)
	}
	return windows.TerminateProcess(h, 1)
}
