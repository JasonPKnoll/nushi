//go:build darwin || linux

package main

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

// A daemon outside an app is never watched; one inside sees its
// executable go missing.
func TestAppDeleted(t *testing.T) {
	dir := t.TempDir()
	exe, app := packagedDaemon(t, dir)
	if appDeleted(filepath.Join(dir, "bin", "nushid"), time.Millisecond) != nil {
		t.Error("a daemon outside an app is watched")
	}
	missing := exeMissing(exe)
	if missing() {
		t.Error("its executable is missing while there")
	}
	if err := os.RemoveAll(app); err != nil {
		t.Fatal(err)
	}
	if !missing() {
		t.Error("its executable isn't missing with its app deleted")
	}
}

// The watch stops once the executable is missing at two checks in a row,
// not when it's back by the next (an upgrade). The ticks are sent by hand:
// a tick is taken only once the check before it is done, so a stop too
// early is seen at the next tick, never raced.
func TestWhenGone(t *testing.T) {
	tick := make(chan time.Time)
	answers := make(chan bool)
	gone := whenGone(tick, func() bool { return <-answers })
	check := func(missing bool) {
		t.Helper()
		select {
		case tick <- time.Time{}:
		case <-gone:
			t.Fatal("stopped too early")
		}
		answers <- missing
	}

	check(true)
	check(false) // replaced before the next check
	check(true)
	check(true)
	select {
	case <-gone:
	case <-time.After(5 * time.Second):
		t.Fatal("still running with its executable missing twice")
	}
}

// writeEmpty makes the file at path, and the folders it's in.
func writeEmpty(t *testing.T, path string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, nil, 0o600); err != nil {
		t.Fatal(err)
	}
}
