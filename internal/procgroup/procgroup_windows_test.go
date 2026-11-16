package procgroup

import (
	"bytes"
	"context"
	"os/exec"
	"strings"
	"testing"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"

	"github.com/afrase/nushi/internal/testcheck"
)

// A run whose context ends returns at once, though the program left a
// process in the background holding its stdout; that process is killed
// with it, so the run doesn't wait out WaitDelay.
func TestBound(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()
	cmd := exec.CommandContext(ctx, "cmd.exe", "/c", "start /b ping -n 100 127.0.0.1 & ping -n 100 127.0.0.1")
	var out bytes.Buffer
	cmd.Stdout = &out // a pipe the background ping holds open
	Bound(cmd)
	start := time.Now()
	if err := cmd.Run(); err == nil {
		t.Error("the run didn't fail")
	}
	if took := time.Since(start); took > 1400*time.Millisecond {
		t.Errorf("the run took %s after its context ended: the background process outlived it", took)
	}
}

// Kill ends what the command left running though the command has exited:
// a ping its cmd.exe started in the background, which taskkill's tree no
// longer finds from cmd.exe once it has gone.
func TestStartKillsWhatTheCommandLeft(t *testing.T) {
	cmd := exec.Command("cmd.exe", "/c", "start /b ping -n 100 127.0.0.1 >NUL")
	g, err := Start(cmd)
	testcheck.NoErr(t, err)
	ping := pings(t, cmd.Process.Pid, 1)[0]
	testcheck.NoErr(t, cmd.Wait()) // cmd.exe is gone; its ping isn't
	if gone(ping, 0) {
		t.Fatal("the ping ended by itself")
	}
	testcheck.NoErr(t, g.Kill())
	if !gone(ping, 5*time.Second) {
		t.Error("the ping outlived the kill")
	}
}

// pings are the n ping.exe processes parent started, once there are n, as
// handles that keep their pids theirs; closed when t ends.
func pings(t *testing.T, parent, n int) []windows.Handle {
	t.Helper()
	var pids []uint32
	found := testcheck.Eventually(func() bool {
		pids = children(t, parent, "ping.exe")
		return len(pids) == n
	})
	if !found {
		t.Fatalf("%d pings of %d, not %d", len(pids), parent, n)
	}
	hs := make([]windows.Handle, 0, n)
	for _, pid := range pids {
		h, err := windows.OpenProcess(windows.SYNCHRONIZE, false, pid)
		testcheck.NoErr(t, err)
		t.Cleanup(func() { _ = windows.CloseHandle(h) })
		hs = append(hs, h)
	}
	return hs
}

// children are the pids of the processes named exe whose parent is
// parent.
func children(t *testing.T, parent int, exe string) []uint32 {
	t.Helper()
	snap, err := windows.CreateToolhelp32Snapshot(windows.TH32CS_SNAPPROCESS, 0)
	testcheck.NoErr(t, err)
	defer windows.CloseHandle(snap) //nolint:errcheck // a snapshot
	var pids []uint32
	e := windows.ProcessEntry32{Size: uint32(unsafe.Sizeof(windows.ProcessEntry32{}))}
	for err := windows.Process32First(snap, &e); err == nil; err = windows.Process32Next(snap, &e) {
		if int(e.ParentProcessID) == parent && strings.EqualFold(windows.UTF16ToString(e.ExeFile[:]), exe) {
			pids = append(pids, e.ProcessID)
		}
	}
	return pids
}

// gone says h's process has exited, waiting up to wait for it.
func gone(h windows.Handle, wait time.Duration) bool {
	ev, err := windows.WaitForSingleObject(h, uint32(wait.Milliseconds()))
	return err == nil && ev == windows.WAIT_OBJECT_0
}
