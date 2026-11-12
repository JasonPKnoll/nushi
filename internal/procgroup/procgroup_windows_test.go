package procgroup

import (
	"bytes"
	"context"
	"os/exec"
	"testing"
	"time"
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
