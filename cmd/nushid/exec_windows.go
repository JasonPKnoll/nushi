package main

import (
	"errors"
	"fmt"
)

// replace refuses: Windows can't replace a process with another, and has
// no login shell to start the daemon through (ADR 0124).
func replace([]string, []string) error {
	return fmt.Errorf("%s on windows: %w", loginFlag, errors.ErrUnsupported)
}
