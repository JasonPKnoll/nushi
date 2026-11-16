package shellenv

import (
	"slices"
	"testing"
)

func TestCombine(t *testing.T) {
	system := []registryVar{
		{name: "Path", value: `%SystemRoot%\system32;%SystemRoot%`, expand: true},
		{name: "TEMP", value: `%SystemRoot%\TEMP`, expand: true},
		{name: "OS", value: "Windows_NT"},
		{name: "NUSHI_DATA", value: `C:\elsewhere`},
	}
	user := []registryVar{
		{name: "PATH", value: `%USERPROFILE%\.local\bin;%NOPE%\bin`, expand: true},
		{name: "TEMP", value: `%USERPROFILE%\AppData\Local\Temp`, expand: true},
		{name: "EDITOR", value: "%OS% 100%"}, // a REG_SZ: never expanded
	}
	base := []string{`SystemRoot=C:\Windows`, `USERPROFILE=C:\Users\me`, "Path=stale"}
	got := combine(system, user, base)
	want := []string{
		`Path=C:\Windows\system32;C:\Windows;C:\Users\me\.local\bin;%NOPE%\bin`,
		`TEMP=C:\Users\me\AppData\Local\Temp`,
		"OS=Windows_NT",
		"EDITOR=%OS% 100%",
	}
	if !slices.Equal(got, want) {
		t.Fatalf("combine =\n%q\nwant\n%q", got, want)
	}
}

func TestCombineNoSystemPath(t *testing.T) {
	got := combine(nil, []registryVar{{name: "Path", value: `C:\bin`}}, nil)
	if want := []string{`Path=C:\bin`}; !slices.Equal(got, want) {
		t.Fatalf("combine = %q, want %q", got, want)
	}
}

func TestExpand(t *testing.T) {
	vars := map[string]string{"A": "x", "B": "y"}
	lookup := func(name string) (string, bool) { v, ok := vars[name]; return v, ok }
	for in, want := range map[string]string{
		"%A%":        "x",
		"%A%%B%":     "xy",
		"50% %A%":    "50% x",
		"%none%%A%":  "%none%x",
		"%%A%":       "%x",
		"trailing %": "trailing %",
		"%%":         "%%",
	} {
		if got := expand(in, lookup); got != want {
			t.Errorf("expand(%q) = %q, want %q", in, got, want)
		}
	}
}
