package main

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
)

// macOnly marks a test file that needs a Mac: goldens are drawn with
// macOS's text rendering, and the others read the Mac itself.
var macOnly = []string{"matchesGoldenFile(", "Platform.isMacOS", "/System/Library/"}

// linuxOnly marks a test file whose tests run on Linux alone, so it
// mustn't be in the macOS set, where every one would skip.
var linuxOnly = []string{"Platform.isLinux ? false"}

// buildsGo marks a test file that builds and runs nushid: the macOS set's
// jobs have no Go.
const buildsGo = "e2e_rig.dart"

// tests are the app's test files, by set, each a path from the app's folder.
type tests struct {
	all, macos, linux []string
	text              map[string]string
}

// readTests reads the app's test files: every test/**/*_test.dart but
// test/web's, which make test-web runs in Chrome.
func readTests(app string) (tests, error) {
	t := tests{text: map[string]string{}}
	root := filepath.Join(app, "test")
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		switch {
		case err != nil:
			return err
		case d.IsDir() && path == filepath.Join(root, "web"):
			return filepath.SkipDir
		case d.IsDir() || !strings.HasSuffix(path, "_test.dart"):
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(app, path)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)
		t.all = append(t.all, rel)
		t.text[rel] = string(data)
		return nil
	})
	if err != nil {
		return tests{}, fmt.Errorf("reading the app's tests: %w", err)
	}
	if len(t.all) == 0 {
		return tests{}, fmt.Errorf("no test files in %s", root)
	}
	slices.Sort(t.all)
	for _, f := range t.all {
		if containsAny(t.text[f], macOnly) {
			t.macos = append(t.macos, f)
		} else {
			t.linux = append(t.linux, f)
		}
	}
	return t, nil
}

// set is the files of the named set.
func (t tests) set(name string) ([]string, bool) {
	switch name {
	case "macos":
		return t.macos, true
	case "linux":
		return t.linux, true
	case "all":
		return t.all, true
	}
	return nil, false
}

// misplaced is each macOS-set file that can't run there.
func (t tests) misplaced() []string {
	var problems []string
	for _, f := range t.macos {
		text := t.text[f]
		if strings.Contains(text, buildsGo) {
			problems = append(problems, f+" builds Go ("+buildsGo+"), which the macOS set's jobs don't have: its goldens go in a file of their own")
		}
		if strings.HasSuffix(f, "_linux_test.dart") || containsAny(text, linuxOnly) {
			problems = append(problems, f+" tests Linux, but its goldens put it in the macOS set: they go in a file of their own")
		}
	}
	return problems
}

func containsAny(s string, subs []string) bool {
	return slices.ContainsFunc(subs, func(sub string) bool { return strings.Contains(s, sub) })
}
