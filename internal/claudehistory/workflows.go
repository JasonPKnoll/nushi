package claudehistory

import (
	"cmp"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/afrase/nushi/internal/claude"
	"github.com/afrase/nushi/internal/model"
	"github.com/afrase/nushi/internal/regularfile"
	"github.com/afrase/nushi/internal/secrets"
)

// A workflow's files are Claude's own, undocumented (docs/facts/claude.md,
// ADR 0127), in the session's folder of its history: the journal and the
// agents' transcripts in subagents/workflows/<run>/, the script in
// workflows/scripts/<name>-<run>.js, and the snapshot Claude writes as the
// run ends, workflows/<run>.json. Each is opened inside the session's
// folder (os.Root: no link or ".." leads out), only as a regular file, and
// read up to a limit; ids come only from what Claude wrote, checked for
// their shape before they name a file.

// agentStat is how often the transcripts of a run's running agents are
// looked at, for when the run last did anything (Runs.agentStat in tests).
const agentStat = 5 * time.Second

var (
	sessionID = regexp.MustCompile(`^[0-9A-Za-z][0-9A-Za-z-]{0,63}$`)
	agentID   = regexp.MustCompile(`^[A-Za-z0-9_-]{1,64}$`)
)

// Runs keeps what was read of each workflow being followed, so a read
// takes only what its journal added since the last (Local.Workflows). A
// nil Runs reads each run whole every time.
type Runs struct {
	mu        sync.Mutex
	runs      map[string]*run
	used      []string      // keys, the most recently used last
	agentStat time.Duration // how often a run's running agents are looked at
}

// NewRuns returns an empty Runs.
func NewRuns() *Runs { return &Runs{runs: map[string]*run{}, agentStat: agentStat} }

// maxRuns is how many runs Runs keeps: the ones being followed, and the
// last few looked at.
const maxRuns = 64

