//go:build unix

package testcheck

import (
	"syscall"
	"testing"
)

// Fifo makes a named pipe at path, for a test that something waits on
// none; failing to fails the test.
func Fifo(t *testing.T, path string) {
	t.Helper()
	NoErr(t, syscall.Mkfifo(path, 0o600))
}
