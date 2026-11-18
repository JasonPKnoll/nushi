package terminal

import (
	"fmt"
	"os"
	"regexp"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/afrase/nushi/internal/testcheck"
)

// sleeperEnv makes this test binary, run again as a child, say its pid
// and sleep instead of running tests: a child that runs whatever its first argument says,
// even an empty one, which a multicall coreutils (uutils, Ubuntu's since
// 25.10, or busybox) would take for no program and exit at once.
const sleeperEnv = "NUSHI_TERMINAL_SLEEPER"

func TestMain(m *testing.M) {
	if os.Getenv(sleeperEnv) == "1" {
		_, _ = fmt.Fprintf(os.Stdout, "sleeper=%d\n", os.Getpid())
		time.Sleep(time.Minute)
		os.Exit(0)
	}
	m.Run()
}

// output collects what a terminal prints.
type output struct {
	mu  sync.Mutex
	buf strings.Builder
}

func (o *output) add(b []byte) {
	o.mu.Lock()
	o.buf.Write(b)
	o.mu.Unlock()
}

// wait returns the first match of re in the output, waiting for it.
func (o *output) wait(t *testing.T, re string) []string {
	t.Helper()
	r := regexp.MustCompile(re)
	var m []string
	if !testcheck.Eventually(func() bool {
		o.mu.Lock()
		defer o.mu.Unlock()
		m = r.FindStringSubmatch(o.buf.String())
		return m != nil
	}) {
		o.mu.Lock()
		defer o.mu.Unlock()
		t.Fatalf("no %q in %q", re, o.buf.String())
	}
	return m
}

// A terminal is named for its shell's program, without its folder (either
// slash) or a Windows .exe, whatever its case.
func TestShellName(t *testing.T) {
	for program, want := range map[string]string{
		"/bin/zsh":                               "zsh",
		"/usr/bin/fish":                          "fish",
		`C:\Program Files\PowerShell\7\pwsh.exe`: "pwsh",
		"cmd.EXE":                                "cmd",
		"bash":                                   "bash",
		"nu.exe.sh":                              "nu.exe.sh",
	} {
		if got := ShellName(program); got != want {
			t.Errorf("ShellName(%q) = %q, want %q", program, got, want)
		}
	}
}
