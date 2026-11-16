//go:build !unix

package testcheck

import "testing"

// Fifo skips the test: Windows has no named pipe in a folder, so what a
// test of one checks can't happen there.
func Fifo(t *testing.T, _ string) {
	t.Helper()
	t.Skip("no fifos here")
}
