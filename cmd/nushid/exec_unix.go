//go:build unix

package main

import "syscall"

// replace replaces the process with argv[0], given argv and env.
func replace(argv, env []string) error {
	return syscall.Exec(argv[0], argv, env) //nolint:gosec // the login shell, running this nushid
}
