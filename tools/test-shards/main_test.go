package main

import (
	"io"
	"maps"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/afrase/nushi/internal/testcheck"
)

// The real test files, split as CI splits them: in every set and every
// shard count, each file is in exactly one shard, and the two hosts' sets
// together are every file, once.
func TestSplitCoversTheAppsTests(t *testing.T) {
	app := filepath.Join("..", "..", appDir)
	found, err := readTests(app)
	testcheck.NoErr(t, err)
	table, bad, err := readTable(filepath.Join(app, tablePath))
	testcheck.NoErr(t, err)
	if len(bad) > 0 {
		t.Errorf("malformed timings: %q", bad)
	}
	if len(found.macos) == 0 || len(found.linux) == 0 {
		t.Fatalf("a set is empty: %d on macOS, %d on Linux", len(found.macos), len(found.linux))
	}
	for _, set := range []string{"macos", "linux", "all"} {
		files, _ := found.set(set)
		for n := 1; n <= 6; n++ {
			if err := covers(files, split(files, table, n)); err != nil {
				t.Errorf("set %s in %d shards: %v", set, n, err)
			}
		}
	}
	if err := covers(found.all, [][]string{found.macos, found.linux}); err != nil {
		t.Error(err)
	}
	if p := found.misplaced(); len(p) > 0 {
		t.Errorf("misplaced: %q", p)
	}
}

func TestSets(t *testing.T) {
	app := t.TempDir()
	files := map[string]string{
		"test/golden_test.dart":       "await expectLater(f, matchesGoldenFile('x.png'));",
		"test/mac_test.dart":          "closeTo(1, Platform.isMacOS ? 0.05 : 0.1)",
		"test/font_test.dart":         "const f = '/System/Library/Fonts/X.ttc';",
		"test/plain_test.dart":        "expect(1, 1);",
		"test/deep/nested_test.dart":  "expect(1, 1);",
		"test/web/browser_test.dart":  "matchesGoldenFile('w.png')",
		"test/harness.dart":           "matchesGoldenFile(name)",
		"test/goldens/readme_test.md": "",
	}
	for f, text := range files {
		testcheck.Write(t, filepath.Join(app, f), text)
	}
	found, err := readTests(app)
	testcheck.NoErr(t, err)
	if want := []string{"test/font_test.dart", "test/golden_test.dart", "test/mac_test.dart"}; !slices.Equal(found.macos, want) {
		t.Errorf("macOS set %q, want %q", found.macos, want)
	}
	if want := []string{"test/deep/nested_test.dart", "test/plain_test.dart"}; !slices.Equal(found.linux, want) {
		t.Errorf("Linux set %q, want %q", found.linux, want)
	}
}

func TestMisplaced(t *testing.T) {
	found := tests{
		macos: []string{"test/e2e_test.dart", "test/x_linux_test.dart", "test/skip_test.dart", "test/ok_test.dart"},
		text: map[string]string{
			"test/e2e_test.dart":     "import 'e2e_rig.dart'; matchesGoldenFile('a')",
			"test/x_linux_test.dart": "matchesGoldenFile('a')",
			"test/skip_test.dart":    "}, skip: Platform.isLinux ? false : 'Linux only'); matchesGoldenFile('a')",
			"test/ok_test.dart":      "matchesGoldenFile('a')",
		},
	}
	got := strings.Join(found.misplaced(), "\n")
	for _, f := range []string{"e2e_test", "x_linux_test", "skip_test"} {
		if !strings.Contains(got, f) {
			t.Errorf("%s isn't reported: %s", f, got)
		}
	}
	if strings.Contains(got, "ok_test") {
		t.Errorf("a golden file is reported: %s", got)
	}
}

