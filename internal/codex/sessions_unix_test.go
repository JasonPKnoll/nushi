//go:build unix

package codex

import (
	"os"
	"path/filepath"
	"testing"

	"golang.org/x/sys/unix"
)

// A thread is open elsewhere while its lock file is locked.
func TestHeld(t *testing.T) {
	dir := t.TempDir()
	for _, id := range []string{"held", "free"} {
		if err := os.WriteFile(filepath.Join(dir, id+".lock"), nil, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	f, err := os.Open(filepath.Join(dir, "held.lock"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = f.Close() })
	if err := unix.Flock(int(f.Fd()), unix.LOCK_EX); err != nil { // as Codex holds it
		t.Fatal(err)
	}
	got := held(dir, []string{"held", "free", "none", "../held"})
	if len(got) != 1 || got["held"] != "another app" {
		t.Errorf("held = %v, want only held", got)
	}
}
