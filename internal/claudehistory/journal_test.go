package claudehistory

import (
	"bytes"
	"errors"
	"math/rand/v2"
	"strings"
	"testing"
)

// readAll reads a whole journal at once.
func readAll(t *testing.T, s string) journal {
	t.Helper()
	var j journal
	if err := j.read(strings.NewReader(s), int64(len(s))); err != nil {
		t.Fatal(err)
	}
	return j
}

func states(j journal) string {
	var b strings.Builder
	for _, r := range j.rows {
		b.WriteString(r.Label + "=" + r.State + ":" + r.AgentID + " ")
	}
	return strings.TrimSpace(b.String())
}

const (
	launchedLine = `{"type":"launched"}` + "\n"
	startA       = `{"type":"started","key":"k1","agentId":"a1","label":"map:a","phase":"Map"}` + "\n"
	startB       = `{"type":"started","key":"k2","agentId":"a2","label":"map:b","phase":"Map"}` + "\n"
	resultA      = `{"type":"result","key":"k1","agentId":"a1","result":"alpha"}` + "\n"
)

// Each kind of line, matched by its key: results of every shape, a failed
// line with no agent id, lines for keys never started, a retry, a restart
// after a result, and lines Claude doesn't write.
func TestJournalLines(t *testing.T) {
	big := `{"type":"result","key":"k2","agentId":"a2","result":{"x":"` + strings.Repeat("y", 200<<10) + `"},"toolsUsed":["Read"],"foreignArtifactContent":null,"remoteControlDeviceReport":false}` + "\n"
	for _, c := range []struct{ name, journal, want string }{
		{"launched only", launchedLine, ""},
		{"started", launchedLine + startA, "map:a=running:a1"},
		{"a string result", launchedLine + startA + resultA, "map:a=done:a1"},
		{"an object result", launchedLine + startB + big, "map:b=done:a2"},
		{"a null result", launchedLine + startA + `{"type":"result","key":"k1","result":null}` + "\n", "map:a=done:a1"},
		{"no result field", launchedLine + startA + `{"type":"result","key":"k1"}` + "\n", "map:a=done:a1"},
		{"failed, no agent id", launchedLine + startA + `{"type":"failed","key":"k1","agentId":""}` + "\n", "map:a=failed:a1"},
		{"never started", launchedLine + resultA + `{"type":"failed","key":"zz"}` + "\n", ""},
		{"a retry", launchedLine + startA + `{"type":"started","key":"k1","agentId":"a9"}` + "\n", "map:a=running:a9"},
		{"started again", launchedLine + startA + resultA + startA, "map:a=running:a1"},
		{"unknown kinds", launchedLine + `{"type":"log"}` + "\n" + `{"type":42}` + "\n" + `{}` + "\n" + startA, "map:a=running:a1"},
		{"no key", launchedLine + `{"type":"started","agentId":"a1"}` + "\n", ""},
		{"no label or phase", launchedLine + `{"type":"started","key":"k1","agentId":"a1"}` + "\n", "=running:a1"},
		{"extra fields", launchedLine + `{"type":"started","key":"k1","agentId":"a1","label":"map:a","new":[1]}` + "\n", "map:a=running:a1"},
		{"wrong types", launchedLine + `{"type":"started","key":7}` + "\n" + `{"type":"started","key":"k3","label":{}}` + "\n" + startA, "map:a=running:a1"},
		{"bad lines", "\xef\xbb\xbf" + launchedLine + "{nope\n\n\r\n" + strings.TrimSuffix(startA, "\n") + "\r\n", "map:a=running:a1"},
		{"invalid UTF-8", launchedLine + `{"type":"started","key":"k1","agentId":"a1","label":"x` + "\xff" + `"}` + "\n", "x�=running:a1"},
		{"no launched", startA, "map:a=running:a1"},
	} {
		if got := states(readAll(t, c.journal)); got != c.want {
			t.Errorf("%s: %q, want %q", c.name, got, c.want)
		}
	}
}

// A line without its newline yet waits for it; one split anywhere, even
// inside a rune, reads the same; an over-long line is passed over without
// being held, and the lines after it read.
func TestJournalGrows(t *testing.T) {
	var j journal
	s := launchedLine + strings.TrimSuffix(startA, "\n")
	_ = j.read(strings.NewReader(s), int64(len(s)))
	if len(j.rows) != 0 {
		t.Fatalf("a line without its end: %q", states(j))
	}
	s += "\n"
	_ = j.read(strings.NewReader(s), int64(len(s)))
	if states(j) != "map:a=running:a1" {
		t.Fatalf("once ended: %q", states(j))
	}

	long := launchedLine + `{"type":"result","key":"k1","result":"` + strings.Repeat("y", maxJournalLine) + `"}` + "\n" + startA
	var k journal
	for end := 0; end < len(long); {
		end = min(len(long), end+3<<20)
		_ = k.read(strings.NewReader(long[:end]), int64(end))
		if len(k.carry) > maxJournalLine {
			t.Fatalf("carry %d", len(k.carry))
		}
	}
	if states(k) != "map:a=running:a1" || k.skipped != 1 {
		t.Errorf("past a long line: %q, skipped %d", states(k), k.skipped)
	}
}

