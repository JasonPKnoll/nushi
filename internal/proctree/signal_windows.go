package proctree

import (
	"errors"
	"fmt"

	"golang.org/x/sys/windows"
)

// signalGroup sends nothing: Windows has no process groups (Pgid is 0, so
// none is ever asked for, ADR 0130).
func signalGroup(int, bool) error {
	return fmt.Errorf("signalling a process group on windows: %w", errors.ErrUnsupported)
}

// signalOne ends a process with hard (TerminateProcess). Without it, it
// sends nothing: Windows has no SIGTERM for a program it didn't start on
// a console of its own, so Stop's grace passes and the hard end follows.
func signalOne(pid int, hard bool) error {
	if !hard {
		return fmt.Errorf("asking a process to end on windows: %w", errors.ErrUnsupported)
	}
	h, err := windows.OpenProcess(windows.PROCESS_TERMINATE, false, uint32(pid)) //nolint:gosec // a pid fits
	if err != nil {
		return err
	}
	defer windows.CloseHandle(h) //nolint:errcheck // done with it
	return windows.TerminateProcess(h, 1)
}
