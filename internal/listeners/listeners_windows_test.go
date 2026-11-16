package listeners

import (
	"slices"
	"testing"

	"github.com/afrase/nushi/internal/testcheck"
)

// These tests check what is Windows' own; the rest are in
// listeners_test.go and tcptable_test.go.

// TestListReadsMany lists every listener of a process holding more than
// the table's first buffer has room for (4096 bytes: 170 IPv4 rows).
func TestListReadsMany(t *testing.T) {
	pid, ports := listenElsewhere(t, slices.Repeat([]string{"ipv4"}, 200)...)
	ls, err := List()
	testcheck.NoErr(t, err)
	if got := ofPid(ls, pid); len(got) != len(ports) {
		t.Fatalf("listed %d of the helper's %d listeners", len(got), len(ports))
	}
}
