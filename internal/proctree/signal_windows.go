package proctree

import (
	"errors"
	"fmt"
)

// signalGroup sends nothing: Windows has neither process groups nor
// signals, and no process is read there to signal (proctree_other.go,
// ADR 0124).
func signalGroup(int, bool) error {
	return fmt.Errorf("signalling a process group on windows: %w", errors.ErrUnsupported)
}

// signalOne sends nothing, as signalGroup.
func signalOne(int, bool) error {
	return fmt.Errorf("signalling a process on windows: %w", errors.ErrUnsupported)
}
