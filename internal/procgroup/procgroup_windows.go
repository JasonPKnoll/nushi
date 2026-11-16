package procgroup

import (
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"sync"
	"syscall"
	"unsafe"

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

// Group is a command Start started, in a job object, which the processes
// it starts join, so Kill ends them all, one whose parent ended first
// included, as a Unix process group's kill does; the job also kills them
// if this process dies. One the command starts before it joins the job (a
// moment after it starts) is left. Without a job (the system refused
// one), Kill is the package's Kill.
type Group struct {
	p   *os.Process
	mu  sync.Mutex
	job windows.Handle // 0 when there is none, or once Kill closed it
}

func hold(p *os.Process) *Group {
	g := &Group{p: p}
	job, err := killJob()
	if err != nil {
		return g
	}
	if p.WithHandle(func(h uintptr) { err = windows.AssignProcessToJobObject(job, windows.Handle(h)) }) != nil || err != nil {
		_ = windows.CloseHandle(job) // it exited already, or the system refused
		return g
	}
	g.job = job
	return g
}

// Kill ends the job, what is left of it once the command has exited too,
// and lets go of it; with no job, the command's tree (Kill).
func (g *Group) Kill() error {
	g.mu.Lock()
	defer g.mu.Unlock()
	if g.job == 0 {
		return Kill(g.p)
	}
	err := windows.TerminateJobObject(g.job, 1)
	_ = windows.CloseHandle(g.job) // kills anything that joined since
	g.job = 0
	return err
}

// killJob is a job object whose processes are killed when it is closed.
func killJob() (windows.Handle, error) {
	job, err := windows.CreateJobObject(nil, nil)
	if err != nil {
		return 0, err
	}
	info := windows.JOBOBJECT_EXTENDED_LIMIT_INFORMATION{
		BasicLimitInformation: windows.JOBOBJECT_BASIC_LIMIT_INFORMATION{LimitFlags: windows.JOB_OBJECT_LIMIT_KILL_ON_JOB_CLOSE},
	}
	if _, err := windows.SetInformationJobObject(job, windows.JobObjectExtendedLimitInformation,
		uintptr(unsafe.Pointer(&info)), uint32(unsafe.Sizeof(info))); err != nil {
		_ = windows.CloseHandle(job)
		return 0, err
	}
	return job, nil
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
