package procgroup

import (
	"os/exec"
	"syscall"
	"testing"

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
