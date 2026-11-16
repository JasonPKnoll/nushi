//go:build unix

package e2e

import (
	"context"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"

	"connectrpc.com/connect"

	"github.com/afrase/nushi/internal/acptest"
	nushiv1 "github.com/afrase/nushi/internal/gen/nushi/v1"
)

// A daemon that stops ends its terminals' shells, as main's does (ADR
// 0022): they don't outlive it.
func TestStopEndsTheTerminals(t *testing.T) {
	ctx := context.Background()
	d := startDaemon(t, t.TempDir(), &acptest.History{})
	a := open(t, d)
	id := a.start(&nushiv1.StartChatRequest{Folder: t.TempDir()})
	term := must(a.c.OpenTerminal(ctx, connect.NewRequest(&nushiv1.OpenTerminalRequest{ChatId: id, Cols: 80, Rows: 24}))).Terminal
	pidFile := filepath.Join(t.TempDir(), "pid")
	must(a.c.WriteTerminal(ctx, connect.NewRequest(&nushiv1.WriteTerminalRequest{TerminalId: term.Id, Data: []byte("echo $$ > " + pidFile + "\n")})))
	var pid int
	until(t, "the shell's pid", func() bool {
		b, err := os.ReadFile(pidFile)
		pid, _ = strconv.Atoi(strings.TrimSpace(string(b)))
		return err == nil && pid > 0
	})
	d.stop()
	until(t, "the shell to end", func() bool { return syscall.Kill(pid, 0) != nil })
}
