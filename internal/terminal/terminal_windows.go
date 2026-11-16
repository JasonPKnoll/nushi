package terminal

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"sync"
	"time"
	"unicode/utf16"
	"unsafe"

	"golang.org/x/sys/windows"

	"github.com/afrase/nushi/internal/batchfile"
	"github.com/afrase/nushi/internal/model"
	"github.com/afrase/nushi/internal/proctree"
)

// On Windows (ADR 0132) the pseudo-terminal is ConPTY's pseudo-console: the
// program runs attached to it, and the daemon holds the other ends of two
// pipes, one the program reads (Terminal's in) and one it writes (out).
// Go's exec can't attach a program to one, so Start calls CreateProcess
// itself, on the command line exec and batchfile.Guard would have used.
// The program starts suspended and joins a job object before it runs, so
// all it starts is in the job, which Close ends. Windows has no
// foreground process group: the foreground program is the newest process
// under the shell.

// sys is a Windows terminal's own: its pseudo-console, the job its
// program's tree runs in, and the daemon's ends of the pipes.
type sys struct {
	console     windows.Handle
	job         windows.Handle
	conIn       *os.File // written: what the program reads
	conOut      *os.File // read: what the program writes
	consoleOnce sync.Once
}

// ctrlC is what typing Ctrl+C sends: ConPTY makes it the console's
// Ctrl+C event for what runs on it.
const ctrlC = "\x03"

// Start runs argv in dir with env, in a terminal of cols × rows. changed is
// called when the foreground program changes and when the program exits.
func Start(argv []string, dir string, env []string, cols, rows int, changed func()) (*Terminal, error) {
	size, err := coord(cols, rows)
	if err != nil {
		return nil, err
	}
	app, line, env, err := startLine(argv, dir, env)
	if err != nil {
		return nil, err
	}
	var inRead, inWrite, outRead, outWrite windows.Handle
	if err := windows.CreatePipe(&inRead, &inWrite, nil, 0); err != nil {
		return nil, err
	}
	if err := windows.CreatePipe(&outRead, &outWrite, nil, 0); err != nil {
		closeAll(inRead, inWrite)
		return nil, err
	}
	var console windows.Handle
	err = windows.CreatePseudoConsole(size, inRead, outWrite, 0, &console)
	closeAll(inRead, outWrite) // the pseudo-console holds its own copies
	if err != nil {
		closeAll(inWrite, outRead)
		return nil, fmt.Errorf("creating a pseudo-console: %w", err)
	}
	t := &Terminal{console: console, conIn: os.NewFile(uintptr(inWrite), "conpty-in"), conOut: os.NewFile(uintptr(outRead), "conpty-out"),
		changed: changed, done: make(chan struct{}), subs: map[*sub]struct{}{}}
	t.out, t.in = t.conOut, t.conIn
	if err := t.spawn(app, line, dir, env); err != nil {
		t.closeConsole()
		_ = t.conIn.Close()
		_ = t.conOut.Close()
		return nil, err
	}
	read := make(chan struct{})
	go t.read(read)
	go t.wait(read)
	go t.watch()
	return t, nil
}

// coord is a terminal size as the pseudo-console takes it, within Unix's
// bounds.
func coord(cols, rows int) (windows.Coord, error) {
	if cols <= 0 || rows <= 0 || cols > 1000 || rows > 1000 {
		return windows.Coord{}, errors.New("bad terminal size")
	}
	return windows.Coord{X: int16(cols), Y: int16(rows)}, nil
}

// startLine is how argv starts: the program's path, its command line
// and its environment, as exec would start it once batchfile.Guard has
// had its say (an npm shim runs as node and its script; the current
// folder is kept out of the program's search).
func startLine(argv []string, dir string, env []string) (app, line string, progEnv []string, err error) {
	if len(argv) == 0 {
		return "", "", nil, errors.New("no program")
	}
	cmd := exec.Command(argv[0], argv[1:]...) //nolint:gosec,noctx // only read for its path, arguments and environment; Start starts it
	cmd.Dir, cmd.Env = dir, env
	batchfile.Guard(cmd)
	if cmd.Err != nil {
		return "", "", nil, cmd.Err
	}
	line = windows.ComposeCommandLine(cmd.Args)
	if cmd.SysProcAttr != nil && cmd.SysProcAttr.CmdLine != "" {
		line = cmd.SysProcAttr.CmdLine
	}
	return cmd.Path, line, cmd.Environ(), nil
}

