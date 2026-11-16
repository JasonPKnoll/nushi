package terminal

import (
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"testing"
	"time"

	"github.com/afrase/nushi/internal/model"
	"github.com/afrase/nushi/internal/proctree"
	"github.com/afrase/nushi/internal/testcheck"
)

// shell starts cmd.exe in a terminal and collects what it prints.
func shell(t *testing.T) (*Terminal, *output) {
	t.Helper()
	cmdExe := filepath.Join(os.Getenv("SystemRoot"), "System32", "cmd.exe")
	out := &output{}
	term, err := Start([]string{cmdExe}, t.TempDir(), os.Environ(), 80, 24, func() {})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(term.Close)
	go term.Attach(nil, func(b []byte) bool { out.add(b); return true })
	return term, out
}

func TestInputOutputAndSize(t *testing.T) {
	term, out := shell(t)
	testcheck.NoErr(t, term.Write([]byte("set /a 40+2\r")))
	out.wait(t, `42`)
	testcheck.NoErr(t, term.Resize(100, 30))
	testcheck.NoErr(t, term.Write([]byte("mode con\r")))
	out.wait(t, `Columns:\s+100`)
	if err := term.Resize(0, 30); err == nil {
		t.Error("a size of no columns was taken")
	}
}

func TestExitCodeAndReplay(t *testing.T) {
	term, out := shell(t)
	testcheck.NoErr(t, term.Write([]byte("echo before-exit& exit 3\r")))
	out.wait(t, `before-exit\r\n`)
	select {
	case <-term.Done():
	case <-time.After(5 * time.Second):
		t.Fatal("the shell didn't exit")
	}
	if exited, code := term.Exit(); !exited || code != 3 {
		t.Fatalf("exit %v %d, want true 3", exited, code)
	}
	again := &output{}
	term.Attach(nil, func(b []byte) bool { again.add(b); return true })
	again.wait(t, `before-exit\r\n`)
}

// The foreground program is the newest one under the shell, and
// Terminate is Ctrl+C to it.
func TestProgramAndTerminate(t *testing.T) {
	ignored := ignoresCtrlC()
	term, out := shell(t)
	if ignoresCtrlC() != ignored {
		t.Errorf("this process ignores Ctrl+C: %v, was %v", ignoresCtrlC(), ignored)
	}
	testcheck.NoErr(t, term.Write([]byte("ping -n 30 127.0.0.1\r")))
	if !testcheck.Eventually(func() bool { return term.Program() == "ping -n 30 127.0.0.1" }) {
		t.Fatalf("program %q, want ping", term.Program())
	}
	if got := term.Foreground(); got != "ping -n 30 127.0.0.1" {
		t.Errorf("foreground %q", got)
	}
	testcheck.NoErr(t, term.Terminate())
	out.wait(t, `Control-C`)
	if !testcheck.Eventually(func() bool { return term.Program() == "" }) {
		t.Errorf("program %q after Ctrl+C, want the shell's", term.Program())
	}
	if exited, _ := term.Exit(); exited {
		t.Error("the shell exited on Ctrl+C")
	}
}

// What the shell started in the background ends with Close too: it's in
// the terminal's job.
func TestCloseKillsChildren(t *testing.T) {
	term, out := shell(t)
	exe, err := os.Executable()
	testcheck.NoErr(t, err)
	// A child in the background, which outlives its shell unless the job
	// ends it.
	testcheck.NoErr(t, term.Write([]byte("set "+sleeperEnv+"=1& start /b \"\" \""+exe+"\"\r")))
	pid, _ := strconv.Atoi(out.wait(t, `sleeper=(\d+)\r\n`)[1])
	if ps, _ := (proctree.Tree{}).Under(term.Pid()); !slices.ContainsFunc(ps, func(p model.Process) bool { return p.Pid == pid }) {
		t.Fatalf("child %d isn't under the shell", pid)
	}
	term.Close()
	if !testcheck.Eventually(func() bool { _, err := (proctree.Tree{}).Get(pid); return err != nil }) {
		p, _ := os.FindProcess(pid)
		_ = p.Kill()
		t.Fatalf("child %d still runs after Close", pid)
	}
	if exited, _ := term.Exit(); !exited {
		t.Error("the shell runs after Close")
	}
}

// A batch file runs as the program, through cmd.exe, with its arguments.
func TestBatchFileProgram(t *testing.T) {
	dir := t.TempDir()
	bat := filepath.Join(dir, "hello.cmd")
	testcheck.Write(t, bat, "@echo hello-%1\r\n")
	out := &output{}
	term, err := Start([]string{bat, "world"}, dir, os.Environ(), 80, 24, func() {})
	testcheck.NoErr(t, err)
	t.Cleanup(term.Close)
	go term.Attach(nil, func(b []byte) bool { out.add(b); return true })
	out.wait(t, `hello-world`)
	select {
	case <-term.Done():
	case <-time.After(5 * time.Second):
		t.Fatal("the batch file didn't end")
	}
}

func TestEnvBlock(t *testing.T) {
	b, err := envBlock([]string{"A=1", "B=é"})
	testcheck.NoErr(t, err)
	if got := string(utf16Decode(b)); got != "A=1\x00B=é\x00\x00" {
		t.Errorf("block %q", got)
	}
	if b, _ := envBlock(nil); len(b) != 2 {
		t.Errorf("an empty block is %d units", len(b))
	}
	if _, err := envBlock([]string{"A=\x00"}); err == nil {
		t.Error("a NUL in a variable was taken")
	}
}

func utf16Decode(u []uint16) []rune {
	r := make([]rune, len(u))
	for i, c := range u {
		r[i] = rune(c)
	}
	return r
}
