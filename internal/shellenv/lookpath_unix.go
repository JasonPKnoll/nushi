//go:build unix

package shellenv

import (
	"os"
	"path/filepath"
	"strings"
)

// LookPath finds the program name in env's PATH, as exec.LookPath does in
// the daemon's; "" when it isn't there. A name with a slash is itself.
func LookPath(name string, env []string) string {
	if strings.Contains(name, "/") {
		if program(name) {
			return name
		}
		return ""
	}
	for _, dir := range filepath.SplitList(Get(env, "PATH")) {
		if dir == "" {
			continue // the current folder, which exec.LookPath refuses too
		}
		if p := filepath.Join(dir, name); program(p) {
			return p
		}
	}
	return ""
}

func program(p string) bool {
	st, err := os.Stat(p)
	return err == nil && st.Mode().IsRegular() && st.Mode()&0o111 != 0
}
