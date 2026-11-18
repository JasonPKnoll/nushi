package footprint

import (
	"fmt"
	"math"
	"unsafe"

	"golang.org/x/sys/windows"
)

// getProcessMemoryInfo is psapi's GetProcessMemoryInfo, which x/sys
// doesn't wrap; the system's own DLL, by its full path (no cgo).
var getProcessMemoryInfo = windows.NewLazySystemDLL("psapi.dll").NewProc("GetProcessMemoryInfo")

// processMemoryCountersEx is PROCESS_MEMORY_COUNTERS_EX.
type processMemoryCountersEx struct {
	Cb                         uint32
	PageFaultCount             uint32
	PeakWorkingSetSize         uintptr
	WorkingSetSize             uintptr
	QuotaPeakPagedPoolUsage    uintptr
	QuotaPagedPoolUsage        uintptr
	QuotaPeakNonPagedPoolUsage uintptr
	QuotaNonPagedPoolUsage     uintptr
	PagefileUsage              uintptr
	PeakPagefileUsage          uintptr
	PrivateUsage               uintptr
}

// Of returns the memory process pid takes in bytes: its private bytes
// (PROCESS_MEMORY_COUNTERS_EX's PrivateUsage, Task Manager's "Commit
// size"), the memory it alone holds, in RAM or paged out, as macOS's
// footprint counts what it has compressed or swapped. Memory it shares
// (its DLLs' code, a mapped file) isn't counted, so Tree's sum over a tree
// counts none of it twice. 0 when it can't be read: gone, or one the
// user can't open (a service's, a protected process).
func Of(pid int) uint64 {
	if pid <= 0 || uint64(pid) > math.MaxUint32 {
		return 0
	}
	h, err := windows.OpenProcess(windows.PROCESS_QUERY_LIMITED_INFORMATION, false, uint32(pid))
	if err != nil {
		return 0
	}
	defer windows.CloseHandle(h) //nolint:errcheck // a process handle
	var c processMemoryCountersEx
	c.Cb = uint32(unsafe.Sizeof(c))
	if ok, _, _ := getProcessMemoryInfo.Call(uintptr(h), uintptr(unsafe.Pointer(&c)), uintptr(c.Cb)); ok == 0 { //nolint:gosec // the call's own struct
		return 0
	}
	return uint64(c.PrivateUsage)
}

// Snapshot lists the running processes, from Toolhelp's snapshot. A
// Windows process keeps the pid of the parent that started it after that
// parent has ended, and the pid can be given to another process, which
// isn't its parent: so a process isn't taken for its parent's child when
// the parent started after it (parented, proctree's rule on Windows).
// Starts are read only as a walk down from a process asks (Tree,
// Children), each once, not every process's at each snapshot.
func Snapshot() (Processes, error) {
	snap, err := windows.CreateToolhelp32Snapshot(windows.TH32CS_SNAPPROCESS, 0)
	if err != nil {
		return Processes{}, fmt.Errorf("listing processes: %w", err)
	}
	defer windows.CloseHandle(snap) //nolint:errcheck // a snapshot
	p := Processes{children: map[int][]int{}}
	e := windows.ProcessEntry32{Size: uint32(unsafe.Sizeof(windows.ProcessEntry32{}))}
	n := 0
	for err = windows.Process32First(snap, &e); err == nil; err = windows.Process32Next(snap, &e) {
		p.children[int(e.ParentProcessID)] = append(p.children[int(e.ParentProcessID)], int(e.ProcessID))
		n++
	}
	if n == 0 {
		return Processes{}, fmt.Errorf("listing processes: %w", err)
	}
	type start struct {
		t  int64
		ok bool
	}
	starts := map[int]start{}
	startOf := func(pid int) (int64, bool) {
		s, read := starts[pid]
		if !read {
			s.t, s.ok = startTime(pid)
			starts[pid] = s
		}
		return s.t, s.ok
	}
	p.isChild = func(parent, child int) bool {
		ps, okParent := startOf(parent)
		cs, okChild := startOf(child)
		return parented(ps, okParent, cs, okChild)
	}
	return p, nil
}

// parented says whether a process that started at child is the child of
// the one its parent id names, which started at parent: unless both are
// known and the parent started after it (its id used again), as
// proctree's parented has it.
func parented(parent int64, okParent bool, child int64, okChild bool) bool {
	return !okParent || !okChild || parent <= child
}

// startTime is when process pid started, in nanoseconds since 1970;
// false when it can't be opened.
func startTime(pid int) (int64, bool) {
	if pid <= 0 || uint64(pid) > math.MaxUint32 {
		return 0, false
	}
	h, err := windows.OpenProcess(windows.PROCESS_QUERY_LIMITED_INFORMATION, false, uint32(pid))
	if err != nil {
		return 0, false
	}
	defer windows.CloseHandle(h) //nolint:errcheck // a process handle
	var created, exited, kernel, user windows.Filetime
	if windows.GetProcessTimes(h, &created, &exited, &kernel, &user) != nil {
		return 0, false
	}
	return created.Nanoseconds(), true
}
