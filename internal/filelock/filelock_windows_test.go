package filelock

import (
	"path/filepath"
	"testing"
	"time"
)

// A second Lock waits for the first to let go, and fails once its wait is
// over.
func TestLock(t *testing.T) {
	path := filepath.Join(t.TempDir(), "settings.json.lock")
	unlock, err := Lock(path, 0)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Lock(path, 50*time.Millisecond); err == nil {
		t.Error("locked twice")
	}
	go func() { time.Sleep(50 * time.Millisecond); unlock() }()
	again, err := Lock(path, 2*time.Second) // waits for the first to let go
	if err != nil {
		t.Fatal(err)
	}
	again()
}