// A journal that shrank is read again; one past maxJournal keeps its rows
// and says so; a read with nothing new reads nothing.
func TestJournalRewindAndLimits(t *testing.T) {
	var j journal
	s := launchedLine + startA + startB
	_ = j.read(strings.NewReader(s), int64(len(s)))
	s = launchedLine + startB
	_ = j.read(strings.NewReader(s), int64(len(s)))
	if states(j) != "map:b=running:a2" {
		t.Errorf("rewritten: %q", states(j))
	}
	r := &countingReader{Reader: strings.NewReader(s)}
	_ = j.read(r, int64(len(s)))
	if r.n != 0 {
		t.Errorf("read %d bytes with nothing new", r.n)
	}
	if err := j.read(zeros{}, maxJournal+1); !errors.Is(err, errJournalTooBig) {
		t.Errorf("too big: %v", err)
	}
}

// zeros is a journal of newlines, as long as asked.
type zeros struct{}

func (zeros) ReadAt(p []byte, _ int64) (int, error) {
	for i := range p {
		p[i] = '\n'
	}
	return len(p), nil
}

type countingReader struct {
	*strings.Reader
	n int
}

func (c *countingReader) ReadAt(p []byte, off int64) (int, error) {
	n, err := c.Reader.ReadAt(p, off)
	c.n += n
	return n, err
}

// A resumed run appends a resumed line to the same journal (Claude
// 2.1.325 appends no launched line for it, and keeps the results it has):
// the agents it had done stay done, the one it starts again runs, and one
// the killed run left without a result and the resume doesn't start again
// is stopped. A hidden script's resume appends a launched line instead,
// which reads the same.
func TestJournalResume(t *testing.T) {
	const resumed = `{"type":"resumed"}` + "\n"
	startC := `{"type":"started","key":"k3","agentId":"a3","label":"map:c","phase":"Map"}` + "\n"
	for name, again := range map[string]string{"resumed": resumed, "a hidden script's": launchedLine} {
		j := readAll(t, launchedLine+startA+resultA+startB+startC+again+startB)
		if got := states(j); got != "map:a=done:a1 map:b=running:a2 map:c=stopped:a3" {
			t.Errorf("%s: got %q", name, got)
		}
	}
}

// Read in random pieces, generated journals give the rows one read gives.
func TestJournalIncrementalIsWhole(t *testing.T) {
	rng := rand.New(rand.NewPCG(1, 2)) //nolint:gosec // seeded, so each run tries the same journals
	lines := []string{launchedLine, startA, startB, resultA, `{"type":"failed","key":"k2","agentId":""}` + "\n", "{bad\n",
		`{"type":"started","key":"k3","agentId":"a3","label":"é\u6f22","phase":"Review"}` + "\n", `{"type":"result","key":"k3","result":{"a":1}}` + "\n"}
	for range 500 {
		var b bytes.Buffer
		for range rng.IntN(20) {
			b.WriteString(lines[rng.IntN(len(lines))])
		}
		s := b.String()
		whole := readAll(t, s)
		var j journal
		for end := 0; end < len(s); {
			end = min(len(s), end+1+rng.IntN(40))
			_ = j.read(strings.NewReader(s[:end]), int64(end))
		}
		if states(j) != states(whole) {
			t.Fatalf("%q: %q, whole %q", s, states(j), states(whole))
		}
	}
}

func FuzzJournal(f *testing.F) {
	for _, s := range []string{launchedLine + startA + resultA, launchedLine + "{bad\n" + startB, "\xef\xbb\xbf" + startA, ""} {
		f.Add(s, 3)
	}
	f.Fuzz(func(t *testing.T, s string, step int) {
		var j journal
		step = max(1, step%64)
		var off int64
		for end := 0; end < len(s); {
			end = min(len(s), end+step)
			_ = j.read(strings.NewReader(s[:end]), int64(end))
			if j.off < off {
				t.Fatal("went back")
			}
			off = j.off
		}
		if len(j.rows) > strings.Count(s, "\n")+1 {
			t.Fatalf("%d rows from %d lines", len(j.rows), strings.Count(s, "\n"))
		}
	})
}
