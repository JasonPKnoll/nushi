// Package batchfile starts a Windows batch file (.cmd, .bat) without
// cmd.exe reading what it's given. Windows runs a batch file through
// cmd.exe, which reads the whole command line again by its own rules: Go
// quotes arguments by the C runtime's, so a `"`, `&` or `%` in one can
// end the command early, run another, or expand a variable (the BatBadBut
// kind of bug). And cmd.exe looks for a bare program name, npm's shims'
// `node`, in the current folder before Path, so a project's own node.exe
// or node.bat would run in a chat started there.
//
// So an npm shim (`claude.cmd`, `pi.cmd`) runs as what it would run, node
// and its script, with no cmd.exe; any other batch file runs only with
// arguments cmd.exe reads as plain text; and every program Nushi starts
// gets NoCurrentDir, which keeps the current folder out of cmd.exe's
// search and CreateProcess's. Guard does it for an exec.Cmd on Windows.
package batchfile

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"
)

// NoCurrentDir is the variable whose presence has Windows (CreateProcess,
// cmd.exe) look for a bare program name on Path alone, never in the
// current folder first.
const NoCurrentDir = "NoDefaultCurrentDirectoryInExePath"

// Environ is env with NoCurrentDir set: a program started with it, and
// what that starts, never runs a program from its current folder by a
// bare name.
func Environ(env []string) []string {
	if get(env, NoCurrentDir) != "" {
		return env
	}
	return append(slices.Clip(env), NoCurrentDir+"=1")
}

// Is says path is a batch file, which Windows runs through cmd.exe.
func Is(path string) bool {
	ext := strings.ToLower(filepath.Ext(path))
	return ext == ".cmd" || ext == ".bat"
}

// Command is how to start path with args (those after the program's
// name) in env when path is a batch file: npm's shim of a Node program
// is node.exe (beside the shim, else on env's Path) and its script,
// before args, in env with the NODE_PATH the shim sets (pnpm's); another
// batch file is itself, with args, only when it and every argument are
// plain (cmd.exe changes nothing in them). Anything else is an error that
// says why. A path that isn't a batch file is itself, in env.
func Command(path string, args, env []string) (prog string, progArgs, progEnv []string, err error) {
	if !Is(path) {
		return path, args, env, nil
	}
	sh, err := npmScript(path)
	if err != nil {
		return "", nil, nil, err
	}
	name := filepath.Base(path)
	if sh.script != "" {
		node := findNode(filepath.Dir(path), env)
		if node == "" {
			return "", nil, nil, fmt.Errorf("%s runs node, and there's no node.exe beside it or on Path", name)
		}
		return node, append([]string{sh.script}, args...), withNodePath(env, sh.nodePath), nil
	}
	if !plain(path) || slices.ContainsFunc(args, func(a string) bool { return !plain(a) }) {
		return "", nil, nil, fmt.Errorf("%s is a batch file, and cmd.exe would read an argument as commands of its own "+
			"(it has one of %s or a line break): install its .exe instead", name, special)
	}
	return path, args, env, nil
}

// withNodePath is env with folders, a shim's NODE_PATH, before what env's
// NODE_PATH has, as pnpm's shim sets it; env itself when folders is "".
func withNodePath(env []string, folders string) []string {
	if folders == "" {
		return env
	}
	if old := get(env, nodePath); old != "" {
		folders += ";" + old
	}
	env = slices.DeleteFunc(slices.Clone(env), func(kv string) bool {
		n, _, _ := strings.Cut(kv, "=")
		return strings.EqualFold(n, nodePath)
	})
	return append(env, nodePath+"="+folders)
}

// nodePath is the variable through which pnpm's shims tell node where
// the package's own dependencies are.
const nodePath = "NODE_PATH"

// special are the characters cmd.exe reads as its own on a command line,
// in quotes or out: they expand a variable (`%`, and `!` with delayed
// expansion), escape (`^`), end the quotes Go writes (`"`), or start
// another command or a redirection.
const special = `"%!^&|<>()`

// plain says cmd.exe passes s to the batch file as it is.
func plain(s string) bool {
	return !strings.ContainsAny(s, special) && !strings.ContainsFunc(s, func(r rune) bool { return r < ' ' || r == 0x7f })
}

// maxShim is the largest batch file read for a shim: npm's are a few
// hundred bytes.
const maxShim = 64 << 10

// shimmed is what an npm shim of a Node program runs: script, the
// absolute path of the script it runs with node, "" when the batch file
// isn't one, and nodePath, the NODE_PATH it sets when none is set (pnpm's
// `@SET "NODE_PATH=<folders>"`), "" when it sets none.
type shimmed struct{ script, nodePath string }