func (r *Runs) get(key string) *run {
	if r == nil {
		return &run{agentStat: agentStat}
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if i := slices.Index(r.used, key); i >= 0 {
		r.used = append(slices.Delete(r.used, i, i+1), key)
		return r.runs[key]
	}
	if len(r.used) >= maxRuns {
		delete(r.runs, r.used[0])
		r.used = r.used[1:]
	}
	r.runs[key] = &run{agentStat: r.agentStat}
	r.used = append(r.used, key)
	return r.runs[key]
}

// run is what was read of one workflow.
type run struct {
	mu      sync.Mutex
	j       journal
	snap    *snapshot
	snapMod time.Time
	active  time.Time
	statted time.Time
	tooBig  bool
	started time.Time // the script's time, when nothing else says
	// agentStat is how often the running agents' transcripts are looked at.
	agentStat time.Duration
	steps     map[string]steps
}

// steps are an agent's transcript as items, as read at its size and time.
type steps struct {
	size  int64
	mod   time.Time
	items []model.Item
}

// runDir opens session sid's folder in folder's history, for a run of w's,
// and the key Runs keeps it by: the run and its task, since a resumed run
// keeps its run id and gets a new task (Claude 2.1.325), and the snapshot
// of the run it resumed, another task's, must not end it.
func (l Local) runDir(sid, folder string, w model.Workflow) (*os.Root, string, error) {
	if !sessionID.MatchString(sid) || !claude.ValidRunID(w.RunID) || w.TaskID != "" && !claude.ValidTaskID(w.TaskID) {
		return nil, "", fmt.Errorf("workflow %q of session %q: %w", w.RunID, sid, fs.ErrNotExist)
	}
	dir := filepath.Join(historyDir(l.Projects, folder), sid)
	root, err := os.OpenRoot(dir)
	if err != nil {
		return nil, "", err
	}
	return root, dir + "\x00" + w.RunID + "\x00" + w.TaskID, nil
}

// ReadWorkflow reads the run w names (its RunID and TaskID, and Started,
// when the daemon saw it start) of session sid in folder: w with what its
// files say. Its State is "running" until the snapshot is there, then how
// it ended; Active is when its files last changed. An error matching
// fs.ErrNotExist says none of its files is there (yet).
func (l Local) ReadWorkflow(sid, folder string, w model.Workflow) (model.Workflow, error) {
	root, key, err := l.runDir(sid, folder, w)
	if err != nil {
		return w, err
	}
	defer root.Close() //nolint:errcheck // read-only
	r := l.Workflows.get(key)
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.snap == nil {
		r.readSnapshot(root, w)
	}
	jerr := r.readJournal(root, w.RunID)
	if r.snap == nil && jerr != nil {
		return w, jerr
	}
	if r.started.IsZero() && w.Started.IsZero() {
		r.started = scriptTime(root, w.RunID)
	}
	return r.summary(w), nil
}

// readSnapshot takes the run's snapshot, once Claude wrote it: one of
// another task (an older run with the same id), or from before the run
// started when it names no task, isn't the run's.
func (r *run) readSnapshot(root *os.Root, w model.Workflow) {
	f, fi, err := regularfile.OpenIn(root, path.Join("workflows", w.RunID+".json"), false)
	if err != nil {
		return
	}
	defer f.Close() //nolint:errcheck // read-only
	data, err := regularfile.ReadAll(f, fi, maxSnapshot)
	if err != nil {
		return
	}
	s, err := readSnapshot(data)
	switch {
	case err != nil: // being written: the next read takes it
	case s.TaskID != "" && w.TaskID != "" && s.TaskID != w.TaskID:
	case (s.TaskID == "" || w.TaskID == "") && !w.Started.IsZero() && fi.ModTime().Before(w.Started.Add(-time.Second)):
	default:
		r.snap, r.snapMod = &s, fi.ModTime()
	}
}

// readJournal reads what the journal added, and when the run last changed
// a file: the journal, or now and then an agent's transcript that runs.
func (r *run) readJournal(root *os.Root, runID string) error {
	dir := path.Join("subagents", "workflows", runID)
	f, fi, err := regularfile.OpenIn(root, path.Join(dir, "journal.jsonl"), false)
	if err != nil {
		return err
	}
	defer f.Close() //nolint:errcheck // read-only
	r.active = latest(r.active, fi.ModTime())
	if !r.tooBig {
		if err := r.j.read(f, fi.Size()); errors.Is(err, errJournalTooBig) {
			r.tooBig = true
		}
	}
	if r.snap == nil && time.Since(r.statted) >= r.agentStat {
		r.statted = time.Now()
		for _, row := range r.j.rows {
			if row.State == "running" && agentID.MatchString(row.AgentID) {
				if fi, err := root.Lstat(path.Join(dir, "agent-"+row.AgentID+".jsonl")); err == nil {
					r.active = latest(r.active, fi.ModTime())
				}
			}
		}
	}
	return nil
}

// scriptTime is when the run's script was written, the run's start when
// nothing else says: workflows/scripts/<name>-<run>.js.
func scriptTime(root *os.Root, runID string) time.Time {
	d, err := root.Open(path.Join("workflows", "scripts"))
	if err != nil {
		return time.Time{}
	}
	defer d.Close() //nolint:errcheck // read-only
	names, _ := d.Readdirnames(4096)
	for _, n := range names {
		if strings.HasSuffix(n, "-"+runID+".js") {
			if fi, err := root.Lstat(path.Join("workflows", "scripts", n)); err == nil && fi.Mode().IsRegular() {
				return fi.ModTime()
			}
		}
	}
	return time.Time{}
}

func latest(a, b time.Time) time.Time {
	if b.After(a) {
		return b
	}
	return a
}

// summary is w with what was read: while it runs, the journal's phases in
// the order reached, its counts and who runs; once it ended, the
// snapshot's.
func (r *run) summary(w model.Workflow) model.Workflow {
	w.Started = cmp.Or(w.Started, r.started)
	w.Active = r.active
	w.Phases, w.Running = nil, nil
	w.Agents, w.Done, w.Failed, w.RunningCount = len(r.j.rows), 0, 0, 0
	if s := r.snap; s != nil {
		return r.ended(w, *s)
	}
	w.State, w.End, w.Ended = "running", "", time.Time{}
	at := map[string]int{}
	for _, row := range r.j.rows {
		i := phaseOf(&w, at, row.Phase, "")
		if i >= 0 {
			w.Phases[i].Started++
		}
		switch row.State {
		case "done":
			w.Done++
			count(&w, i, func(p *model.WorkflowPhase) { p.Done++ })
		case "failed":
			w.Failed++
			count(&w, i, func(p *model.WorkflowPhase) { p.Failed++ })
		case "stopped": // the run it was in ended before it did
		default:
			w.RunningCount++
			if len(w.Running) < model.WorkflowRunning {
				w.Running = append(w.Running, label(row.Label, row.AgentID))
			}
		}
	}
	return w
}

// ended is w as the snapshot says the run ended.
func (r *run) ended(w model.Workflow, s snapshot) model.Workflow {
	w.State, w.End = endState(s.Status), s.Status
	w.Name, w.Description = cmp.Or(w.Name, clipText(s.Name, 200)), cmp.Or(w.Description, clipText(s.Summary, model.WorkflowSummaryText))
	w.Started = cmp.Or(w.Started, s.Started)
	w.Ended = r.snapMod
	if !s.Started.IsZero() && s.Duration > 0 {
		w.Ended = s.Started.Add(s.Duration)
	}
	if w.State == "failed" {
		first, _, _ := strings.Cut(strings.TrimSpace(s.Error), "\n")
		w.Summary = clipText(secrets.Mask(first, "…", nil), model.WorkflowSummaryText)
	}
	at := map[string]int{}
	for _, p := range s.Phases {
		phaseOf(&w, at, p.Title, p.Detail)
	}
	for _, a := range s.Agents {
		i := phaseOf(&w, at, a.Phase, "")
		count(&w, i, func(p *model.WorkflowPhase) { p.Started++ })
		switch agentState(a.State) {
		case "done":
			w.Done++
			count(&w, i, func(p *model.WorkflowPhase) { p.Done++ })
		case "failed":
			w.Failed++
			count(&w, i, func(p *model.WorkflowPhase) { p.Failed++ })
		}
	}
	w.Agents = max(w.Agents, s.AgentCount, len(s.Agents))
	return w
}

// endState is a workflow's state from the snapshot's status: Claude's
// words seen are completed, failed and killed; stopped and cancelled are
// its words for a stopped task elsewhere; anything else (paused) ended
// in a way Nushi can't name.
func endState(status string) string {
	switch status {
	case "completed":
		return "done"
	case "failed":
		return "failed"
	case "killed", "stopped", "cancelled":
		return "stopped"
	}
	return "unknown"
}

// agentState is an agent's state from the snapshot's row: one that
// hadn't ended when the run did was stopped with it.
func agentState(s string) string {
	switch s {
	case "done":
		return "done"
	case "error", "failed":
		return "failed"
	}
	return "stopped"
}

// phaseOf is the index of the phase titled title in w, added when new
// (WorkflowPhases at most); -1 for none.
func phaseOf(w *model.Workflow, at map[string]int, title, detail string) int {
	if title == "" {
		return -1
	}
	if i, ok := at[title]; ok {
		return i
	}
	if len(w.Phases) >= model.WorkflowPhases {
		return -1
	}
	at[title] = len(w.Phases)
	w.Phases = append(w.Phases, model.WorkflowPhase{Title: label(title, ""), Detail: clipText(detail, 200)})
	return len(w.Phases) - 1
}

func count(w *model.Workflow, i int, f func(*model.WorkflowPhase)) {
	if i >= 0 {
		f(&w.Phases[i])
	}
}

// label is a name of the agent's for the app, as untrusted plain text: a
// secret's shape masked, at most WorkflowLabel runes; or the start of id
// when there's none.
func label(s, id string) string {
	if strings.TrimSpace(s) == "" {
		s = id
		if len(s) > 8 {
			s = s[:8]
		}
	}
	s = strings.ToValidUTF8(secrets.Mask(s, "…", nil), "�")
	if utf8.RuneCountInString(s) > model.WorkflowLabel {
		s = string([]rune(s)[:model.WorkflowLabel-1]) + "…"
	}
	return s
}

// clipText cuts s to n bytes at most, at a rune's start.
func clipText(s string, n int) string {
	s = strings.ToValidUTF8(s, "�")
	if len(s) <= n {
		return s
	}
	for n > 0 && !utf8.RuneStart(s[n]) {
		n--
	}
	return s[:n]
}
