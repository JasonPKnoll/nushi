// Command test-shards says which of the app's test files a test run takes:
// the files of a host's set (-set), split into shards by their time
// (-shard i/n), each shard's files slowest first. CI's jobs and make
// test-app run what it prints (ADR 0128).
//
// The macOS set is every file that needs a Mac: one that compares goldens
// (drawn with macOS's text rendering) or reads the Mac itself
// (Platform.isMacOS, a font under /System/Library). The Linux set is the
// rest. app/test/timings.tsv weighs each file by its seconds; a file it
// doesn't name weighs its median, so new files spread evenly. The split is
// longest processing time first: each file, slowest first, goes to the
// shard with the least time so far, ties to the lower shard and to the
// path, so every run splits alike.
//
// -check fails on a file in the wrong set (a macOS one that builds Go, a
// Linux-only one on the Mac), a malformed table, or a split that drops or
// repeats a file; it notes the table's stale rows. -timings rewrites the
// table from flutter test JSON reports, CI's shards' (make test-timings),
// each file at its mean time over those that ran it. Run it from the
// repository root.
package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// Where the app is, from the repository root, and its table, from the app.
const (
	appDir    = "app"
	tablePath = "test/timings.tsv"
)

// staleAfter is how many files the table may miss before -check asks for
// make test-timings: a missing file costs balance, never coverage.
const staleAfter = 30

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

// run is the command, with its exit code: 2 for a wrong use.
func run(args []string, stdout, stderr io.Writer) int {
	flags := flag.NewFlagSet("test-shards", flag.ContinueOnError)
	flags.SetOutput(stderr)
	set := flags.String("set", "all", "the files of `set`: macos, linux or all")
	shard := flags.String("shard", "1/1", "shard `i/n` of the set")
	check := flags.Bool("check", false, "check the sets, the split and the table")
	timings := flags.Bool("timings", false, "rewrite the table from the flutter test JSON reports named after the flags")
	if err := flags.Parse(args); err != nil || (flags.NArg() > 0) != *timings {
		_, _ = fmt.Fprintln(stderr, "usage: test-shards [-set s] [-shard i/n] | -check | -timings <report>...")
		return 2
	}
	i, n, err := parseShard(*shard)
	if err != nil {
		_, _ = fmt.Fprintln(stderr, "test-shards:", err)
		return 2
	}
	if _, ok := (tests{}).set(*set); !ok {
		_, _ = fmt.Fprintf(stderr, "test-shards: no set %q (macos, linux or all)\n", *set)
		return 2
	}
	found, err := readTests(appDir)
	if err != nil {
		_, _ = fmt.Fprintln(stderr, "test-shards:", err)
		return 1
	}
	switch {
	case *check:
		return runCheck(found, stdout, stderr)
	case *timings:
		if err := rewriteTable(found, flags.Args()); err != nil {
			_, _ = fmt.Fprintln(stderr, "test-shards:", err)
			return 1
		}
		return 0
	}
	files, _ := found.set(*set)
	table, _, err := readTable(filepath.Join(appDir, tablePath))
	if err != nil {
		_, _ = fmt.Fprintln(stderr, "test-shards:", err)
		return 1
	}
	picked := split(files, table, n)[i-1]
	if len(picked) == 0 {
		// flutter test given no file runs them all.
		_, _ = fmt.Fprintf(stderr, "test-shards: shard %d/%d of %s has no file\n", i, n, *set)
		return 1
	}
	for _, f := range picked {
		_, _ = fmt.Fprintln(stdout, f)
	}
	return 0
}

// parseShard reads i/n, 1 <= i <= n.
func parseShard(s string) (i, n int, err error) {
	a, b, ok := strings.Cut(s, "/")
	i, errI := strconv.Atoi(a)
	n, errN := strconv.Atoi(b)
	if !ok || errI != nil || errN != nil || n < 1 || i < 1 || i > n {
		return 0, 0, fmt.Errorf("shard %q isn't i/n with 1 <= i <= n", s)
	}
	return i, n, nil
}

// runCheck is -check: its findings, and 1 when one is wrong.
func runCheck(t tests, stdout, stderr io.Writer) int {
	table, problems, err := readTable(filepath.Join(appDir, tablePath))
	if err != nil {
		_, _ = fmt.Fprintln(stderr, "test-shards:", err)
		return 1
	}
	problems = append(problems, t.misplaced()...)
	for _, set := range []string{"macos", "linux", "all"} {
		files, _ := t.set(set)
		for n := 1; n <= 6; n++ {
			if err := covers(files, split(files, table, n)); err != nil {
				problems = append(problems, fmt.Sprintf("set %s in %d shards: %v", set, n, err))
			}
		}
	}
	if err := covers(t.all, [][]string{t.macos, t.linux}); err != nil {
		problems = append(problems, "the macOS and Linux sets: "+err.Error())
	}
	missing, gone := tableDrift(t.all, table)
	if len(gone) > 0 {
		_, _ = fmt.Fprintf(stdout, "test-shards: %s names %d files that are gone (make test-timings drops them)\n", tablePath, len(gone))
	}
	if len(missing) > 0 {
		hint := ""
		if len(missing) > staleAfter {
			hint = "; make test-timings RUN=<a full CI run>"
		}
		_, _ = fmt.Fprintf(stdout, "test-shards: %d files not in %s, weighed at its median%s\n", len(missing), tablePath, hint)
	}
	for _, p := range problems {
		_, _ = fmt.Fprintln(stderr, "test-shards:", p)
	}
	if len(problems) > 0 {
		return 1
	}
	_, _ = fmt.Fprintf(stdout, "test-shards: %d files, %d on macOS, %d on Linux\n", len(t.all), len(t.macos), len(t.linux))
	return 0
}

// covers says how the shards differ from the files: one missing or extra,
// or one in two shards.
func covers(files []string, shards [][]string) error {
	seen := map[string]int{}
	for _, shard := range shards {
		for _, f := range shard {
			seen[f]++
		}
	}
	var errs []error
	for _, f := range files {
		switch seen[f] {
		case 0:
			errs = append(errs, fmt.Errorf("%s in no shard", f))
		case 1:
		default:
			errs = append(errs, fmt.Errorf("%s in %d shards", f, seen[f]))
		}
		delete(seen, f)
	}
	for f := range seen {
		errs = append(errs, fmt.Errorf("%s isn't a test file of the set", f))
	}
	return errors.Join(errs...)
}
