package procgroup

import (
	"errors"
	"os"
	"runtime"
	"sync"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
)

// Windows has no process groups, and taskkill's tree, which follows each
// process's parent, misses one whose parent ended first. So this process
// (the daemon) puts itself in a job of its own, whose one limit lets a
// process that asks to break away from it do so (allowBreakaway), whose
// completion port tells of each process created below it, in the order
// they are created (JOB_OBJECT_MSG_NEW_PROCESS), and of each exit. The
// tracker reads each new process's parent then, while the parent's entry
// is still there (a parent can't exit before the child it creates is told
// of), and puts the process in its parent's group; each child of this
// process starts a group of its own. A bound command's cancel kills its
// group (ADR 0133), what it started included, whatever has ended between.
// Nothing is killed by a command's own exit or by the daemon's, as on
// Unix. Without the job (refused), a cancel is taskkill's tree (Kill).
// The job's messages may be lost, so tracking is best-effort; what a lost
// exit would leave held, a sweep drops (tracker_sweep_windows.go).

// Job object messages (winnt.h's JOB_OBJECT_MSG_*).
const (
	jobMsgNewProcess          = 6
	jobMsgExitProcess         = 7
	jobMsgAbnormalExitProcess = 8
)

// getQueuedCompletionStatus is kernel32's GetQueuedCompletionStatus, called
// directly rather than through x/sys's wrapper, whose out-parameter is a
// *Overlapped: a job message's "overlapped" is a process id, and a small
// integer in a pointer-typed variable on the stack makes the Go runtime
// throw ("invalid pointer found on stack") when it copies that stack. Here
// it is a uintptr, as hcsshim reads job messages.
var getQueuedCompletionStatus = windows.NewLazySystemDLL("kernel32.dll").NewProc("GetQueuedCompletionStatus")

// isProcessInJob is kernel32's IsProcessInJob, which x/sys doesn't wrap.
var isProcessInJob = windows.NewLazySystemDLL("kernel32.dll").NewProc("IsProcessInJob")

// inJob says whether h's process (opened with query rights) is in job
// (opened with JOB_OBJECT_QUERY; CreateJobObject's handle has every
// right); a process in a job nested in it is in it too (Nested Jobs:
// a child job holds a subset of its parent's processes).
func inJob(h, job windows.Handle) (bool, error) {
	if err := isProcessInJob.Find(); err != nil {
		return false, err
	}
	var in int32 // a BOOL
	if r, _, e := syscall.SyscallN(isProcessInJob.Addr(), uintptr(h), uintptr(job), uintptr(unsafe.Pointer(&in))); r == 0 {
		return false, e
	}
	return in != 0, nil
}

// jobAssociateCompletionPort is JOBOBJECT_ASSOCIATE_COMPLETION_PORT.
type jobAssociateCompletionPort struct {
	CompletionKey  uintptr
	CompletionPort windows.Handle
}

// tracker is what the job told of: the live processes below this one.
type tracker struct {
	self uint32
	job  windows.Handle
	port windows.Handle
	mu   sync.Mutex
	// procs are the live processes, by pid.
	procs map[uint32]tracked
	// roots are each group's first process (a child of this one) by
	// pid, while its group has a process left.
	roots map[uint32]tracked
	// size is how many live processes each group has.
	size map[uint64]int
	// killed are the groups a cancel ended: a process told of later is
	// killed as it comes.
	killed map[uint64]bool
	// doomed are processes a cancel found before the job had told of
	// them, by pid: they're killed with their group when it does.
	doomed map[uint32]doom
	next   uint64
	// stale are what a sweep found exited, dropped once the port has
	// given every message posted before it (tracker_sweep_windows.go).
	stale    []stale
	sweeping bool
}

// doom is a process a cancel killed before the job told of it: its start,
// and a handle of its own on it (SYNCHRONIZE and query rights), which
// keeps its pid its and tells a sweep when it has exited.
type doom struct {
	h       windows.Handle
	started int64
}

// tracked is a process the job told of, by a handle that keeps its pid
// its (a root's, once it has exited, is closed: its start is enough).
type tracked struct {
	h       windows.Handle
	started int64
	group   uint64 // 0: none, its parent wasn't told of (it started before the job)
}

var (
	trackerOnce sync.Once
	theTracker  *tracker
)

// processes is this process's tracker, started the first time it's
// asked for; nil when the job couldn't be set up.
func processes() *tracker {
	trackerOnce.Do(func() { theTracker = startTracker() })
	return theTracker
}

