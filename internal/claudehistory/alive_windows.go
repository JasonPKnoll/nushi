package claudehistory

import (
	"errors"

	"golang.org/x/sys/windows"
)

// stillActive is GetExitCodeProcess's code for a process that hasn't
// ended.
const stillActive = 259

// alive says whether a process runs with pid: one that can be opened and
// hasn't an exit code yet, or one that exists but can't be opened
// (another user's, which counts, as on Unix).
func alive(pid int) bool {
	h, err := windows.OpenProcess(windows.PROCESS_QUERY_LIMITED_INFORMATION, false, uint32(pid)) //nolint:gosec // a pid from claude's record
	if errors.Is(err, windows.ERROR_ACCESS_DENIED) {
		return true
	}
	if err != nil {
		return false
	}
	defer windows.CloseHandle(h) //nolint:errcheck // a query's handle
	var code uint32
	return windows.GetExitCodeProcess(h, &code) == nil && code == stillActive
}
