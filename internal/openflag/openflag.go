// Package openflag holds the flags os.OpenFile takes that differ by OS:
// on macOS and Linux open(2)'s own (openflag_unix.go); on Windows
// CreateFile's that do the same where it has one, else nothing
// (openflag_windows.go, ADR 0123).
package openflag