// startTracker puts this process in a job of its own whose completion port
// tells of what is created below it, and reads that port; nil when the
// system refuses either.
func startTracker() *tracker {
	if getQueuedCompletionStatus.Find() != nil {
		return nil
	}
	job, err := windows.CreateJobObject(nil, nil)
	if err != nil {
		return nil
	}
	port, err := windows.CreateIoCompletionPort(windows.InvalidHandle, 0, 0, 1)
	if err != nil {
		_ = windows.CloseHandle(job)
		return nil
	}
	// On the heap, which doesn't move, as the call takes it as a uintptr;
	// kept alive through the call, which a uintptr doesn't do.
	assoc := &jobAssociateCompletionPort{CompletionPort: port}
	_, err = windows.SetInformationJobObject(job, windows.JobObjectAssociateCompletionPortInformation,
		uintptr(unsafe.Pointer(assoc)), uint32(unsafe.Sizeof(*assoc))) //nolint:gosec // the system's own struct, kept alive below
	runtime.KeepAlive(assoc)
	if err == nil {
		err = allowBreakaway(job)
	}
	if err == nil {
		err = windows.AssignProcessToJobObject(job, windows.CurrentProcess())
	}
	if err != nil {
		_ = windows.CloseHandle(port)
		_ = windows.CloseHandle(job)
		return nil
	}
	// The job and its port stay open for as long as this process runs.
	t := &tracker{
		self: windows.GetCurrentProcessId(), job: job, port: port, procs: map[uint32]tracked{}, roots: map[uint32]tracked{},
		size: map[uint64]int{}, killed: map[uint64]bool{}, doomed: map[uint32]doom{},
	}
	go t.read()
	return t
}

// allowBreakaway sets the job's one limit, JOB_OBJECT_LIMIT_BREAKAWAY_OK:
// a process below this one that is started with CREATE_BREAKAWAY_FROM_JOB
// leaves the job (and so the tracker's groups), as it asks, instead of
// failing to start with access denied, as it would in a job without it
// (CreateProcess's flags). Not JOB_OBJECT_LIMIT_SILENT_BREAKAWAY_OK, which
// would take every child out of the job and leave nothing to track.
func allowBreakaway(job windows.Handle) error {
	// On the heap, which doesn't move, as the call takes it as a uintptr.
	info := &windows.JOBOBJECT_EXTENDED_LIMIT_INFORMATION{
		BasicLimitInformation: windows.JOBOBJECT_BASIC_LIMIT_INFORMATION{LimitFlags: windows.JOB_OBJECT_LIMIT_BREAKAWAY_OK},
	}
	_, err := windows.SetInformationJobObject(job, windows.JobObjectExtendedLimitInformation,
		uintptr(unsafe.Pointer(info)), uint32(unsafe.Sizeof(*info))) //nolint:gosec // the system's own struct, kept alive below
	runtime.KeepAlive(info)
	return err
}

// read takes the port's messages, in the order they came, until the port
// is gone (it never is: it lives as long as this process).
func (t *tracker) read() {
	for {
		msg, key, pid, err := dequeue(t.port)
		switch {
		case errors.Is(err, windows.ERROR_ABANDONED_WAIT_0), errors.Is(err, windows.ERROR_INVALID_HANDLE):
			return // the port was closed, or isn't one
		case err != nil:
			continue // a job's messages carry no I/O to fail; a lost one is skipped
		}
		if key == sweepKey {
			t.swept()
			continue
		}
		switch msg {
		case jobMsgNewProcess:
			t.created(pid)
		case jobMsgExitProcess, jobMsgAbnormalExitProcess:
			t.exited(pid)
		}
	}
}

// dequeue waits for the port's next message: its id (a job message's
// "number of bytes"), its key (the job's 0, or sweepKey) and the process
// id it carries (its "overlapped").
// When the call fails it is an error, whether or not it dequeued a packet
// (GetQueuedCompletionStatus's documentation tells the two apart by a
// NULL overlapped; a job never posts a failed one).
func dequeue(port windows.Handle) (msg uint32, key uintptr, pid uint32, err error) {
	var ov uintptr
	r, _, e := syscall.SyscallN(getQueuedCompletionStatus.Addr(), uintptr(port),
		uintptr(unsafe.Pointer(&msg)), uintptr(unsafe.Pointer(&key)), uintptr(unsafe.Pointer(&ov)), uintptr(windows.INFINITE))
	if r == 0 {
		return 0, 0, 0, e
	}
	return msg, key, uint32(ov), nil //nolint:gosec // a job message's "overlapped" is a 32-bit process id
}