// Longest first, each to the least loaded shard; a file the table doesn't
// name weighs its median.
func TestSplit(t *testing.T) {
	table := map[string]float64{"a": 5, "b": 4, "c": 3, "d": 3, "e": 1}
	got := split([]string{"e", "d", "c", "b", "a"}, table, 2)
	want := [][]string{{"a", "d"}, {"b", "c", "e"}}
	if !slices.EqualFunc(got, want, slices.Equal) {
		t.Errorf("split %q, want %q", got, want)
	}
	got = split([]string{"a", "new", "e"}, table, 3)
	want = [][]string{{"a"}, {"new"}, {"e"}}
	if !slices.EqualFunc(got, want, slices.Equal) {
		t.Errorf("with a new file %q, want %q", got, want)
	}
	if got := split([]string{"a"}, table, 2); len(got[1]) != 0 {
		t.Errorf("one file in two shards: %q", got)
	}
}

func TestCovers(t *testing.T) {
	files := []string{"a", "b", "c"}
	if err := covers(files, [][]string{{"a", "c"}, {"b"}}); err != nil {
		t.Error(err)
	}
	err := covers(files, [][]string{{"a", "b"}, {"b", "x"}})
	if err == nil {
		t.Fatal("a dropped, repeated and extra file passed")
	}
	for _, want := range []string{"c in no shard", "b in 2 shards", "x isn't"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("%q doesn't say %q", err, want)
		}
	}
}

func TestParseShard(t *testing.T) {
	if i, n, err := parseShard("2/3"); err != nil || i != 2 || n != 3 {
		t.Errorf("2/3: %d, %d, %v", i, n, err)
	}
	for _, s := range []string{"3/2", "0/2", "1/0", "1", "a/b", "1/2/3", ""} {
		if _, _, err := parseShard(s); err == nil {
			t.Errorf("%q passed", s)
		}
	}
	if code := run([]string{"-shard", "3/2"}, io.Discard, io.Discard); code != 2 {
		t.Errorf("a shard past the count exits %d", code)
	}
	if code := run([]string{"-set", "windows"}, io.Discard, io.Discard); code != 2 {
		t.Errorf("an unknown set exits %d", code)
	}
}

func TestReadTable(t *testing.T) {
	path := filepath.Join(t.TempDir(), "t.tsv")
	testcheck.Write(t, path, "# a comment\n\ntest/a_test.dart\t1.5\ntest/b_test.dart 2\ntest/c_test.dart\tx\ntest/a_test.dart\t3\nREADME\t1\n")
	table, bad, err := readTable(path)
	testcheck.NoErr(t, err)
	if len(table) != 1 || table["test/a_test.dart"] != 1.5 {
		t.Errorf("table %v", table)
	}
	if len(bad) != 4 {
		t.Errorf("bad lines %q", bad)
	}
	if median(map[string]float64{"a": 1, "b": 3, "c": 10}) != 3 || median(map[string]float64{"a": 1, "b": 3}) != 2 || median(nil) != 1 {
		t.Error("median")
	}
}

// Each file's weight is its time from loaded to its last test plus the
// median load; a file that didn't load has none.
func TestReportTimes(t *testing.T) {
	report := `{"type":"start","time":0}
{"suite":{"id":0,"path":"/r/app/test/a_test.dart"},"type":"suite","time":0}
{"test":{"id":1,"name":"loading /r/app/test/a_test.dart","suiteID":0},"type":"testStart","time":0}
{"suite":{"id":2,"path":"/r/app/test/b_test.dart"},"type":"suite","time":1}
{"test":{"id":3,"name":"loading /r/app/test/b_test.dart","suiteID":2},"type":"testStart","time":1}
{"suite":{"id":4,"path":"/r/app/test/c_test.dart"},"type":"suite","time":1}
{"test":{"id":5,"name":"loading /r/app/test/c_test.dart","suiteID":4},"type":"testStart","time":1}
{"testID":1,"result":"success","hidden":true,"type":"testDone","time":6000}
{"testID":3,"result":"success","hidden":true,"type":"testDone","time":2001}
{"testID":5,"result":"error","hidden":false,"type":"testDone","time":50}
{"test":{"id":6,"name":"one","suiteID":0},"type":"testStart","time":6000}
{"testID":6,"result":"success","hidden":false,"type":"testDone","time":7000}
{"test":{"id":7,"name":"two","suiteID":0},"type":"testStart","time":7000}
{"testID":7,"result":"failure","hidden":false,"type":"testDone","time":9500}
{"test":{"id":8,"name":"three","suiteID":2},"type":"testStart","time":2001}
{"testID":8,"result":"success","hidden":false,"type":"testDone","time":2501}
{"success":false,"type":"done","time":9600}
`
	got := reportTimes([]byte(report))
	// Loads 6 s and 2 s: their median is 4.
	want := map[string]float64{"test/a_test.dart": 3.5 + 4, "test/b_test.dart": 0.5 + 4}
	if len(got) != len(want) {
		t.Fatalf("times %v, want %v", got, want)
	}
	for f, w := range want {
		if d := got[f] - w; d > 1e-9 || d < -1e-9 {
			t.Errorf("%s: %v, want %v", f, got[f], w)
		}
	}
}

