package main

import "testing"

func TestWTDefaultLine(t *testing.T) {
	gitFragment := []byte(`{"profiles": [{"name": "Git Bash", "commandline": "C:\\Program Files\\Git\\bin\\bash.exe -i -l"}]}`)
	wslFragment := []byte(`{"profiles": [{"guid": "{aaaa}", "name": "Ubuntu", "commandline": "wsl.exe --distribution-id {1234}"}]}`)
	fragments := map[string][][]byte{"Git": {[]byte("not json"), gitFragment}, "Microsoft.WSL": {wslFragment}}
	for _, c := range []struct{ name, settings, want string }{
		{"Windows PowerShell, the out-of-the-box default", `{
			// comments, as Windows Terminal writes them
			"defaultProfile": "{61c54bbd-c2c6-5271-96e7-009a87ff44bf}",
			"profiles": {"defaults": {}, "list": [
				{"guid": "{61c54bbd-c2c6-5271-96e7-009a87ff44bf}", "name": "Windows PowerShell", "hidden": false},
				{"guid": "{0caa0dad-35be-5f56-a8ff-afceeeaa6101}", "name": "Command Prompt",},
			]},
		}`, windowsPowerShellLine},
		{"no default: Windows Terminal's own", `{"profiles": []}`, windowsPowerShellLine},
		{"the Command Prompt", `{"defaultProfile": "{0CAA0DAD-35BE-5F56-A8FF-AFCEEEAA6101}", "profiles": {"list": []}}`, commandPromptLine},
		{"a profile's own command line", `{"defaultProfile": "{mine}", "profiles": [
			{"guid": "{mine}", "name": "Mine", "commandline": "%USERPROFILE%\\bin\\fish.exe -l"}]}`, `%USERPROFILE%\bin\fish.exe -l`},
		{"named by its name", `{"defaultProfile": "Mine", "profiles": [
			{"guid": "{mine}", "name": "Mine", "commandline": "nu.exe"}]}`, "nu.exe"},
		{"PowerShell 7", `{"defaultProfile": "{574e775e-4f2a-5b96-ac1e-a2962a402336}", "profiles": {"list": [
			{"guid": "{574e775e-4f2a-5b96-ac1e-a2962a402336}", "name": "PowerShell", "source": "Windows.Terminal.PowershellCore"}]}}`, "pwsh.exe"},
		{"Git Bash, from Git's fragment", `{"defaultProfile": "{2ece5bfe-50ed-5f3a-ab87-5cd4baafed2b}", "profiles": {"list": [
			{"guid": "{2ece5bfe-50ed-5f3a-ab87-5cd4baafed2b}", "name": "Git Bash", "source": "Git"}]}}`, `C:\Program Files\Git\bin\bash.exe -i -l`},
		{"WSL, from its fragment", `{"defaultProfile": "{b}", "profiles": {"list": [
			{"guid": "{b}", "name": "Ubuntu", "source": "Microsoft.WSL"}]}}`, "wsl.exe --distribution-id {1234}"},
		{"WSL with no fragment", `{"defaultProfile": "{c}", "profiles": {"list": [
			{"guid": "{c}", "name": "Debian", "source": "Windows.Terminal.Wsl"}]}}`, `%SystemRoot%\System32\wsl.exe -d "Debian"`},
		{"Azure's Cloud Shell: none", `{"defaultProfile": "{d}", "profiles": {"list": [
			{"guid": "{d}", "name": "Azure Cloud Shell", "source": "Windows.Terminal.Azure"}]}}`, ""},
		{"a default no profile has", `{"defaultProfile": "{gone}", "profiles": {"list": []}}`, ""},
		{"not JSON", `{"defaultProfile": `, ""},
	} {
		if got := wtDefaultLine([]byte(c.settings), fragments); got != c.want {
			t.Errorf("%s: %q, want %q", c.name, got, c.want)
		}
	}
}
