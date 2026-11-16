package claudehistory

import (
	"bytes"
	"cmp"
	"encoding/json"
	"errors"
	"io"
)

// A workflow's journal (subagents/workflows/<run>/journal.jsonl) is read as
// it grows, from where the last read stopped; it is Claude's own, undocumented
// format (docs/facts/claude.md), so anything unexpected is passed over and
// counted, never an error.
const (
	maxJournalLine = 8 << 20  // a line longer is passed over: results can be large
	maxJournal     = 64 << 20 // the most of a journal read: thousands of agents fit
	journalChunk   = 64 << 10
)

// errJournalTooBig says the journal grew past maxJournal: the rows read so
// far stand, and nothing more is read.
var errJournalTooBig = errors.New("workflow journal too big")

// journalRow is one agent of a workflow as its journal tells: State is
// "running" from its started line, "done" from its result, "failed".
type journalRow struct {
	Key, AgentID, Label, Phase, State string
}

// journal reads a workflow's journal incrementally: off is how far it read,
// carry the start of a line not yet ended. Rows are in start order, found
// by their key (a failed line has an empty agentId). Each launched line
// starts the rows again: a resumed run replays what it cached.
type journal struct {
	off      int64
	carry    []byte
	skipping bool // within a line past maxJournalLine
	rows     []journalRow
	byKey    map[string]int
	launched bool
	skipped  int // lines passed over
}

// read reads what was added to the journal since the last read; a journal
// smaller than what was read is read again from its start (it was
// rewritten). changed says the rows changed.
func (j *journal) read(r io.ReaderAt, size int64) (changed bool, err error) {
	if size < j.off {
		*j = journal{}
		changed = true
	}
	end := min(size, maxJournal)
	buf := make([]byte, journalChunk)
	for j.off < end {
		n, err := r.ReadAt(buf[:min(int64(len(buf)), end-j.off)], j.off)
		if n == 0 && err != nil {
			return changed, err
		}
		j.off += int64(n)
		changed = j.take(buf[:n]) || changed
	}
	if size > maxJournal {
		return changed, errJournalTooBig
	}
	return changed, nil
}

// take takes a piece of the journal: each line it ends, and what follows
// its last newline as carry.
func (j *journal) take(b []byte) (changed bool) {
	for len(b) > 0 {
		i := bytes.IndexByte(b, '\n')
		if i < 0 {
			if !j.skipping && len(j.carry)+len(b) <= maxJournalLine {
				j.carry = append(j.carry, b...)
			} else {
				j.carry, j.skipping = nil, true
			}
			return changed
		}
		line := b[:i]
		b = b[i+1:]
		if j.skipping || len(j.carry)+len(line) > maxJournalLine {
			j.carry, j.skipping = nil, false
			j.skipped++
			continue
		}
		if len(j.carry) > 0 {
			line = append(j.carry, line...)
			j.carry = nil
		}
		changed = j.line(line) || changed
	}
	return changed
}

// line applies one journal line; a line that isn't one of the known kinds,
// or not as expected, is passed over.
func (j *journal) line(line []byte) bool {
	line = bytes.TrimSpace(bytes.TrimPrefix(line, []byte("\xef\xbb\xbf")))
	if len(line) == 0 {
		return false
	}
	var l struct {
		Type    string `json:"type"`
		Key     string `json:"key"`
		AgentID string `json:"agentId"`
		Label   string `json:"label"`
		Phase   string `json:"phase"`
	}
	if json.Unmarshal(line, &l) != nil {
		j.skipped++
		return false
	}
	if j.byKey == nil {
		j.byKey = map[string]int{}
	}
	i, known := j.byKey[l.Key]
	switch {
	case l.Type == "launched":
		again := j.launched
		j.launched, j.rows, j.byKey = true, nil, map[string]int{}
		return again
	case l.Key == "":
	case l.Type == "started" && known:
		r := &j.rows[i]
		r.AgentID, r.State = cmp.Or(l.AgentID, r.AgentID), "running"
		r.Label, r.Phase = cmp.Or(l.Label, r.Label), cmp.Or(l.Phase, r.Phase)
		return true
	case l.Type == "started":
		j.byKey[l.Key] = len(j.rows)
		j.rows = append(j.rows, journalRow{Key: l.Key, AgentID: l.AgentID, Label: l.Label, Phase: l.Phase, State: "running"})
		return true
	case (l.Type == "result" || l.Type == "failed") && known:
		r := &j.rows[i]
		r.AgentID, r.State = cmp.Or(r.AgentID, l.AgentID), map[string]string{"result": "done", "failed": "failed"}[l.Type]
		return true
	}
	j.skipped++
	return false
}
