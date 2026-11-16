//go:build !windows

package main

import "os"

// ownDataFolder makes the data folder the user's alone: 0700, whatever
// it was. A folder of someone else's can't be changed, and stops the
// start.
func ownDataFolder(dir string) error {
	return os.Chmod(dir, 0o700) //nolint:gosec // a folder: 0700 is owner-only
}
