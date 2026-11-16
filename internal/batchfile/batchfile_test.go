package batchfile

import (
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/afrase/nushi/internal/testcheck"
)

// npmShim is a shim as npm's cmd-shim writes it today, for claude.
const npmShim = `@ECHO off
GOTO start
:find_dp0
SET dp0=%~dp0
EXIT /b
:start
SETLOCAL
CALL :find_dp0

IF EXIST "%dp0%\node.exe" (
  SET "_prog=%dp0%\node.exe"
) ELSE (
  SET "_prog=node"
  SET PATHEXT=%PATHEXT:;.JS;=;%
)

endLocal & goto #_undefined_# 2>NUL || title %COMSPEC% & "%_prog%"  "%dp0%\node_modules\@anthropic-ai\claude-code\cli.js" %*
`

// oldShim is the form npm and pnpm wrote before, for pi.
const oldShim = `@IF EXIST "%~dp0\node.exe" (
  "%~dp0\node.exe"  "%~dp0\node_modules\pi\dist\cli.js" %*
) ELSE (
  @SETLOCAL
  @SET PATHEXT=%PATHEXT:;.JS;=;%
  node  "%~dp0\node_modules\pi\dist\cli.js" %*
)
`

// pnpmShim is a shim as pnpm's cmd-shim writes it, with the NODE_PATH it
// sets before node runs; nodePathFolders are its folders.
const pnpmShim = `@SETLOCAL
@IF NOT DEFINED NODE_PATH (
  @SET "NODE_PATH=` + nodePathFolders + `"
) ELSE (
  @SET "NODE_PATH=` + nodePathFolders + `;%NODE_PATH%"
)
@IF EXIST "%~dp0\node.exe" (
  "%~dp0\node.exe"  "%~dp0\..\.pnpm\tool@1.0.0\node_modules\tool\cli.js" %*
) ELSE (
  @SET PATHEXT=%PATHEXT:;.JS;=;%
  node  "%~dp0\..\.pnpm\tool@1.0.0\node_modules\tool\cli.js" %*
)
`

const nodePathFolders = `C:\Users\Jo Smith\pnpm\.pnpm\tool@1.0.0\node_modules;C:\Users\Jo Smith\pnpm\.pnpm\node_modules`

// shellShim is npm's shim of a program with a #!/bin/sh line: not Node's.
const shellShim = `@ECHO off
SETLOCAL
IF EXIST "%dp0%\sh.exe" (
  SET "_prog=%dp0%\sh.exe"
) ELSE (
  SET "_prog=sh"
)
endLocal & goto #_undefined_# 2>NUL || title %COMSPEC% & "%_prog%"  "%dp0%\node_modules\tool\bin\tool" %*
`

// dataArgs are arguments cmd.exe would read as its own: a variable, an
// escape, a quote that ends Go's quoting, a second command.
var dataArgs = []string{"--mcp-config", `{"a":"%TEMP%"}`, `x" & calc & "`, "a^b", `C:\Users\R&D\cfg.json`}

// shim writes a batch file named name with text in a new folder, its
// script beside it under node_modules, and is the batch file's path.
func shim(t *testing.T, name, text, script string) string {
	t.Helper()
	dir := t.TempDir()
	if script != "" {
		testcheck.Write(t, filepath.Join(dir, filepath.FromSlash(script)), "")
	}
	testcheck.Write(t, filepath.Join(dir, name), text)
	return filepath.Join(dir, name)
}

func pathEnv(dirs ...string) []string {
	return []string{"PATH=" + strings.Join(dirs, string(filepath.ListSeparator))}
}

