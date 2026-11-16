//go:build !windows

package batchfile

import "os/exec"

// Guard does nothing: only Windows runs a batch file through cmd.exe.
func Guard(*exec.Cmd) {}
