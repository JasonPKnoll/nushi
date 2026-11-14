package shellenv

import "strings"

// nameKey is a variable's name as Windows compares it: whatever its case
// (Path and PATH are one variable).
func nameKey(name string) string { return strings.ToUpper(name) }
