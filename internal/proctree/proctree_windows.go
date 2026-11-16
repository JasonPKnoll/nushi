package proctree

import (
	"errors"
	"fmt"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
)

// On Windows (ADR 0132) who started whom is a Toolhelp snapshot's; a
// process's start time is GetProcessTimes's and its command line the
// system's own copy (NtQueryInformationProcess), both readable for the
// user's own processes. Windows has no process groups: Pgid is 0, so
// StopGroup stops none (signal_windows.go).

// all is every process, with its parent and, where it can be read, its
// start time. Windows keeps a process's parent id after the parent has
// ended, and may give the id to another: a parent that started after its
// child isn't its parent, and the child is left with none.
func all() ([]proc, error) {
	snap, err := windows.CreateToolhelp32Snapshot(windows.TH32CS_SNAPPROCESS, 0)
	if err != nil {
		return nil, fmt.Errorf("listing processes: %w", err)
	}
	defer windows.CloseHandle(snap) //nolint:errcheck // read only
	var ps []proc
	e := windows.ProcessEntry32{Size: uint32(unsafe.Sizeof(windows.ProcessEntry32{}))}
	for err = windows.Process32First(snap, &e); err == nil; err = windows.Process32Next(snap, &e) {
		ps = append(ps, proc{Pid: int(e.ProcessID), Started: started(e.ProcessID), ppid: int(e.ParentProcessID)})
	}
	if !errors.Is(err, windows.ERROR_NO_MORE_FILES) {
		return nil, fmt.Errorf("listing processes: %w", err)
	}
	return parented(ps), nil
}

// parented drops each parent id that names a process started after its
// child: an id used again.
func parented(ps []proc) []proc {
	start := make(map[int]time.Time, len(ps))
	for _, p := range ps {
		start[p.Pid] = p.Started
	}
	for i, p := range ps {
		if s, ok := start[p.ppid]; ok && !s.IsZero() && !p.Started.IsZero() && s.After(p.Started) {
			ps[i].ppid = -1
		}
	}
	return ps
}

// started is when process pid started, or the zero time when it can't be
// read (it ended, or it isn't the user's).
func started(pid uint32) time.Time {
	h, err := windows.OpenProcess(windows.PROCESS_QUERY_LIMITED_INFORMATION, false, pid)
	if err != nil {
		return time.Time{}
	}
	defer windows.CloseHandle(h) //nolint:errcheck // read only
	var created, exited, kernel, user windows.Filetime
	if windows.GetProcessTimes(h, &created, &exited, &kernel, &user) != nil {
		return time.Time{}
	}
	return time.Unix(0, created.Nanoseconds())
}

// Args reads a process's command line as Windows split it for the
// program's C runtime, nil when it can't be read (it ended, or it isn't
// the user's).
func Args(pid int) []string {
	if pid <= 0 {
		return nil
	}
	h, err := windows.OpenProcess(windows.PROCESS_QUERY_LIMITED_INFORMATION, false, uint32(pid)) //nolint:gosec // a pid fits
	if err != nil {
		return nil
	}
	defer windows.CloseHandle(h) //nolint:errcheck // read only
	var n uint32
	_ = windows.NtQueryInformationProcess(h, windows.ProcessCommandLineInformation, nil, 0, &n) // its size
	if n < uint32(unsafe.Sizeof(windows.NTUnicodeString{})) {
		return nil
	}
	buf := make([]uint64, (n+7)/8)                                                                                          // aligned for the UNICODE_STRING it starts with
	if windows.NtQueryInformationProcess(h, windows.ProcessCommandLineInformation, unsafe.Pointer(&buf[0]), n, &n) != nil { //nolint:gosec // the system fills buf, n bytes
		return nil
	}
	line := (*windows.NTUnicodeString)(unsafe.Pointer(&buf[0])).String() //nolint:gosec // buf starts with the UNICODE_STRING the system wrote
	if line == "" {
		return []string{}
	}
	args, err := windows.DecomposeCommandLine(line)
	if err != nil {
		return nil
	}
	return args
}

// now is the time in Started's terms, the wall clock's.
func now() time.Time { return time.Now() }
