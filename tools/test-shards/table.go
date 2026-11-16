package main

import (
	"bufio"
	"bytes"
	"cmp"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"

	"github.com/afrase/nushi/internal/atomicfile"
)

// header opens the table, saying what it is.
const header = `# Each app test file's weight in seconds, for tools/test-shards' split:
# its run in CI's shard of its set (macOS or Linux) plus the shard's
# median load, each shard scaled to its set's mean, since hosts differ
# in speed. Only the balance of CI's shards depends on it.
# make test-timings RUN=<CI run> rewrites it from that run's reports.
`

// readTable reads the table: its weights, and each line it couldn't read.
func readTable(path string) (table map[string]float64, bad []string, err error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, nil, fmt.Errorf("reading the timings: %w", err)
	}
	table = map[string]float64{}
	lines := bufio.NewScanner(bytes.NewReader(data))
	for n := 1; lines.Scan(); n++ {
		line := lines.Text()
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		file, secs, ok := strings.Cut(line, "\t")
		w, err := strconv.ParseFloat(secs, 64)
		_, dup := table[file]
		if !ok || err != nil || w < 0 || !strings.HasSuffix(file, "_test.dart") || dup {
			bad = append(bad, fmt.Sprintf("%s:%d isn't <test file>\\t<seconds>, once: %q", path, n, line))
			continue
		}
		table[file] = w
	}
	if err := lines.Err(); err != nil {
		return nil, nil, fmt.Errorf("reading the timings: %w", err)
	}
	return table, bad, nil
}

// median is the table's middle weight, what a file it doesn't name weighs.
func median(table map[string]float64) float64 {
	if len(table) == 0 {
		return 1
	}
	w := slices.Sorted(maps.Values(table))
	if len(w)%2 == 1 {
		return w[len(w)/2]
	}
	return (w[len(w)/2-1] + w[len(w)/2]) / 2
}

// split deals the files into n shards, longest processing time first, so
// each shard lists its files slowest first.
func split(files []string, table map[string]float64, n int) [][]string {
	def := median(table)
	weight := func(f string) float64 {
		if w, ok := table[f]; ok {
			return w
		}
		return def
	}
	order := slices.Clone(files)
	slices.SortFunc(order, func(a, b string) int {
		return cmp.Or(cmp.Compare(weight(b), weight(a)), cmp.Compare(a, b))
	})
	shards := make([][]string, n)
	loads := make([]float64, n)
	for _, f := range order {
		least := 0
		for i := range loads {
			if loads[i] < loads[least] {
				least = i
			}
		}
		shards[least] = append(shards[least], f)
		loads[least] += weight(f)
	}
	return shards
}

// tableDrift is the files the table doesn't name, and its rows for files
// that are gone.
func tableDrift(files []string, table map[string]float64) (missing, gone []string) {
	for _, f := range files {
		if _, ok := table[f]; !ok {
			missing = append(missing, f)
		}
	}
	for f := range table {
		if !slices.Contains(files, f) {
			gone = append(gone, f)
		}
	}
	slices.Sort(gone)
	return missing, gone
}

// rewriteTable writes the table anew: each of today's files at its mean
// time over the reports that ran it, else as it was, and no gone ones.
func rewriteTable(t tests, reports []string) error {
	path := filepath.Join(appDir, tablePath)
	old, _, err := readTable(path)
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return err
	}
	var runs []map[string]float64
	for _, report := range reports {
		data, err := os.ReadFile(report)
		if err != nil {
			return fmt.Errorf("reading the report: %w", err)
		}
		times := reportTimes(data)
		if len(times) == 0 {
			return fmt.Errorf("%s has no test file's time", report)
		}
		runs = append(runs, times)
	}
	evenHosts(runs, t.macos)
	sums := map[string]float64{}
	counts := map[string]int{}
	for _, times := range runs {
		for f, w := range times {
			sums[f] += w
			counts[f]++
		}
	}
	var b strings.Builder
	b.WriteString(header)
	for _, f := range t.all {
		w, ok := old[f]
		if n := counts[f]; n > 0 {
			w, ok = sums[f]/float64(n), true
		}
		if ok {
			fmt.Fprintf(&b, "%s\t%.1f\n", f, w)
		}
	}
	if err := atomicfile.Replace(path, []byte(b.String()), 0o644); err != nil {
		return fmt.Errorf("writing the timings: %w", err)
	}
	return nil
}

