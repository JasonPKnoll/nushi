package main

import (
	"os"
	"path/filepath"
)

// defaultDataFolder is the daemon's data folder unless NUSHI_DATA names
// another: Nushi in the user's local application data (%LOCALAPPDATA%,
// which a roaming profile doesn't copy), else under the home's AppData
// (ADR 0123). Its access is the profile's: the user's, SYSTEM's and the
// administrators'.
func defaultDataFolder(home string) string {
	if dir := os.Getenv("LOCALAPPDATA"); filepath.IsAbs(dir) {
		return filepath.Join(dir, "Nushi")
	}
	return filepath.Join(home, "AppData", "Local", "Nushi")
}