// Each file at its mean over the reports that ran it; a file none ran
// keeps its row, a gone file's row goes.
func TestRewriteTable(t *testing.T) {
	root := t.TempDir()
	for _, f := range []string{"a_test.dart", "b_test.dart", "c_test.dart"} {
		testcheck.Write(t, filepath.Join(root, appDir, "test", f), "")
	}
	testcheck.Write(t, filepath.Join(root, appDir, tablePath), "test/c_test.dart\t9\ntest/gone_test.dart\t3\n")
	report := func(name string, ms int) string {
		path := filepath.Join(root, name)
		testcheck.Write(t, path, `{"suite":{"id":0,"path":"/x/app/test/a_test.dart"},"type":"suite","time":0}
{"test":{"id":1,"name":"loading a","suiteID":0},"type":"testStart","time":0}
{"testID":1,"result":"success","type":"testDone","time":1000}
{"test":{"id":2,"name":"one","suiteID":0},"type":"testStart","time":1000}
{"testID":2,"result":"success","type":"testDone","time":`+strconv.Itoa(1000+ms)+`}
`)
		return path
	}
	t.Chdir(root)
	found, err := readTests(appDir)
	testcheck.NoErr(t, err)
	testcheck.NoErr(t, rewriteTable(found, []string{report("r1.json", 2000), report("r2.json", 4000)}))
	table, bad, err := readTable(filepath.Join(appDir, tablePath))
	testcheck.NoErr(t, err)
	// Each report: the run plus its median load, 1 s.
	want := map[string]float64{"test/a_test.dart": 4, "test/c_test.dart": 9}
	if len(bad) > 0 || !maps.Equal(table, want) {
		t.Errorf("table %v (bad %q), want %v", table, bad, want)
	}
	if code := run([]string{"-timings"}, io.Discard, io.Discard); code != 2 {
		t.Errorf("-timings without a report exits %d", code)
	}
	if code := run([]string{"r1.json"}, io.Discard, io.Discard); code != 2 {
		t.Errorf("a report without -timings exits %d", code)
	}
}

// A slow host's shard and a fast one's weigh the same once scaled to
// their set's mean; another set's report is scaled by its own.
func TestEvenHosts(t *testing.T) {
	runs := []map[string]float64{
		{"test/a_test.dart": 2, "test/b_test.dart": 2},
		{"test/c_test.dart": 8, "test/d_test.dart": 8},
		{"test/e_test.dart": 3},
	}
	evenHosts(runs, []string{"test/a_test.dart", "test/b_test.dart", "test/c_test.dart", "test/d_test.dart"})
	want := []map[string]float64{
		{"test/a_test.dart": 5, "test/b_test.dart": 5},
		{"test/c_test.dart": 5, "test/d_test.dart": 5},
		{"test/e_test.dart": 3},
	}
	for i := range runs {
		if !maps.Equal(runs[i], want[i]) {
			t.Errorf("report %d: %v, want %v", i, runs[i], want[i])
		}
	}
}
