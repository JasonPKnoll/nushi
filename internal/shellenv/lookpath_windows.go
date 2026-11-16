package shellenv

import (
	"os"
	"path/filepath"
	"strings"
)

// defaultExts are the extensions Windows runs when PATHEXT isn't set.
const defaultExts = ".COM;.EXE;.BAT;.CMD"

// LookPath finds the program name in env's Path as Windows's own search
// does: in each folder, name with each of PATHEXT's extensions (`claude`
// is `claude.exe` or npm's `claude.cmd`), or name itself when it has one
// of them already; "" when it isn't there. A name with a folder in it is
// itself. Windows has no mode bit that makes a file a program: a regular
// file with such an extension is one.
func LookPath(name string, env []string) string {
	exts := filepath.SplitList(strings.ToLower(Get(env, "PATHEXT")))
	if len(exts) == 0 {
		exts = filepath.SplitList(strings.ToLower(defaultExts)) // lower, as exec.LookPath finds them
	}
	if strings.ContainsAny(name, `/\:`) {
		return withExt(name, exts)
	}
	for _, dir := range filepath.SplitList(Get(env, "PATH")) {
		if dir == "" || !filepath.IsAbs(dir) {
			continue // the current folder, or one relative to it, as exec.LookPath refuses
		}
		if p := withExt(filepath.Join(dir, name), exts); p != "" {
			return p
		}
	}
	return ""
}

// withExt is p when its extension is one of exts and it's a program,
// else the first p+ext that is; "" when none is.
func withExt(p string, exts []string) string {
	for _, ext := range exts {
		if strings.EqualFold(filepath.Ext(p), ext) && program(p) {
			return p
		}
	}
	for _, ext := range exts {
		if program(p + ext) {
			return p + ext
		}
	}
	return ""
}

func program(p string) bool {
	st, err := os.Stat(p)
	return err == nil && st.Mode().IsRegular()
}
