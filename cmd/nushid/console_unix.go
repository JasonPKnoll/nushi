//go:build unix

package main

// hideConsole does nothing on Unix: a program started there opens no
// window of its own (console_windows.go).
func hideConsole() (relaunched bool, err error) { return false, nil }
