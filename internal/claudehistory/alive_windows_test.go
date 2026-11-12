package claudehistory

import (
	"os"
	"os/exec"
	"testing"
)

// This process is alive; one that has exited isn't.
func TestAlive(t *testing.T) {
	if !alive(os.Getpid()) {
		t.Error("this process isn't alive")
	}
	cmd := exec.Command("cmd.exe", "/c", "exit 0")
	if err := cmd.Run(); err != nil {
		t.Fatal(err)
	}
	if alive(cmd.Process.Pid) {
		t.Error("an exited process is alive")
	}
}
