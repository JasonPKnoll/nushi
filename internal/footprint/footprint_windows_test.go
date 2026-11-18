package footprint

import (
	"os"
	"runtime"
	"slices"
	"testing"
)

// Of counts memory this process takes for itself: 64 MiB more once it
// has written that much.
func TestOfCountsPrivateBytes(t *testing.T) {
	before := Of(os.Getpid())
	b := make([]byte, 64<<20)
	for i := range len(b) / 4096 {
		b[i*4096] = 1
	}
	after := Of(os.Getpid())
	runtime.KeepAlive(b)
	if before == 0 || after < before+60<<20 {
		t.Errorf("before %d, after %d", before, after)
	}
}

// A process the user can't open (the System process, pid 4) and one that
// isn't there read 0.
func TestOfUnreadable(t *testing.T) {
	for _, pid := range []int{4, 0, -1, 1 << 30} {
		if n := Of(pid); n != 0 {
			t.Errorf("Of(%d) = %d", pid, n)
		}
	}
}

// This process is among its parent's children, the start of each read
// as the walk asked.
func TestSnapshotFindsThisProcess(t *testing.T) {
	p, err := Snapshot()
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Contains(p.Children(os.Getppid()), os.Getpid()) {
		t.Errorf("this test's parent's children: %v", p.Children(os.Getppid()))
	}
}

// A parent id names the child's parent unless that process is known to
// have started after the child: its id was given again.
func TestParented(t *testing.T) {
	for _, c := range []struct {
		parent, child     int64
		okParent, okChild bool
		want              bool
	}{
		{100, 200, true, true, true},
		{200, 200, true, true, true},
		{300, 200, true, true, false},
		{300, 200, false, true, true}, // a start that can't be read keeps it, as proctree does
		{300, 200, true, false, true},
	} {
		if got := parented(c.parent, c.okParent, c.child, c.okChild); got != c.want {
			t.Errorf("%+v: %v", c, got)
		}
	}
}
