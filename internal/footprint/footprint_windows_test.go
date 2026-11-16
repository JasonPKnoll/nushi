package footprint

import (
	"os"
	"runtime"
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

// The System process is no one's child here: its start can't be read,
// nor can its parent's (pid 0, the idle process, its own parent).
func TestSnapshotLeavesOutUnreadable(t *testing.T) {
	p, err := Snapshot()
	if err != nil {
		t.Fatal(err)
	}
	if len(p[0]) != 0 || len(p[os.Getppid()]) == 0 {
		t.Errorf("idle's children %v; this test's parent's %v", p[0], p[os.Getppid()])
	}
}
