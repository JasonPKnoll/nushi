package main

import "testing"

// The daemon runs with no arguments or -login; anything else is refused,
// so `nushid -h` starts no daemon.
func TestDaemonArgs(t *testing.T) {
	for _, tc := range []struct {
		args  []string
		login bool
		ok    bool
	}{
		{nil, false, true},
		{[]string{"-login"}, true, true},
		{[]string{"-psn_0_12345", "-login"}, true, true},
		{[]string{"-h"}, false, false},
		{[]string{"--help"}, false, false},
		{[]string{"-login", "extra"}, false, false},
		{[]string{"login"}, false, false},
	} {
		login, err := daemonArgs(tc.args)
		if login != tc.login || (err == nil) != tc.ok {
			t.Errorf("daemonArgs(%q) = %v, %v", tc.args, login, err)
		}
	}
}

// A terminal is named for its shell's program, without its folder or a
// Windows .exe, whatever its case.
func TestShellName(t *testing.T) {
	for program, want := range map[string]string{
		"/bin/zsh":      "zsh",
		"/usr/bin/fish": "fish",
		"pwsh.exe":      "pwsh",
		"cmd.EXE":       "cmd",
		"bash":          "bash",
		"nu.exe.sh":     "nu.exe.sh",
	} {
		if got := shellName(program); got != want {
			t.Errorf("shellName(%q) = %q, want %q", program, got, want)
		}
	}
}
