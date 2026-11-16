package main

import (
	"errors"
	"io/fs"
	"os"
	"time"
)

// appCheck is how often a daemon inside an app looks for its executable.
const appCheck = 30 * time.Second

// appDeleted is closed once the app this daemon runs from is gone: its
// executable missing at two checks in a row, every [appCheck]. So
// uninstalling Nushi (Homebrew's uninstall, or the app dragged to the
// Trash) stops the daemon, while an upgrade or make install, which put a
// new app in its place within seconds, leaves it running (ADR 0055).
// Homebrew can't stop it itself: its signal directive doesn't find an app
// opened with `open -n`. A daemon outside an app (inApp: make run's
// bin/nushid) is never stopped so; nil stop channel then.
func appDeleted(exe string, every time.Duration) <-chan struct{} {
	if !inApp(exe) {
		return nil
	}
	return whenGone(time.Tick(every), exeMissing(exe))
}

// exeMissing reports whether there's no file at exe.
func exeMissing(exe string) func() bool {
	return func() bool {
		_, err := os.Stat(exe)
		return errors.Is(err, fs.ErrNotExist)
	}
}

// whenGone is closed once missing is true at two ticks in a row:
// [appDeleted]'s loop, with its clock and its look at the file passed in.
func whenGone(tick <-chan time.Time, missing func() bool) <-chan struct{} {
	gone := make(chan struct{})
	go func() {
		missed := 0
		for range tick {
			if missing() {
				missed++
			} else {
				missed = 0
			}
			if missed == 2 {
				close(gone)
				return
			}
		}
	}()
	return gone
}
