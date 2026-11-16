//go:build unix

package claudehistory

import "syscall"

// makeFifo makes a FIFO at path, for TestWorkflowFilesStayInside.
var makeFifo = func(path string) error { return syscall.Mkfifo(path, 0o600) }
