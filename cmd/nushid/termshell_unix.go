//go:build unix

package main

import "os"

// terminalShell is the program a terminal starts and its arguments: the
// user's login shell, as a login shell.
func terminalShell() []string { return []string{loginShell(), "-l"} }

// terminalBase is the environment a terminal's starts from: the daemon's.
// The login shell builds the rest of its own.
func terminalBase(interface{ Environ() []string }) []string { return os.Environ() }