// npmScript reads the batch file at path, and is what it runs when it's a
// shim npm (or pnpm) wrote for a Node program: every line that passes on
// its arguments (`%*`) runs node on one script in or below the shim's
// folder (`"%dp0%\node_modules\…\cli.js" %*`). Its script is "" for any
// other batch file, one npm wrote for a shell script included.
func npmScript(path string) (shimmed, error) {
	f, err := os.Open(path) //nolint:gosec // the program the caller is about to run
	if err != nil {
		return shimmed{}, err
	}
	defer func() { _ = f.Close() }()
	text, err := io.ReadAll(io.LimitReader(f, maxShim+1))
	if err != nil || len(text) > maxShim {
		return shimmed{}, err
	}
	lines := bytes.FieldsFunc(text, func(r rune) bool { return r == '\n' || r == '\r' })
	var sh shimmed
	for _, line := range lines {
		words := fields(string(line))
		if v, ok := setsNodePath(words); ok && sh.nodePath == "" {
			sh.nodePath = v
		}
		if !slices.Contains(words, "%*") {
			continue
		}
		rel, ok := nodeLine(words, bytes.Contains(bytes.ToLower(text), []byte(`set "_prog=node"`)))
		if !ok || (sh.script != "" && rel != sh.script) {
			return shimmed{}, nil
		}
		sh.script = rel
	}
	if sh.script == "" {
		return shimmed{}, nil
	}
	sh.script = filepath.Join(filepath.Dir(path), filepath.FromSlash(strings.ReplaceAll(sh.script, `\`, "/")))
	return sh, nil
}

// setsNodePath is the folders of a shim's line `@SET "NODE_PATH=<folders>"`
// (pnpm writes them whole), when it's that line. One that names a
// variable (the ELSE branch's `%NODE_PATH%`) isn't: withNodePath does
// what it does.
func setsNodePath(words []string) (string, bool) {
	if len(words) != 2 || !strings.EqualFold(strings.TrimPrefix(words[0], "@"), "set") {
		return "", false
	}
	name, folders, ok := strings.Cut(words[1], "=")
	if !ok || !strings.EqualFold(name, nodePath) || folders == "" || strings.Contains(folders, "%") {
		return "", false
	}
	return folders, true
}

// nodeLine is the script, relative to the shim's folder, of a shim's line
// that ends `<node> "<folder>\<script>" %*`; progIsNode says the shim set
// npm's _prog to node, for a line that runs "%_prog%".
func nodeLine(words []string, progIsNode bool) (string, bool) {
	n := len(words)
	if n < 3 || words[n-1] != "%*" {
		return "", false
	}
	switch strings.ToLower(words[n-3]) {
	case "node", "node.exe", `%dp0%\node.exe`, `%~dp0\node.exe`:
	case "%_prog%":
		if !progIsNode {
			return "", false
		}
	default:
		return "", false
	}
	for _, dir := range []string{`%dp0%\`, `%~dp0\`} {
		if rel, ok := strings.CutPrefix(words[n-2], dir); ok && rel != "" && !strings.ContainsAny(rel, `%!"`) {
			return rel, true
		}
	}
	return "", false
}

// fields are a line's words as cmd.exe splits them, quotes kept together
// and taken off.
func fields(line string) []string {
	var words []string
	var word strings.Builder
	quoted, in := false, false
	for _, r := range line {
		switch {
		case r == '"':
			quoted, in = !quoted, true
		case !quoted && (r == ' ' || r == '\t'):
			if in {
				words = append(words, word.String())
				word.Reset()
			}
			in = false
		default:
			word.WriteRune(r)
			in = true
		}
	}
	if in {
		words = append(words, word.String())
	}
	return words
}

// findNode is node.exe in dir (the shim's own folder), else in the first
// folder of env's Path that has one; "" when none does. A Path entry that
// isn't absolute is passed over: it would name a folder below the current
// one, which a project chooses.
func findNode(dir string, env []string) string {
	for _, d := range slices.Concat([]string{dir}, filepath.SplitList(get(env, "PATH"))) {
		if !filepath.IsAbs(d) {
			continue
		}
		p := filepath.Join(d, "node.exe")
		if st, err := os.Stat(p); err == nil && st.Mode().IsRegular() {
			return p
		}
	}
	return ""
}

// get is the value of the variable named name in env, whatever its case
// (as Windows takes it), the last when it's there twice; "" when it isn't.
func get(env []string, name string) string {
	for _, kv := range slices.Backward(env) {
		if n, v, ok := strings.Cut(kv, "="); ok && strings.EqualFold(n, name) {
			return v
		}
	}
	return ""
}