func TestNpmShimRunsAsNodeAndItsScript(t *testing.T) {
	for _, tc := range []struct{ name, text, script string }{
		{"claude.cmd", npmShim, "node_modules/@anthropic-ai/claude-code/cli.js"},
		{"pi.CMD", oldShim, "node_modules/pi/dist/cli.js"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			path := shim(t, tc.name, tc.text, tc.script)
			nodeDir := t.TempDir()
			node := filepath.Join(nodeDir, "node.exe")
			testcheck.Write(t, node, "")

			got, args, _, err := Command(path, dataArgs, pathEnv(nodeDir))
			testcheck.NoErr(t, err)
			want := append([]string{filepath.Join(filepath.Dir(path), filepath.FromSlash(tc.script))}, dataArgs...)
			if got != node || !slices.Equal(args, want) {
				t.Fatalf("Command = %q %q, want %q %q", got, args, node, want)
			}
		})
	}
}

// pnpm's shim runs as node and its script too, with the NODE_PATH the
// shim would set: its folders alone, or before the one already set.
func TestPnpmShimKeepsItsNodePath(t *testing.T) {
	path := shim(t, "tool.cmd", pnpmShim, "")
	nodeDir := t.TempDir()
	testcheck.Write(t, filepath.Join(nodeDir, "node.exe"), "")
	wantScript := filepath.Join(filepath.Dir(path), "..", ".pnpm", "tool@1.0.0", "node_modules", "tool", "cli.js")
	for _, tc := range []struct {
		env  []string
		want string
	}{
		{nil, nodePathFolders},
		{[]string{`node_path=D:\mine`}, nodePathFolders + `;D:\mine`},
	} {
		got, args, env, err := Command(path, []string{"-p"}, append(pathEnv(nodeDir), tc.env...))
		testcheck.NoErr(t, err)
		if want := []string{wantScript, "-p"}; got != filepath.Join(nodeDir, "node.exe") || !slices.Equal(args, want) {
			t.Fatalf("Command = %q %q, want node and %q", got, args, want)
		}
		if v := get(env, "NODE_PATH"); v != tc.want || len(env) != 2 {
			t.Errorf("env = %q, want PATH and NODE_PATH=%s", env, tc.want)
		}
	}
}

// npm's shim sets no NODE_PATH, and the environment is passed on as it is.
func TestNpmShimLeavesTheEnvironment(t *testing.T) {
	path := shim(t, "claude.cmd", npmShim, "node_modules/@anthropic-ai/claude-code/cli.js")
	nodeDir := t.TempDir()
	testcheck.Write(t, filepath.Join(nodeDir, "node.exe"), "")
	env := pathEnv(nodeDir)
	_, _, got, err := Command(path, nil, env)
	testcheck.NoErr(t, err)
	if !slices.Equal(got, env) {
		t.Fatalf("env = %q, want %q", got, env)
	}
}

func TestNodeBesideTheShimComesFirst(t *testing.T) {
	path := shim(t, "claude.cmd", npmShim, "node_modules/@anthropic-ai/claude-code/cli.js")
	beside := filepath.Join(filepath.Dir(path), "node.exe")
	testcheck.Write(t, beside, "")
	onPath := t.TempDir()
	testcheck.Write(t, filepath.Join(onPath, "node.exe"), "")

	got, _, _, err := Command(path, nil, pathEnv(onPath))
	testcheck.NoErr(t, err)
	if got != beside {
		t.Fatalf("node = %q, want the one beside the shim, %q", got, beside)
	}
}

// A project's own node.exe or node.bat in the folder a chat starts in is
// never what a shim runs, even with the folder on Path by a relative name.
func TestNodeInTheCurrentFolderNeverRuns(t *testing.T) {
	path := shim(t, "claude.cmd", npmShim, "node_modules/@anthropic-ai/claude-code/cli.js")
	project := t.TempDir()
	testcheck.Write(t, filepath.Join(project, "node.exe"), "")
	testcheck.Write(t, filepath.Join(project, "node.bat"), "")
	testcheck.Write(t, filepath.Join(project, "bin", "node.exe"), "")
	t.Chdir(project)

	got, args, _, err := Command(path, []string{"-p"}, pathEnv("", ".", "bin"))
	if err == nil {
		t.Fatalf("Command = %q %q, want an error: no node.exe but the project's", got, args)
	}

	nodeDir := t.TempDir()
	testcheck.Write(t, filepath.Join(nodeDir, "node.exe"), "")
	got, _, _, err = Command(path, nil, pathEnv(".", "bin", nodeDir))
	testcheck.NoErr(t, err)
	if want := filepath.Join(nodeDir, "node.exe"); got != want {
		t.Fatalf("node = %q, want Path's absolute one, %q", got, want)
	}
}

