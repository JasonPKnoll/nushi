package procgroup

import "golang.org/x/sys/windows"

// A job's messages to its completion port aren't guaranteed to arrive
// (JOBOBJECT_ASSOCIATE_COMPLETION_PORT). A lost EXIT_PROCESS would leave a
// process's handle held and its group's count above 0 for good, and a
// doomed process whose NEW_PROCESS is lost would stay doomed; so at each
// exit the port tells of and each kill, the tracker sweeps: it looks for
// held processes that have exited (WaitForSingleObject, no wait) and drops
// them. Not at once, though: a process that has exited may have children
// whose NEW_PROCESS is still in the port, and they find their group through
// it. So the sweep posts a packet of its own (sweepKey) and drops what it
// found once the port gives that packet back, after every message posted
// before it (a port queues its packets first in, first out, and the
// tracker's one reader takes them in that order, which it relies on
// already): its children's, all posted before it exited. A lost NEW_PROCESS can't be made up for: that process, and what
// it starts, isn't tracked, so a cancel misses them, as taskkill's tree
// would miss one whose parent ended first.

// sweepKey is the completion key of a sweep's packet; the job's messages
// carry 0, the key the job's port was associated with.
const sweepKey = 1

// stale is a process a sweep found exited: its pid and the handle it was
// held by, in procs or (doomed) in doomed.
type stale struct {
	pid    uint32
	h      windows.Handle
	doomed bool
}

// sweep notes the held processes that have exited and posts the packet
// that drops them (swept), unless a sweep is already under way, whose
// packet is still to come. Called with t.mu held.
func (t *tracker) sweep() {
	if t.sweeping {
		return
	}
	for pid, p := range t.procs {
		if exited(p.h) {
			t.stale = append(t.stale, stale{pid: pid, h: p.h})
		}
	}
	for pid, d := range t.doomed {
		if exited(d.h) {
			t.stale = append(t.stale, stale{pid: pid, h: d.h, doomed: true})
		}
	}
	if len(t.stale) == 0 {
		return
	}
	if windows.PostQueuedCompletionStatus(t.port, 0, sweepKey, nil) != nil {
		t.stale = nil // the next exit or kill sweeps again
		return
	}
	t.sweeping = true
}

// swept drops what the sweep found, now that the port has given every
// message posted before its packet: each process still held by the same
// handle, which has exited (so a handle closed and given again since names
// no live process).
func (t *tracker) swept() {
	t.mu.Lock()
	defer t.mu.Unlock()
	for _, s := range t.stale {
		if s.doomed {
			if d, ok := t.doomed[s.pid]; ok && d.h == s.h && exited(d.h) {
				_ = windows.CloseHandle(d.h)
				delete(t.doomed, s.pid)
			}
			continue
		}
		if p, ok := t.procs[s.pid]; ok && p.h == s.h && exited(p.h) {
			t.drop(s.pid, p)
		}
	}
	t.stale, t.sweeping = nil, false
}

// doom notes process pid, which h holds and a cancel is killing before
// the job has told of it, so that its group is killed when the job does.
// It holds a handle of its own on it (h is the command's, which Wait
// closes), so its pid stays its and a sweep can tell it has exited; when
// that can't be had it isn't noted, and only the process itself is killed.
func (t *tracker) doom(pid uint32, h windows.Handle, started int64) {
	if _, ok := t.doomed[pid]; ok {
		return
	}
	var own windows.Handle
	self := windows.CurrentProcess()
	if windows.DuplicateHandle(self, h, self, &own, windows.SYNCHRONIZE|windows.PROCESS_QUERY_LIMITED_INFORMATION, false, 0) != nil {
		return
	}
	t.doomed[pid] = doom{h: own, started: started}
}
