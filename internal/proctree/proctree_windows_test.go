package proctree

import (
	"os"
	"os/exec"
	"slices"
	"testing"
	"time"

	"github.com/afrase/nushi/internal/model"
	"github.com/afrase/nushi/internal/testcheck"
)

// sleeperEnv makes this test binary, run again as a child, sleep instead
// of running tests.
const sleeperEnv = "NUSHI_PROCTREE_SLEEPER"

func TestMain(m *testing.M) {
	if os.Getenv(sleeperEnv) == "1" {
		time.Sleep(time.Minute)
		os.Exit(0)
	}
	m.Run()
}

// sleeper starts this test binary as a sleeping child with args.
func sleeper(t *testing.T, args ...string) *exec.Cmd {
	t.Helper()
	exe, err := os.Executable()
	testcheck.NoErr(t, err)
	cmd := exec.Command(exe, args...)
	cmd.Env = append(os.Environ(), sleeperEnv+"=1")
	testcheck.NoErr(t, cmd.Start())
	done := make(chan struct{})
	go func() { _ = cmd.Wait(); close(done) }()
	t.Cleanup(func() {
		_ = cmd.Process.Kill()
		<-done
	})
	return cmd
}

func TestUnderReadsChildrenAndTheirCommandLines(t *testing.T) {
	cmd := sleeper(t, "a b", `quo"te`, "")
	var got model.Process
	if !testcheck.Eventually(func() bool {
		ps, err := (Tree{}).Under(os.Getpid())
		testcheck.NoErr(t, err)
		i := slices.IndexFunc(ps, func(p model.Process) bool { return p.Pid == cmd.Process.Pid })
		if i >= 0 {
			got = ps[i]
		}
		return i >= 0
	}) {
		t.Fatal("the child isn't under this process")
	}
	if want := []string{"a b", `quo"te`, ""}; !slices.Equal(got.Args[1:], want) {
		t.Errorf("args %q, want %q after the program", got.Args, want)
	}
	if got.Started.IsZero() || time.Since(got.Started) > time.Minute || got.Pgid != 0 {
		t.Errorf("started %v, pgid %d", got.Started, got.Pgid)
	}
	again, err := (Tree{}).Get(cmd.Process.Pid)
	testcheck.NoErr(t, err)
	if !again.Started.Equal(got.Started) {
		t.Errorf("start time %v, then %v", got.Started, again.Started)
	}
}

func TestStopEndsTheSameProcessOnly(t *testing.T) {
	defer func(g time.Duration) { Grace = g }(Grace)
	Grace = 100 * time.Millisecond
	cmd := sleeper(t)
	p, err := (Tree{}).Get(cmd.Process.Pid)
	testcheck.NoErr(t, err)

	other := p
	other.Started = p.Started.Add(-time.Second) // the same pid, another process
	(Tree{}).Stop([]model.Process{other})
	if _, err := (Tree{}).Get(p.Pid); err != nil {
		t.Fatal("a process with another start time was stopped")
	}

	(Tree{}).Stop([]model.Process{p})
	if !testcheck.Eventually(func() bool { _, err := (Tree{}).Get(p.Pid); return err != nil }) {
		t.Error("the process still runs")
	}
}

func TestStopGroupStopsNoneOnWindows(t *testing.T) {
	cmd := sleeper(t)
	p, err := (Tree{}).Get(cmd.Process.Pid)
	testcheck.NoErr(t, err)
	if err := (Tree{}).StopGroup(os.Getpid(), p); err == nil {
		t.Error("a group stopped where there are none")
	}
}

func TestParentedDropsAParentStartedAfterItsChild(t *testing.T) {
	t0 := time.Now()
	ps := parented([]proc{
		{Pid: 1, Started: t0, ppid: 0},
		{Pid: 2, Started: t0.Add(time.Second), ppid: 1},  // 1's child
		{Pid: 3, Started: t0.Add(-time.Second), ppid: 1}, // older than "its parent": the id was used again
		{Pid: 4, ppid: 1},                                // its start unread: kept
	})
	if got := under(ps, 1); len(got) != 2 || got[0].Pid != 2 || got[1].Pid != 4 {
		t.Errorf("under 1: %+v", got)
	}
}

func TestArgsOfNoProcess(t *testing.T) {
	if got := Args(0); got != nil {
		t.Errorf("Args(0) = %q", got)
	}
}
