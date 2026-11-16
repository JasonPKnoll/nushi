package claudehistory

import (
	"encoding/json"
	"fmt"
	"maps"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/afrase/nushi/internal/testcheck"
)

// A record counts while its process runs and started when it says.
func TestOpen(t *testing.T) {
	dir := t.TempDir()
	me := os.Getpid()
	start, ok := started(me)
	if !ok {
		t.Fatal("no start time for this process")
	}
	dead := exec.Command("true")
	if err := dead.Run(); err != nil {
		t.Fatal(err)
	}
	sleeper := exec.Command("sleep", "5")
	if err := sleeper.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = sleeper.Process.Kill() })
	stamp := func(t time.Time) string { return t.UTC().Format("Mon Jan _2 15:04:05 2006") }
	write := func(pid int, r map[string]any) {
		r["pid"] = pid
		b, _ := json.Marshal(r)
		if err := os.WriteFile(filepath.Join(dir, fmt.Sprintf("%d.json", pid)), b, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write(me, map[string]any{"sessionId": "cli", "kind": "interactive", "entrypoint": "cli", "procStart": stamp(start), "pidDomain": domain()})
	write(dead.Process.Pid, map[string]any{"sessionId": "dead", "kind": "interactive", "updatedAt": time.Now().UnixMilli()})
	write(sleeper.Process.Pid, map[string]any{"sessionId": "reused", "kind": "interactive", "procStart": stamp(start.Add(-time.Hour))})
	write(1, map[string]any{"sessionId": "sdk", "kind": "interactive", "entrypoint": "sdk-cli", "updatedAt": time.Now().UnixMilli()})
	testcheck.NoErr(t, os.WriteFile(filepath.Join(dir, "1.abc.key"), []byte("{}"), 0o600))
	got := open(dir, domain(), started)
	want := map[string]string{"cli": "a terminal", "sdk": "another app"}
	if !maps.Equal(got, want) {
		t.Errorf("open = %v, want %v", got, want)
	}
	write(1, map[string]any{"sessionId": "stale", "updatedAt": time.Now().Add(-25 * time.Hour).UnixMilli()})
	if got := open(dir, domain(), started); got["stale"] != "" {
		t.Errorf("a day-old record without a start time counted: %v", got)
	}
	for _, d := range []string{"darwin:0123456789abcdef:pid:[4026531836]", "darwin:other-host", "darwin", "linux"} {
		if d == domain() {
			continue
		}
		write(1, map[string]any{"sessionId": "elsewhere", "entrypoint": "cli", "pidDomain": d, "updatedAt": time.Now().UnixMilli()})
		if got := open(dir, domain(), started); got["elsewhere"] != "" {
			t.Errorf("a record of another pid domain (%q) counted: %v", d, got)
		}
	}
	// A record a second off, as a step of the clock between Claude's ps
	// and this read leaves it on Linux, is still its process there; on the
	// Mac the start never moves, so it is another.
	if at, ok := started(sleeper.Process.Pid); ok {
		write(sleeper.Process.Pid, map[string]any{"sessionId": "stepped", "entrypoint": "cli", "procStart": stamp(at.Add(time.Second))})
		if got, want := open(dir, domain(), started)["stepped"] != "", stepSlack >= time.Second; got != want {
			t.Errorf("a record a second off counted: %v, want %v (slack %v)", got, want, stepSlack)
		}
	}
}

// A record as Claude Code writes it on Windows: no procStart, the
// process's creation time as a FILETIME in procStartFt, and the host's
// lowercased name in pidDomain (docs/facts/claude.md). It counts on that
// host while the start matches to the 100 ns.
func TestOpenWindowsRecord(t *testing.T) {
	dir := t.TempDir()
	me := os.Getpid()
	at := time.Date(2025, 11, 18, 19, 30, 12, 345678900, time.UTC) // FILETIME 134079678123456789
	write := func(ft, domain string, updated time.Time) {
		r := fmt.Sprintf(`{"pid":%d,"sessionId":"win","cwd":"C:\\Users\\jason\\src\\app","startedAt":%d,"procStartFt":%q,"kind":"interactive","entrypoint":"cli","status":"idle","updatedAt":%d,"pidDomain":%q}`,
			me, updated.UnixMilli(), ft, updated.UnixMilli(), domain)
		testcheck.NoErr(t, os.WriteFile(filepath.Join(dir, fmt.Sprintf("%d.json", me)), []byte(r), 0o600))
	}
	startedAt := func(start time.Time) func(int) (time.Time, bool) {
		return func(int) (time.Time, bool) { return start, true }
	}
	now := time.Now()
	for _, c := range []struct {
		name, ft, domain string
		start, updated   time.Time
		counts           bool
	}{
		{"its process", "134079678123456789", "darwin:jason-pc", at, now, true},
		{"another process on the pid", "134079678123456789", "darwin:jason-pc", at.Add(100 * time.Nanosecond), now, false},
		{"another host's", "134079678123456789", "darwin:other-pc", at, now, false},
		{"a token in another unit, fresh", "1763494212345678900", "darwin:jason-pc", at, now, true},
		{"a token in another unit, a day old", "1763494212345678900", "darwin:jason-pc", at, now.Add(-25 * time.Hour), false},
		{"an unreadable token, a day old", "soon", "darwin:jason-pc", at, now.Add(-25 * time.Hour), false},
	} {
		write(c.ft, c.domain, c.updated)
		got := open(dir, "darwin:jason-pc", startedAt(c.start))
		want := map[string]string{}
		if c.counts {
			want["win"] = "a terminal"
		}
		if !maps.Equal(got, want) {
			t.Errorf("%s: open = %v, want %v", c.name, got, want)
		}
	}
}
