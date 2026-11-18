package procgroup

import (
	"bufio"
	"context"
	"os/exec"
	"strings"
	"syscall"
	"testing"
	"time"

	"golang.org/x/sys/windows"

	"github.com/afrase/nushi/internal/testcheck"
)

// theTrackerOrFail is this process's tracker, which the tests need.
func theTrackerOrFail(t *testing.T) *tracker {
	t.Helper()
	tr := processes()
	if tr == nil {
		t.Fatal("the system refused the tracker's job")
	}
	return tr
}

// A process started with CREATE_BREAKAWAY_FROM_JOB below the tracker's job
// starts, outside it, as it asks; one started without it is in the job.
func TestTrackerAllowsBreakaway(t *testing.T) {
	tr := theTrackerOrFail(t)
	for _, c := range []struct {
		flags uint32
		in    bool
	}{{0, true}, {windows.CREATE_BREAKAWAY_FROM_JOB, false}} {
		cmd := exec.Command("ping", "-n", "30", "127.0.0.1")
		cmd.SysProcAttr = &syscall.SysProcAttr{CreationFlags: c.flags}
		if err := cmd.Start(); err != nil {
			t.Fatalf("flags %#x: %v", c.flags, err)
		}
		var in bool
		var err error
		testcheck.NoErr(t, cmd.Process.WithHandle(func(h uintptr) { in, err = inJob(windows.Handle(h), tr.job) }))
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
		testcheck.NoErr(t, err)
		if in != c.in {
			t.Errorf("flags %#x: in the job %v, want %v", c.flags, in, c.in)
		}
	}
}

// A pid told of that names a process outside the job (the one told of
// has gone, and its pid is another's) isn't tracked.
func TestTrackerSkipsAPidOutsideTheJob(t *testing.T) {
	tr := theTrackerOrFail(t)
	cmd := exec.Command("ping", "-n", "30", "127.0.0.1")
	cmd.SysProcAttr = &syscall.SysProcAttr{CreationFlags: windows.CREATE_BREAKAWAY_FROM_JOB}
	testcheck.NoErr(t, cmd.Start())
	defer func() {
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
	}()
	pid := uint32(cmd.Process.Pid) //nolint:gosec // a Windows pid is 32-bit
	tr.created(pid)
	tr.mu.Lock()
	_, tracked := tr.procs[pid]
	tr.mu.Unlock()
	if tracked {
		t.Error("a process outside the job was tracked")
	}
}

// under runs f with the tracker's lock held.
func under(tr *tracker, f func()) {
	tr.mu.Lock()
	defer tr.mu.Unlock()
	f()
}

// groupGone says the tracker keeps nothing of group g, nor of pid as a
// root or a doomed process.
func groupGone(tr *tracker, g uint64, pid uint32) bool {
	gone := true
	under(tr, func() {
		for _, p := range tr.procs {
			gone = gone && p.group != g
		}
		_, sized := tr.size[g]
		_, root := tr.roots[pid]
		_, doomed := tr.doomed[pid]
		gone = gone && !sized && !tr.killed[g] && !root && !doomed
	})
	return gone
}

// A cancelled command's group, what it left behind its exited parent
// included, is killed whole, and the tracker then holds nothing of it.
func TestTrackerKillsAGroupAndLetsGo(t *testing.T) {
	tr := theTrackerOrFail(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	cmd := helper(ctx, "parent")
	out, err := cmd.StdoutPipe()
	testcheck.NoErr(t, err)
	Bound(cmd)
	testcheck.NoErr(t, cmd.Start())
	r := bufio.NewReader(out)
	ping := orphanPing(t, r)
	for {
		line, err := r.ReadString('\n')
		testcheck.NoErr(t, err)
		if strings.TrimSpace(line) == "orphaned" {
			break
		}
	}
	pid := uint32(cmd.Process.Pid) //nolint:gosec // a Windows pid is 32-bit
	var g uint64
	under(tr, func() { g = tr.roots[pid].group })
	if g == 0 {
		t.Fatal("the command has no group")
	}
	cancel()
	_ = cmd.Wait()
	if !gone(ping, 5*time.Second) {
		t.Fatal("the orphaned ping outlived the cancel")
	}
	if !testcheck.Eventually(func() bool { return groupGone(tr, g, pid) }) {
		t.Error("the tracker still holds some of the killed group")
	}
}

// exitedProcess is a process that has exited, by a handle that keeps its
// pid its, for the test to give the tracker (whose drop closes it). It
// broke away from the job, so the job tells nothing of it: what the
// tracker holds of it is only what the test puts there.
func exitedProcess(t *testing.T) (pid uint32, h windows.Handle) {
	t.Helper()
	cmd := exec.Command("cmd.exe", "/c", "exit")
	cmd.SysProcAttr = &syscall.SysProcAttr{CreationFlags: windows.CREATE_BREAKAWAY_FROM_JOB}
	testcheck.NoErr(t, cmd.Start())
	pid = uint32(cmd.Process.Pid) //nolint:gosec // a Windows pid is 32-bit
	self := windows.CurrentProcess()
	testcheck.NoErr(t, cmd.Process.WithHandle(func(ph uintptr) {
		testcheck.NoErr(t, windows.DuplicateHandle(self, windows.Handle(ph), self, &h, windows.SYNCHRONIZE|windows.PROCESS_QUERY_LIMITED_INFORMATION, false, 0))
	}))
	testcheck.NoErr(t, cmd.Wait())
	return pid, h
}

// A process whose exit the job didn't tell of (its message lost) is let go
// of by the next sweep, its group's counts with it; so is a doomed process
// the job never told of.
func TestTrackerSweepsWhatItWasntToldOf(t *testing.T) {
	tr := theTrackerOrFail(t)
	pid, h := exitedProcess(t)
	dpid, dh := exitedProcess(t)
	var g uint64
	under(tr, func() {
		tr.next++
		g = tr.next
		tr.procs[pid] = tracked{h: h, group: g}
		tr.roots[pid] = tracked{group: g}
		tr.size[g] = 1
		tr.killed[g] = true
		tr.doomed[dpid] = doom{h: dh}
	})
	let := func() bool {
		under(tr, tr.sweep) // again while one under way (another exit's) didn't see these
		doomed := false
		under(tr, func() { _, doomed = tr.doomed[dpid] })
		return groupGone(tr, g, pid) && !doomed
	}
	if !testcheck.Eventually(let) {
		t.Error("the sweep left an exited process held")
	}
}