func TestOtherBatchFileRunsOnlyWithPlainArguments(t *testing.T) {
	for _, text := range []string{"@echo %*\n", shellShim} {
		path := shim(t, "tool.bat", text, "node_modules/tool/bin/tool")
		plainArgs := []string{"mcp", "list", `C:\Users\Jo Smith\x.json`, "--name=a,b;c", "é"}
		got, args, _, err := Command(path, plainArgs, nil)
		testcheck.NoErr(t, err)
		if got != path || !slices.Equal(args, plainArgs) {
			t.Fatalf("Command = %q %q, want it as it was", got, args)
		}
		for _, arg := range []string{"a&b", "50%", `say "hi"`, "a^b", "!x!", "a|b", "<in", ">out", "(x)", "a\nb", "a\rb"} {
			if got, args, _, err := Command(path, []string{"mcp", arg}, nil); err == nil {
				t.Errorf("Command with %q = %q %q, want an error", arg, got, args)
			}
		}
	}
}

func TestBatchFileInAFolderCmdReads(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "R&D")
	testcheck.Write(t, filepath.Join(dir, "tool.cmd"), "@echo %*\n")
	if _, _, _, err := Command(filepath.Join(dir, "tool.cmd"), nil, nil); err == nil {
		t.Fatal("a batch file whose path has & ran")
	}
}

func TestShimsThatArentNodesAreOtherBatchFiles(t *testing.T) {
	for name, text := range map[string]string{
		"two scripts":     "node \"%~dp0\\a.js\" %*\nnode \"%~dp0\\b.js\" %*\n",
		"node flags":      "node --inspect \"%~dp0\\a.js\" %*\n",
		"a variable":      "node \"%~dp0\\%X%.js\" %*\n",
		"outside dp0":     "node \"C:\\a.js\" %*\n",
		"no prog set":     "\"%_prog%\" \"%dp0%\\a.js\" %*\n",
		"no args passed":  "node \"%~dp0\\a.js\"\n",
		"another program": "python \"%~dp0\\a.py\" %*\n",
	} {
		path := shim(t, "x.cmd", text, "")
		if sh, err := npmScript(path); err != nil || sh.script != "" {
			t.Errorf("%s: npmScript says a node shim (%v)", name, err)
		}
	}
}

func TestNotABatchFileIsItself(t *testing.T) {
	got, args, _, err := Command(`C:\bin\claude.exe`, dataArgs, nil)
	testcheck.NoErr(t, err)
	if got != `C:\bin\claude.exe` || !slices.Equal(args, dataArgs) {
		t.Fatalf("Command = %q %q", got, args)
	}
}

func TestEnviron(t *testing.T) {
	env := Environ([]string{"PATH=x"})
	if want := []string{"PATH=x", NoCurrentDir + "=1"}; !slices.Equal(env, want) {
		t.Fatalf("Environ = %q, want %q", env, want)
	}
	set := []string{"nodefaultcurrentdirectoryinexepath=yes"}
	if env := Environ(set); !slices.Equal(env, set) {
		t.Fatalf("Environ = %q, want it kept as set", env)
	}
}

func TestFields(t *testing.T) {
	got := fields(`a  "b c" "%dp0%\x y.js" %*`)
	if want := []string{"a", "b c", `%dp0%\x y.js`, "%*"}; !slices.Equal(got, want) {
		t.Fatalf("fields = %q, want %q", got, want)
	}
}

func TestLargeFileIsNoShim(t *testing.T) {
	path := shim(t, "big.cmd", strings.Repeat("rem\n", maxShim/3)+"node \"%~dp0\\a.js\" %*\n", "a.js")
	if sh, _ := npmScript(path); sh.script != "" {
		t.Fatal("a file past maxShim read as a shim")
	}
}
