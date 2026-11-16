package claudehistory

import (
	"os"
	"strings"
	"time"

	"golang.org/x/sys/windows"
)

// started is when process pid started: its creation time from
// GetProcessTimes, which Claude Code records as procStartFt.
func started(pid int) (time.Time, bool) {
	h, err := windows.OpenProcess(windows.PROCESS_QUERY_LIMITED_INFORMATION, false, uint32(pid)) //nolint:gosec // a pid from claude's record
	if err != nil {
		return time.Time{}, false
	}
	defer windows.CloseHandle(h) //nolint:errcheck // a query's handle
	var created, exited, kernel, user windows.Filetime
	if windows.GetProcessTimes(h, &created, &exited, &kernel, &user) != nil {
		return time.Time{}, false
	}
	return time.Unix(0, created.Nanoseconds()).UTC(), true
}

// domain is the pidDomain Claude Code writes on Windows: "darwin:" and
// the host's name, lowercased (docs/facts/claude.md).
func domain() string {
	host, _ := os.Hostname()
	return "darwin:" + strings.ToLower(host)
}

// stepSlack is how far a record's procStartFt may be from the process's
// start and still be it: none, since a process's creation time is fixed
// and both come from it to the 100 ns.
const stepSlack time.Duration = 0
