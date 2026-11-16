//go:build unix

package filelock

import (
	"errors"
	"os"

	"golang.org/x/sys/unix"
)

// posixHeld says fcntl's F_GETLK finds a lock on f that another process
// holds.
func posixHeld(f *os.File) bool {
	lk := unix.Flock_t{Type: unix.F_WRLCK}
	return unix.FcntlFlock(f.Fd(), unix.F_GETLK, &lk) == nil && lk.Type != unix.F_UNLCK
}

// tryLock takes f's flock(2) lock, exclusive, without waiting: errBusy
// while another process holds it.
func tryLock(f *os.File) error {
	err := unix.Flock(int(f.Fd()), unix.LOCK_EX|unix.LOCK_NB)
	if errors.Is(err, unix.EWOULDBLOCK) {
		return errBusy
	}
	return err
}

// release lets f's lock go, by closing f: flock(2)'s lock goes with the
// last descriptor of its open file.
func release(f *os.File) { _ = f.Close() }
