//go:build unix

package logs

import (
	"os"

	"golang.org/x/sys/unix"
)

// toStd makes fds 1 and 2 copies of f's, so os.Stdout, os.Stderr and the
// runtime's crash all write to it.
func toStd(f *os.File) error {
	for _, fd := range []int{1, 2} {
		if err := unix.Dup2(int(f.Fd()), fd); err != nil {
			return err
		}
	}
	return nil
}

// openAppend opens path for appending, made 0600 if missing, as the
// launcher's `>>` does.
func openAppend(path string) (*os.File, error) {
	return os.OpenFile(path, os.O_WRONLY|os.O_APPEND|os.O_CREATE, 0o600) //nolint:gosec // the daemon's own log, in its data folder
}
