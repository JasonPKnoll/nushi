//go:build !windows

package main

// noCurrentDirPrograms does nothing: Unix never looks a bare name up in the
// current folder unless PATH names it, which shellenv's lookups pass over.
func noCurrentDirPrograms() {}
