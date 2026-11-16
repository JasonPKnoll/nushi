package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"golang.org/x/sys/windows"

	"github.com/afrase/nushi/internal/regularfile"
)

// maxWTSettings bounds what is read of a Windows Terminal settings file
// or fragment: its own are a few kilobytes.
const maxWTSettings = 4 << 20

// terminalShell is the program a terminal starts and its arguments:
// Windows Terminal's default profile's (wtprofile.go, ADR 0130), else
// PowerShell 7 where it's installed, else Windows PowerShell, which every
// Windows has. It is read again for each terminal, so a change there
// counts from the next.
func terminalShell() []string {
	if line := wtDefaultLine(wtSettings(), wtFragments()); line != "" {
		if argv, err := windows.DecomposeCommandLine(expandEnv(line)); err == nil && len(argv) > 0 && argv[0] != "" {
			return argv
		}
	}
	if pwsh, err := exec.LookPath("pwsh.exe"); err == nil {
		return []string{pwsh}
	}
	return []string{expandEnv(windowsPowerShellLine)}
}

// terminalBase is the environment a terminal's starts from: the agents'
// (the registry's, ADR 0126). A shell on Windows doesn't build one of its
// own at login, as a Unix login shell does, and the daemon's is the app's,
// as it was when the app started.
func terminalBase(agents interface{ Environ() []string }) []string {
	return agents.Environ()
}

// wtSettings is Windows Terminal's settings file's text, or nil: the
// installed app's, else its preview's, else an unpackaged one's.
func wtSettings() []byte {
	local := os.Getenv("LOCALAPPDATA")
	if local == "" {
		return nil
	}
	for _, path := range []string{
		filepath.Join(local, "Packages", "Microsoft.WindowsTerminal_8wekyb3d8bbwe", "LocalState", "settings.json"),
		filepath.Join(local, "Packages", "Microsoft.WindowsTerminalPreview_8wekyb3d8bbwe", "LocalState", "settings.json"),
		filepath.Join(local, "Microsoft", "Windows Terminal", "settings.json"),
	} {
		if b, err := regularfile.Read(path, maxWTSettings); err == nil {
			return b
		}
	}
	return nil
}

// wtFragments is the fragment files other apps wrote for Windows
// Terminal, by their source (the folder each is in: Git, Microsoft.WSL),
// for everyone and for this user.
func wtFragments() map[string][][]byte {
	out := map[string][][]byte{}
	for _, root := range []string{os.Getenv("ProgramData"), os.Getenv("LOCALAPPDATA")} {
		if root == "" {
			continue
		}
		dir := filepath.Join(root, "Microsoft", "Windows Terminal", "Fragments")
		sources, _ := os.ReadDir(dir)
		for _, s := range sources {
			if !s.IsDir() {
				continue
			}
			files, _ := filepath.Glob(filepath.Join(dir, s.Name(), "*.json"))
			for _, f := range files {
				if b, err := regularfile.Read(f, maxWTSettings); err == nil {
					out[s.Name()] = append(out[s.Name()], b)
				}
			}
		}
	}
	return out
}

// expandEnv expands %NAME% in s as Windows does, from the daemon's
// environment; s as it is when it can't.
func expandEnv(s string) string {
	if !strings.Contains(s, "%") {
		return s
	}
	src, err := windows.UTF16PtrFromString(s)
	if err != nil {
		return s
	}
	n, err := windows.ExpandEnvironmentStrings(src, nil, 0)
	if err != nil || n == 0 {
		return s
	}
	buf := make([]uint16, n)
	if _, err := windows.ExpandEnvironmentStrings(src, &buf[0], n); err != nil {
		return s
	}
	return windows.UTF16ToString(buf)
}