// evenHosts scales each report of a set to the set's mean total: CI's
// hosts differ in speed (one macOS host ran a shard four times slower
// than another, 2026-11-16), and the shards were split to weigh the same,
// so what tells one shard's total from another's is mostly its host. A
// report holds one set's files, as CI's shards do (or every file, as a
// local run's does, which then has its own set).
func evenHosts(runs []map[string]float64, macos []string) {
	mac := map[string]bool{}
	for _, f := range macos {
		mac[f] = true
	}
	set := func(times map[string]float64) string {
		n := 0
		for f := range times {
			if mac[f] {
				n++
			}
		}
		switch n {
		case 0:
			return "linux"
		case len(times):
			return "macos"
		}
		return "all"
	}
	totals := map[string][]float64{}
	for _, times := range runs {
		k := set(times)
		totals[k] = append(totals[k], sum(times))
	}
	for _, times := range runs {
		all := totals[set(times)]
		scale := mean(all) / sum(times)
		for f := range times {
			times[f] *= scale
		}
	}
}

func sum(times map[string]float64) float64 {
	var s float64
	for _, w := range times {
		s += w
	}
	return s
}

func mean(xs []float64) float64 {
	var s float64
	for _, x := range xs {
		s += x
	}
	return s / float64(len(xs))
}

// reportTimes reads flutter test's JSON report: each file's weight, the
// time from its load to its last test plus the median file's load. Its
// own load is left out: the first files to load wait for the compile.
func reportTimes(data []byte) map[string]float64 {
	type event struct {
		Type  string `json:"type"`
		Time  float64
		Suite struct {
			ID   int
			Path string
		}
		Test struct {
			ID      int
			Name    string
			SuiteID int
		}
		TestID int
		Result string
	}
	type file struct {
		loaded, end float64
		load        float64
		done        bool
	}
	paths := map[int]string{}
	files := map[string]*file{}
	starts := map[int]float64{} // a loading test's start, by its id
	suiteOf := map[int]int{}
	for line := range bytes.SplitSeq(data, []byte("\n")) {
		var e event
		if json.Unmarshal(line, &e) != nil {
			continue
		}
		switch e.Type {
		case "suite":
			paths[e.Suite.ID] = e.Suite.Path
		case "testStart":
			suiteOf[e.Test.ID] = e.Test.SuiteID
			if strings.HasPrefix(e.Test.Name, "loading ") {
				starts[e.Test.ID] = e.Time
			}
		case "testDone":
			rel := relTest(paths[suiteOf[e.TestID]])
			if rel == "" {
				continue
			}
			f := files[rel]
			if f == nil {
				f = &file{}
				files[rel] = f
			}
			if start, ok := starts[e.TestID]; ok {
				if e.Result == "success" {
					f.done, f.loaded, f.load = true, e.Time, e.Time-start
				}
				continue
			}
			f.end = max(f.end, e.Time)
		}
	}
	loads := map[string]float64{}
	for rel, f := range files {
		if f.done {
			loads[rel] = f.load
		}
	}
	load := median(loads)
	times := map[string]float64{}
	for rel, f := range files {
		if f.done {
			times[rel] = (max(f.end, f.loaded) - f.loaded + load) / 1000
		}
	}
	return times
}

// relTest is a test file's path from the app's folder, or "" for one
// outside the app's tests.
func relTest(abs string) string {
	abs = filepath.ToSlash(abs)
	i := strings.LastIndex(abs, "/"+appDir+"/test/")
	if i < 0 {
		return ""
	}
	return abs[i+len(appDir)+2:]
}
