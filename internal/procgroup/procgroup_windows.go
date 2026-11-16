package procgroup

import (
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"syscall"

	"golang.org/x/sys/windows"

	"github.com/afrase/nushi/internal/batchfile"
)

// Own guards cmd against cmd.exe (batchfile.Guard), as every command
// Nushi starts passes here: an npm shim runs as node and its script, and
// nothing runs from the current folder by a bare name. Windows has no
// process group a child joins by itself, so Kill finds what cmd started by
// its parents instead (ADR 0123).
func Own(cmd *exec.Cmd) { batchfile.Guard(cmd) }

// Kill kills p's process tree with taskkill's /T, which follows each
// process's parent from p down. One whose parent ended before the kill is
// no longer found from p, and lives on. p's handle is held meanwhile, so
// its pid can't have been given to another process: once p has exited, or
// Wait has let go of it, nothing is killed, and the answer is
// os.ErrProcessDone.
func Kill(p *os.Process) error {
	err := os.ErrProcessDone
	if p.WithHandle(func(h uintptr) { err = killTree(windows.Handle(h), p.Pid) }) != nil {
		return os.ErrProcessDone // Wait let go of it: it has exited
	}
	return err
}

// killTree kills the tree of pid, whose handle is h, unless it has exited.
func killTree(h windows.Handle, pid int) error {
	if ev, err := windows.WaitForSingleObject(h, 0); err == nil && ev == windows.WAIT_OBJECT_0 {
		return os.ErrProcessDone
	}
	kill := exec.Command(taskkill(), "/T", "/F", "/PID", strconv.Itoa(pid)) //nolint:gosec,noctx // the system's own taskkill, a pid; it doesn't wait on anything
	if kill.Run() != nil {
		return windows.TerminateProcess(h, 1) // taskkill missing or refused: at least the process itself
	}
	return nil
}

// taskkill is the system's taskkill, by its full path so PATH can't name
// another.
func taskkill() string {
	root := os.Getenv("SystemRoot")
	if root == "" {
		root = `C:\Windows`
	}
	return filepath.Join(root, "System32", "taskkill.exe")
}

// Session has cmd start with no console window: the daemon has no
// console of its own to share, and a console program started without one
// would open a window of its own.
func Session(cmd *exec.Cmd) {
	if cmd.SysProcAttr == nil {
		cmd.SysProcAttr = &syscall.SysProcAttr{}
	}
	cmd.SysProcAttr.CreationFlags |= windows.CREATE_NO_WINDOW
}