// spawn starts the program on the pseudo-console, suspended until it has
// joined the job, and takes it as t's process.
func (t *Terminal) spawn(app, line, dir string, env []string) error {
	attrs, err := windows.NewProcThreadAttributeList(1)
	if err != nil {
		return err
	}
	defer attrs.Delete()
	// The attribute's value is the pseudo-console's handle itself, not a
	// pointer to one.
	if err := attrs.Update(windows.PROC_THREAD_ATTRIBUTE_PSEUDOCONSOLE, *(*unsafe.Pointer)(unsafe.Pointer(&t.console)), unsafe.Sizeof(t.console)); err != nil { //nolint:gosec // the handle as a pointer-sized value, as the attribute takes it
		return err
	}
	si := windows.StartupInfoEx{ProcThreadAttributeList: attrs.List()}
	si.Cb = uint32(unsafe.Sizeof(si))
	// Standard handles of none: without this the program would get the
	// daemon's own (its log), not the pseudo-console's.
	si.Flags = windows.STARTF_USESTDHANDLES
	appW, err := windows.UTF16PtrFromString(app)
	if err != nil {
		return err
	}
	lineW, err := windows.UTF16FromString(line)
	if err != nil {
		return err
	}
	var dirW *uint16
	if dir != "" {
		if dirW, err = windows.UTF16PtrFromString(dir); err != nil {
			return err
		}
	}
	block, err := envBlock(env)
	if err != nil {
		return err
	}
	job, err := killJob()
	if err != nil {
		return err
	}
	var pi windows.ProcessInformation
	flags := uint32(windows.EXTENDED_STARTUPINFO_PRESENT | windows.CREATE_UNICODE_ENVIRONMENT | windows.CREATE_SUSPENDED)
	err = withCtrlC(func() error {
		return windows.CreateProcess(appW, &lineW[0], nil, nil, false, flags, &block[0], dirW, &si.StartupInfo, &pi)
	})
	runtime.KeepAlive(attrs)
	if err != nil {
		_ = windows.CloseHandle(job)
		return fmt.Errorf("starting %s: %w", filepath.Base(app), err)
	}
	defer closeAll(pi.Thread, pi.Process) // os.FindProcess holds its own
	if err := windows.AssignProcessToJobObject(job, pi.Process); err != nil {
		_ = windows.TerminateProcess(pi.Process, 1)
		_ = windows.CloseHandle(job)
		return err
	}
	// Found while the process's handle above is still open, so its id
	// can't name another yet.
	proc, err := os.FindProcess(int(pi.ProcessId))
	if err != nil {
		_ = windows.CloseHandle(job) // ends it
		return err
	}
	if _, err := windows.ResumeThread(pi.Thread); err != nil {
		_ = windows.CloseHandle(job)
		_ = proc.Release()
		return err
	}
	t.job, t.proc = job, proc
	return nil
}

var (
	setConsoleCtrlHandler = windows.NewLazySystemDLL("kernel32.dll").NewProc("SetConsoleCtrlHandler")
	ctrlCMu               sync.Mutex
)

// ignoreCtrlC is the process's ignore-Ctrl+C flag in its parameters'
// ConsoleFlags, which SetConsoleCtrlHandler(NULL, TRUE) sets.
const ignoreCtrlC = 1

// ignoresCtrlC says whether this process ignores Ctrl+C. A process
// started in a process group of its own does (the app starts the daemon
// so), and what it starts inherits that.
func ignoresCtrlC() bool {
	return windows.RtlGetCurrentPeb().ProcessParameters.ConsoleFlags&ignoreCtrlC != 0
}

// withCtrlC runs start (a CreateProcess) with Ctrl+C on in this process,
// so the program it starts inherits it on, then puts this process's flag
// back: a terminal's Ctrl+C must reach what runs on it, while the daemon,
// which stops on Ctrl+C (os.Interrupt), keeps ignoring one on its own
// console. A daemon started from a terminal, which takes Ctrl+C, is left
// as it is.
func withCtrlC(start func() error) error {
	ctrlCMu.Lock()
	defer ctrlCMu.Unlock()
	if !ignoresCtrlC() {
		return start()
	}
	setConsoleCtrlHandler.Call(0, 0)       //nolint:errcheck // put back by the defer below
	defer setConsoleCtrlHandler.Call(0, 1) //nolint:errcheck // as it was
	return start()
}