// created notes a new process in its parent's group, or a new group when
// its parent is this process, and kills it when that group was killed. One
// that has already gone, and can't be opened, isn't noted, nor is one the
// pid names now that isn't in the job: the process told of has gone and
// its pid is another's, on which no terminate-capable handle is held.
func (t *tracker) created(pid uint32) {
	if pid == t.self {
		return
	}
	h, err := windows.OpenProcess(windows.SYNCHRONIZE|windows.PROCESS_TERMINATE|windows.PROCESS_QUERY_LIMITED_INFORMATION, false, pid)
	if err != nil {
		return
	}
	if in, err := inJob(h, t.job); err != nil || !in {
		_ = windows.CloseHandle(h)
		return
	}
	started := startOf(h)
	parent, okParent := parentOf(h)
	t.mu.Lock()
	defer t.mu.Unlock()
	if old, ok := t.procs[pid]; ok { // its exit was never told of
		t.drop(pid, old)
	}
	p := tracked{h: h, started: started}
	switch pp, ok := t.procs[parent]; {
	case okParent && parent == t.self:
		t.next++
		p.group = t.next
		t.roots[pid] = tracked{started: started, group: p.group}
	case okParent && ok && pp.started <= started:
		p.group = pp.group
	}
	if d, ok := t.doomed[pid]; ok && d.started == started {
		_ = windows.CloseHandle(d.h)
		delete(t.doomed, pid)
		if p.group != 0 {
			t.killed[p.group] = true
		}
		_ = windows.TerminateProcess(h, 1)
	}
	if p.group != 0 {
		t.size[p.group]++
		if t.killed[p.group] {
			_ = windows.TerminateProcess(h, 1)
		}
	}
	t.procs[pid] = p
}

// exited forgets a process that has exited, and a doomed one the job
// never told of; then sweeps for exits the job didn't tell of.
func (t *tracker) exited(pid uint32) {
	t.mu.Lock()
	defer t.mu.Unlock()
	if p, ok := t.procs[pid]; ok {
		t.drop(pid, p)
	}
	if d, ok := t.doomed[pid]; ok {
		_ = windows.CloseHandle(d.h)
		delete(t.doomed, pid)
	}
	t.sweep()
}

// drop forgets process pid, and its group once that has no process left.
func (t *tracker) drop(pid uint32, p tracked) {
	_ = windows.CloseHandle(p.h)
	delete(t.procs, pid)
	if p.group == 0 {
		return
	}
	if t.size[p.group]--; t.size[p.group] > 0 {
		return
	}
	delete(t.size, p.group)
	delete(t.killed, p.group)
	for rpid, r := range t.roots {
		if r.group == p.group {
			delete(t.roots, rpid)
		}
	}
}

// kill kills the group p, a child of this process, started (when the job
// has told of it), what is left of it once p has exited included; p alone
// before the job has told of it, and the rest of its group as the job
// does. Once p and all it started have exited, or Wait has let go of p,
// nothing is killed, and the answer is os.ErrProcessDone.
func (t *tracker) kill(p *os.Process) error {
	err := os.ErrProcessDone
	if p.WithHandle(func(h uintptr) { err = t.killGroupOf(windows.Handle(h), p.Pid) }) != nil {
		return os.ErrProcessDone
	}
	return err
}

func (t *tracker) killGroupOf(h windows.Handle, pid int) error {
	started := startOf(h)
	t.mu.Lock()
	defer t.mu.Unlock()
	t.sweep()
	if r, ok := t.roots[uint32(pid)]; ok && r.started == started { //nolint:gosec // a Windows pid is 32-bit
		t.killed[r.group] = true
		done := os.ErrProcessDone
		for _, q := range t.procs {
			if q.group == r.group && windows.TerminateProcess(q.h, 1) == nil {
				done = nil
			}
		}
		return done
	}
	if exited(h) { // its group, if it had one, is empty
		return os.ErrProcessDone
	}
	t.doom(uint32(pid), h, started) //nolint:gosec // a Windows pid is 32-bit
	return terminate(h)
}

// startOf is when h's process started, in nanoseconds since 1970; 0
// when it can't be read.
func startOf(h windows.Handle) int64 {
	var created, exitedAt, kernel, user windows.Filetime
	if windows.GetProcessTimes(h, &created, &exitedAt, &kernel, &user) != nil {
		return 0
	}
	return created.Nanoseconds()
}

// parentOf is the id of the process that created h's.
func parentOf(h windows.Handle) (uint32, bool) {
	var bi windows.PROCESS_BASIC_INFORMATION
	if windows.NtQueryInformationProcess(h, windows.ProcessBasicInformation, unsafe.Pointer(&bi), uint32(unsafe.Sizeof(bi)), nil) != nil { //nolint:gosec // the system's own struct
		return 0, false
	}
	return uint32(bi.InheritedFromUniqueProcessId), true //nolint:gosec // a Windows pid is 32-bit
}
