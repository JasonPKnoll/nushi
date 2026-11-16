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
	if pid <= 0 || pid > math.MaxUint32 {
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
// isn't its parent: so a process is only taken for its parent's child
// when it started after the parent did, and one whose start can't be read
// (a service's, a protected process) for no one's. That also keeps a tree
// from looping back on itself.
func Snapshot() (Processes, error) {
	snap, err := windows.CreateToolhelp32Snapshot(windows.TH32CS_SNAPPROCESS, 0)
	if err != nil {
		return nil, fmt.Errorf("listing processes: %w", err)
	}
	defer windows.CloseHandle(snap) //nolint:errcheck // a snapshot
	type entry struct{ pid, ppid uint32 }
	var all []entry
	e := windows.ProcessEntry32{Size: uint32(unsafe.Sizeof(windows.ProcessEntry32{}))}
	for err = windows.Process32First(snap, &e); err == nil; err = windows.Process32Next(snap, &e) {
		all = append(all, entry{e.ProcessID, e.ParentProcessID})
	}
	if len(all) == 0 {
		return nil, fmt.Errorf("listing processes: %w", err)
	}
	started := make(map[uint32]int64, len(all))
	for _, p := range all {
		if t, ok := startTime(p.pid); ok {
			started[p.pid] = t
		}
	}
	p := Processes{}
	for _, c := range all {
		child, okChild := started[c.pid]
		parent, okParent := started[c.ppid]
		if okChild && okParent && child >= parent && c.pid != c.ppid {
			p[int(c.ppid)] = append(p[int(c.ppid)], int(c.pid))
		}
	}
	return p, nil
}

// startTime is when process pid started, in nanoseconds since 1970;
// false when it can't be opened.
func startTime(pid uint32) (int64, bool) {
	h, err := windows.OpenProcess(windows.PROCESS_QUERY_LIMITED_INFORMATION, false, pid)
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