// envBlock is env as CreateProcess takes it: each variable, ended by a
// NUL, and one more NUL.
func envBlock(env []string) ([]uint16, error) {
	var block []uint16
	for _, v := range env {
		if strings.IndexByte(v, 0) >= 0 {
			return nil, fmt.Errorf("a NUL in the environment variable %q", v)
		}
		block = append(block, utf16.Encode([]rune(v))...)
		block = append(block, 0)
	}
	if len(block) == 0 {
		block = append(block, 0)
	}
	return append(block, 0), nil
}

// killJob is a job object whose processes are killed when it is closed
// (as procgroup's).
func killJob() (windows.Handle, error) {
	job, err := windows.CreateJobObject(nil, nil)
	if err != nil {
		return 0, err
	}
	// On the heap, which doesn't move, as the call takes it as a uintptr.
	info := &windows.JOBOBJECT_EXTENDED_LIMIT_INFORMATION{
		BasicLimitInformation: windows.JOBOBJECT_BASIC_LIMIT_INFORMATION{LimitFlags: windows.JOB_OBJECT_LIMIT_KILL_ON_JOB_CLOSE},
	}
	_, err = windows.SetInformationJobObject(job, windows.JobObjectExtendedLimitInformation,
		uintptr(unsafe.Pointer(info)), uint32(unsafe.Sizeof(*info))) //nolint:gosec // the system's own struct, kept alive below
	runtime.KeepAlive(info)
	if err != nil {
		_ = windows.CloseHandle(job)
		return 0, err
	}
	return job, nil
}

// closeAll closes handles, those that are set.
func closeAll(hs ...windows.Handle) {
	for _, h := range hs {
		if h != 0 && h != windows.InvalidHandle {
			_ = windows.CloseHandle(h)
		}
	}
}

// closeConsole closes the pseudo-console, once: what is attached to it
// gets CTRL_CLOSE_EVENT, and once the console host has written the last of
// its output, the output pipe ends.
func (t *Terminal) closeConsole() {
	t.consoleOnce.Do(func() { windows.ClosePseudoConsole(t.console) })
}

// waitProgram waits for the program to exit, and says its exit code. The
// pseudo-console is closed then: its output pipe doesn't end before.
func (t *Terminal) waitProgram() int {
	code := -1
	if st, err := t.proc.Wait(); err == nil {
		code = st.ExitCode()
	}
	t.closeConsole()
	return code
}

// foreground returns the newest process under the program, or 0 while it
// has none: what the user is running at the shell's prompt.
func (t *Terminal) foreground() int {
	ps, err := proctree.Tree{}.Under(t.proc.Pid)
	if err != nil || len(ps) == 0 {
		return 0
	}
	newest := slices.MaxFunc(ps, func(a, b model.Process) int { return a.Started.Compare(b.Started) })
	return newest.Pid
}

// Terminate asks the foreground program to end: Ctrl+C, as typed. A dev
// server's stop sends it before Close (ADR 0098).
func (t *Terminal) Terminate() error {
	select {
	case <-t.done:
		return nil
	default:
	}
	_, err := t.conIn.WriteString(ctrlC)
	return err
}

// Resize sets the terminal's size.
func (t *Terminal) Resize(cols, rows int) error {
	size, err := coord(cols, rows)
	if err != nil {
		return err
	}
	select {
	case <-t.done:
		return nil // the pseudo-console is closed
	default:
	}
	return windows.ResizePseudoConsole(t.console, size)
}

// hangUp is Close's work: the pseudo-console closed, a grace for what ran
// on it to end, then the job ended, with all that is left in it.
func (t *Terminal) hangUp() {
	t.closeConsole()
	select {
	case <-t.done:
	case <-time.After(grace):
	}
	_ = windows.TerminateJobObject(t.job, 1)
	<-t.done
	_ = windows.CloseHandle(t.job)
	_ = t.conIn.Close()
	_ = t.conOut.Close()
	_ = t.proc.Release()
}

// name returns a process's program name without its folder, or "".
func name(pid int) string {
	h, err := windows.OpenProcess(windows.PROCESS_QUERY_LIMITED_INFORMATION, false, uint32(pid)) //nolint:gosec // a pid fits
	if err != nil {
		return ""
	}
	defer windows.CloseHandle(h) //nolint:errcheck // read only
	buf := make([]uint16, windows.MAX_LONG_PATH)
	n := uint32(len(buf)) //nolint:gosec // MAX_LONG_PATH fits
	if windows.QueryFullProcessImageName(h, 0, &buf[0], &n) != nil {
		return ""
	}
	return filepath.Base(windows.UTF16ToString(buf[:n]))
}
