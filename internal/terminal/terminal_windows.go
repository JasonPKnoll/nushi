package terminal

import (
	"errors"
	"fmt"
)

// On Windows (ADR 0123) no terminal starts yet: its pseudo-terminal is
// ConPTY's, not a device a program opens (docs/roadmap.md). Start refuses,
// so none of the rest runs.

// Start refuses: no terminal on Windows yet.
func Start([]string, string, []string, int, int, func()) (*Terminal, error) {
	return nil, fmt.Errorf("terminals on windows: %w", errors.ErrUnsupported)
}

// foreground is never read: no terminal runs.
func (*Terminal) foreground() int { return 0 }

// Terminate does nothing: no terminal runs.
func (*Terminal) Terminate() error { return nil }

// Resize refuses: no terminal runs.
func (*Terminal) Resize(int, int) error { return errors.ErrUnsupported }

// hangUp waits for the program, which never started.
func (t *Terminal) hangUp() { <-t.done }

// name is "": no process is read on Windows.
func name(int) string { return "" }
