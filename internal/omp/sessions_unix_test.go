//go:build unix

package omp

import (
	"os"
	"path/filepath"
	"syscall"
	"testing"

	"github.com/afrase/nushi/internal/testcheck"
)

func TestHeld(t *testing.T) {
	dir := t.TempDir()
	testcheck.NoErr(t, os.WriteFile(filepath.Join(dir, "a.lock"), nil, 0o600))
	testcheck.NoErr(t, os.WriteFile(filepath.Join(dir, "b.lock"), nil, 0o600))
	f, err := os.Open(filepath.Join(dir, "a.lock"))
	testcheck.NoErr(t, err)
	t.Cleanup(func() { _ = f.Close() })
	if err := syscall.Flock(int(f.Fd()), syscall.LOCK_EX); err != nil {
		t.Fatal(err)
	}
	if got := held(dir, []string{"a", "b", "../a"}); len(got) != 1 || got["a"] != "another app" {
		t.Errorf("held = %v, want a only", got)
	}
}
