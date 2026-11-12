package procgroup

import (
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
)

// bound kills cmd's process tree on cancel: Windows has no process group
// a child joins by itself, so taskkill's /T follows each process's parent
// from cmd down (ADR 0123). One whose parent ended before the cancel is
// no longer found from cmd, and lives on; a job object would hold it too
// (docs/roadmap.md).
func bound(cmd *exec.Cmd) {
	cmd.Cancel = func() error {
		kill := exec.Command(taskkill(), "/T", "/F", "/PID", strconv.Itoa(cmd.Process.Pid)) //nolint:gosec // the system's own taskkill, a pid
		if err := kill.Run(); err != nil {
			return cmd.Process.Kill() // taskkill missing or refused: at least cmd itself
		}
		return nil
	}
}

// taskkill is the system's taskkill, by its full path so PATH can't name
// another.
func taskkill() string {
	root := os.Getenv("SystemRoot")
	if root == "" {
		root = `C:\Windows`
	}
	return filepath.Join(root, "System32", "taskkill.exe")
}
