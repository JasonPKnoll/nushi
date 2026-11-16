//go:build unix

package main

import (
	"os"
	"syscall"

	"golang.org/x/sys/unix"
)

// signalName is the signal that stopped the daemon by its name, "SIGTERM".
func signalName(sig os.Signal) string {
	if s, ok := sig.(syscall.Signal); ok {
		return unix.SignalName(s)
	}
	return sig.String()
}
