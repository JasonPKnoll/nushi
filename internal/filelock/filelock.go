// Package filelock tells whether another process holds a lock on a file,
// without taking it, and takes one as Ante does. Codex and omp lock with
// flock(2). fcntl's F_GETLK sees those on macOS, where flock and POSIX
// locks are one kind; on Linux they are apart and F_GETLK never sees a
// flock lock, so Held also reads /proc/locks, the kernel's list of every
// lock held (held_linux.go). On Windows a lock is LockFileEx's, and none
// is read without taking it (filelock_windows.go, ADR 0123).
package filelock

import (
	"errors"
	"fmt"
	"os"
	"time"
)

// errBusy is tryLock's answer when another process holds the lock.
var errBusy = errors.New("held by another process")

// Held says whether some process holds a lock on the file at path; false
// when there is no such file.
func Held(path string) bool {
	f, err := os.Open(path) //nolint:gosec // an agent's lock file, opened read-only
	if err != nil {
		return false
	}
	defer f.Close() //nolint:errcheck // read-only
	return held(f)
}

// Lock takes path's lock as an agent does (exclusive; the file made if
// missing), trying for up to wait while another process holds it. unlock
// lets it go.
func Lock(path string, wait time.Duration) (unlock func(), err error) {
	f, err := os.OpenFile(path, os.O_RDONLY|os.O_CREATE, 0o644) //nolint:gosec // an agent's lock file beside its settings
	if err != nil {
		return nil, err
	}
	for deadline := time.Now().Add(wait); ; {
		err = tryLock(f)
		if err == nil {
			return func() { _ = f.Close() }, nil // closing lets the lock go
		}
		if !errors.Is(err, errBusy) || time.Now().After(deadline) {
			_ = f.Close() // the lock's error is the one to return
			if errors.Is(err, errBusy) {
				err = fmt.Errorf("%s is held by another process", path)
			}
			return nil, err
		}
		time.Sleep(20 * time.Millisecond)
	}
}
