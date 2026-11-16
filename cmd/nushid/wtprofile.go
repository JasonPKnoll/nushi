package main

import (
	"encoding/json"
	"strings"

	"github.com/tailscale/hujson"
)

// Windows has no login shell, so a terminal opens the shell the user
// chose where Windows does: Windows Terminal's default profile, as a new
// tab there would (ADR 0130). This file reads its settings, JSON with
// comments, on any OS, so CI's Linux job tests it; termshell_windows.go
// finds the files and starts what it names.

// Profiles Windows Terminal makes without a command line in the user's
// settings: the system's own shells, by their fixed ids.
const (
	wtWindowsPowerShell = "{61c54bbd-c2c6-5271-96e7-009a87ff44bf}"
	wtCommandPrompt     = "{0caa0dad-35be-5f56-a8ff-afceeeaa6101}"
)

// Their command lines, as Windows Terminal starts them.
const (
	windowsPowerShellLine = `%SystemRoot%\System32\WindowsPowerShell\v1.0\powershell.exe`
	commandPromptLine     = `%SystemRoot%\System32\cmd.exe`
)

// wtProfile is a profile as Windows Terminal's settings and fragments
// hold it.
type wtProfile struct {
	GUID        string `json:"guid"`
	Name        string `json:"name"`
	Source      string `json:"source"`
	CommandLine string `json:"commandline"`
}

// wtDefaultLine is the command line, environment variables unexpanded, of
// the default profile in settings: its own, else one a fragment of its
// source gives (fragments by source, each file's text: Git for Windows's
// Git Bash, a WSL distribution's), else the system shell or PowerShell 7
// its id or source names. "" when it names none Nushi can start (Azure's
// Cloud Shell, Visual Studio's prompts) or settings can't be read.
func wtDefaultLine(settings []byte, fragments map[string][][]byte) string {
	var s struct {
		DefaultProfile string          `json:"defaultProfile"`
		Profiles       json.RawMessage `json:"profiles"`
	}
	if !jsonc(settings, &s) {
		return ""
	}
	list := wtProfiles(s.Profiles)
	if s.DefaultProfile == "" { // Windows Terminal's own default
		s.DefaultProfile = wtWindowsPowerShell
	}
	p := wtProfile{GUID: s.DefaultProfile}
	for _, q := range list {
		if strings.EqualFold(q.GUID, s.DefaultProfile) || q.Name == s.DefaultProfile {
			p = q
			break
		}
	}
	if p.CommandLine != "" {
		return p.CommandLine
	}
	for _, text := range fragments[p.Source] {
		var f struct {
			Profiles []wtProfile `json:"profiles"`
		}
		if !jsonc(text, &f) {
			continue
		}
		for _, q := range f.Profiles {
			if q.CommandLine != "" && (q.Name == p.Name || q.GUID != "" && strings.EqualFold(q.GUID, p.GUID)) {
				return q.CommandLine
			}
		}
	}
	switch {
	case strings.EqualFold(p.GUID, wtWindowsPowerShell):
		return windowsPowerShellLine
	case strings.EqualFold(p.GUID, wtCommandPrompt):
		return commandPromptLine
	case p.Source == "Windows.Terminal.PowershellCore":
		return "pwsh.exe"
	case (p.Source == "Windows.Terminal.Wsl" || p.Source == "Microsoft.WSL") && p.Name != "":
		return `%SystemRoot%\System32\wsl.exe -d "` + p.Name + `"`
	}
	return ""
}

// wtProfiles is the profiles' list, which settings hold either as a list
// or as an object's list beside its defaults.
func wtProfiles(raw json.RawMessage) []wtProfile {
	var list []wtProfile
	if json.Unmarshal(raw, &list) == nil {
		return list
	}
	var o struct {
		List []wtProfile `json:"list"`
	}
	_ = json.Unmarshal(raw, &o)
	return o.List
}

// jsonc reads JSON with comments and trailing commas into v, and says
// whether it could.
func jsonc(b []byte, v any) bool {
	std, err := hujson.Standardize(b)
	return err == nil && json.Unmarshal(std, v) == nil
}
