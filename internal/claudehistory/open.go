package claudehistory

import (
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/afrase/nushi/internal/claude"
)

// OpenElsewhere returns the sessions a running Claude Code holds open (any, not
// only ids), by session id: "a terminal" for the CLI run by hand, "another
// app" for one a client such as another ACP client started. Each running
// CLI writes <config>/sessions/<pid>.json (its session id, entrypoint, the
// process's start time). A record counts by the rules claude-agent-acp's
// session list had: the process is alive, and started when the record
// says (within stepSlack), so a pid used again doesn't count; a record
// without a start time
// counts for a day. A record whose pidDomain isn't this host's (a pid of
// another machine or pid namespace, docs/facts/claude.md) doesn't count.
func (Local) OpenElsewhere(_ []string) map[string]string {
	if _, err := os.UserHomeDir(); err != nil {
		return nil
	}
	return open(filepath.Join(claude.ConfigDir(), "sessions"), domain(), started)
}

var record = regexp.MustCompile(`^(\d+)\.json$`)

// open reads the records in dir, here being this host's pidDomain.
func open(dir, here string, startedAt func(pid int) (time.Time, bool)) map[string]string {
	names, _ := os.ReadDir(dir)
	out := map[string]string{}
	for _, n := range names {
		m := record.FindStringSubmatch(n.Name())
		if m == nil || !n.Type().IsRegular() {
			continue
		}
		pid, _ := strconv.Atoi(m[1])
		var r struct {
			Pid         int    `json:"pid"`
			SessionID   string `json:"sessionId"`
			ProcStart   string `json:"procStart"`
			ProcStartFt string `json:"procStartFt"`
			Entrypoint  string `json:"entrypoint"`
			Domain      string `json:"pidDomain"`
			Updated     int64  `json:"updatedAt"`
			Started     int64  `json:"startedAt"`
			StatusTime  int64  `json:"statusUpdatedAt"`
		}
		b, err := os.ReadFile(filepath.Join(dir, n.Name()))
		if err != nil || json.Unmarshal(b, &r) != nil || r.Pid != pid || r.SessionID == "" || r.Domain != "" && r.Domain != here || !alive(pid) {
			continue
		}
		start, known := startedAt(pid)
		recorded, unit, ok := recordedStart(r.ProcStart, r.ProcStartFt)
		switch {
		case ok && known && recorded.Sub(start.Truncate(unit)).Abs() > stepSlack:
			continue // another process has the pid now
		case !ok && time.Since(time.UnixMilli(max(r.Updated, r.Started, r.StatusTime))) > 24*time.Hour:
			continue
		}
		out[r.SessionID] = "another app"
		if r.Entrypoint == "cli" { // an ACP client's CLI says "sdk-cli" (its kind is "interactive" too)
			out[r.SessionID] = "a terminal"
		}
	}
	return out
}

// fileTimeEpoch is 1601-01-01 UTC, where a Windows FILETIME counts from,
// in 100 ns ticks before the Unix epoch.
const fileTimeEpoch = 116444736000000000

// fileTimeLimit is where Claude stops reading a procStartFt as a FILETIME:
// it takes a token past it for another unit and calls such a pair the
// same process, unable to tell (docs/facts/claude.md). Today's FILETIME
// is about 1.34e17.
const fileTimeLimit = 300000000000000000

// recordedStart is when a record says its process started, to the unit
// the record gives it in: procStart, ps's lstart to the second (macOS and
// Linux), or procStartFt, the process's creation time as a FILETIME in
// decimal (Windows). A record with neither, or one it can't read, has
// none.
func recordedStart(procStart, procStartFt string) (time.Time, time.Duration, bool) {
	if procStartFt != "" {
		ticks, err := strconv.ParseInt(procStartFt, 10, 64)
		if err != nil || ticks < fileTimeEpoch || ticks >= fileTimeLimit {
			return time.Time{}, 0, false
		}
		ticks -= fileTimeEpoch
		return time.Unix(ticks/1e7, ticks%1e7*100).UTC(), 100 * time.Nanosecond, true
	}
	t, err := time.ParseInLocation("Mon Jan 2 15:04:05 2006", strings.Join(strings.Fields(procStart), " "), time.UTC)
	return t, time.Second, err == nil
}
