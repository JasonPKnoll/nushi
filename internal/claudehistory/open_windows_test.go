package claudehistory

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"golang.org/x/sys/windows"

	"github.com/afrase/nushi/internal/testcheck"
)

// A record with this process's creation time, read as Claude Code reads
// it (GetProcessTimes' FILETIME, in decimal), and this host's domain
// counts; the domain is Claude's shape.
func TestOpenWindows(t *testing.T) {
	d := domain()
	if !strings.HasPrefix(d, "darwin:") || len(d) == len("darwin:") || d != strings.ToLower(d) {
		t.Fatalf("domain %q", d)
	}
	var created, exited, kernel, user windows.Filetime
	testcheck.NoErr(t, windows.GetProcessTimes(windows.CurrentProcess(), &created, &exited, &kernel, &user))
	ft := uint64(created.HighDateTime)<<32 | uint64(created.LowDateTime)
	dir := t.TempDir()
	me := os.Getpid()
	b, _ := json.Marshal(map[string]any{"pid": me, "sessionId": "win", "kind": "interactive", "entrypoint": "cli", "procStartFt": strconv.FormatUint(ft, 10), "pidDomain": d})
	testcheck.NoErr(t, os.WriteFile(filepath.Join(dir, fmt.Sprintf("%d.json", me)), b, 0o600))
	if got := open(dir, domain(), started); got["win"] != "a terminal" {
		t.Fatalf("open = %v, want this process's record (FILETIME %d)", got, ft)
	}
}
