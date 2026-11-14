//go:build unix

package shellenv

// nameKey is a variable's name as Unix compares it: as it is.
func nameKey(name string) string { return name }
