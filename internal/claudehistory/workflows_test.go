package claudehistory

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/afrase/claude-acp-go/claudeagent/sessions"

	"github.com/afrase/nushi/internal/model"
	"github.com/afrase/nushi/internal/testcheck"
)

// recorded copies a parity scenario's history into a config folder: its
// Local, the session's id and folder, and the session's own folder.
func recorded(t *testing.T, scenario string) (Local, string, string, string) {
	t.Helper()
	end := filepath.Join("..", "features", "chats", "testdata", "claude-parity", scenario, "history", "end")
	transcripts, err := filepath.Glob(filepath.Join(end, "*.jsonl"))
	if err != nil || len(transcripts) != 1 {
		t.Fatalf("the recording's session: %v, %v", transcripts, err)
	}
	id := strings.TrimSuffix(filepath.Base(transcripts[0]), ".jsonl")
	config, folder := t.TempDir(), t.TempDir()
	project := sessions.Store{ConfigDir: config}.ProjectDir(folder)
	testcheck.NoErr(t, os.MkdirAll(filepath.Dir(project), 0o700))
	testcheck.NoErr(t, os.CopyFS(project, os.DirFS(end)))
	return Local{Projects: filepath.Join(config, "projects"), Workflows: NewRuns()}, id, folder, filepath.Join(project, id)
}

func runOf(t *testing.T, session string) model.Workflow {
	t.Helper()
	runs, _ := filepath.Glob(filepath.Join(session, "subagents", "workflows", "wf_*"))
	if len(runs) != 1 {
		t.Fatalf("runs: %v", runs)
	}
	return model.Workflow{RunID: filepath.Base(runs[0])}
}

// The recorded run ended: done, with its four agents in two phases; its
// agents list with their labels, and one's steps read as items.
func TestWorkflowOfARecording(t *testing.T) {
	l, id, folder, session := recorded(t, "workflow")
	w := runOf(t, session)
	got, err := l.ReadWorkflow(id, folder, w)
	testcheck.NoErr(t, err)
	if got.State != "done" || got.End != "completed" || got.Agents != 4 || got.Done != 4 || len(got.Phases) != 2 ||
		got.Phases[0].Title != "Map" || got.Phases[0].Detail != "two helpers" || got.Phases[1].Done != 2 || got.Ended.IsZero() || got.Started.IsZero() {
		t.Errorf("got %+v", got)
	}
	agents, err := l.WorkflowAgents(id, folder, w)
	testcheck.NoErr(t, err)
	if len(agents) != 4 || agents[0].Label != "map:alpha" || agents[3].State != "done" || agents[3].Phase != "Review" || agents[0].Prompt == "" {
		t.Fatalf("agents: %+v", agents)
	}
	items, earlier, err := l.WorkflowAgentItems(id, folder, w, agents[3].ID, 400)
	testcheck.NoErr(t, err)
	if earlier != 0 || len(items) == 0 || !strings.HasPrefix(items[0].ID, "wf/"+agents[3].ID+"/") {
		t.Errorf("steps: %d, %+v", earlier, items)
	}
	var call *model.Item
	for i := range items {
		if items[i].Tool == "StructuredOutput" {
			call = &items[i]
		}
	}
	if call == nil || call.Status != "completed" || !strings.Contains(call.Input, "delta") {
		t.Errorf("the schema's call: %+v", call)
	}
	if _, _, err := l.WorkflowAgentItems(id, folder, w, "a0000000000000000", 400); !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("an agent the run doesn't list: %v", err)
	}
}

// The stopped run's snapshot says killed: stopped, its agent too.
func TestStoppedWorkflowOfARecording(t *testing.T) {
	l, id, folder, session := recorded(t, "workflow-stop")
	w := runOf(t, session)
	got, err := l.ReadWorkflow(id, folder, w)
	testcheck.NoErr(t, err)
	agents, _ := l.WorkflowAgents(id, folder, w)
	if got.State != "stopped" || got.End != "killed" || len(agents) != 1 || agents[0].State != "stopped" {
		t.Errorf("got %+v, %+v", got, agents)
	}
}

// fakeRun writes a run's journal (and snapshot, when given) into a fresh
// session's folder.
func fakeRun(t *testing.T, journal, snapshot string) (Local, string, string, string) {
	t.Helper()
	config, folder := t.TempDir(), t.TempDir()
	id := "11111111-2222-3333-4444-555555555555"
	session := filepath.Join(sessions.Store{ConfigDir: config}.ProjectDir(folder), id)
	if journal != "" {
		testcheck.Write(t, filepath.Join(session, "subagents", "workflows", "wf_abc", "journal.jsonl"), journal)
	}
	if snapshot != "" {
		testcheck.Write(t, filepath.Join(session, "workflows", "wf_abc.json"), snapshot)
	}
	return Local{Projects: filepath.Join(config, "projects"), Workflows: NewRuns()}, id, folder, session
}

// While it runs: the phases reached, who runs, the counts; the journal
// growing between reads.
func TestRunningWorkflow(t *testing.T) {
	l, id, folder, session := fakeRun(t, launchedLine+startA+startB, "")
	w := model.Workflow{RunID: "wf_abc", TaskID: "t1"}
	got, err := l.ReadWorkflow(id, folder, w)
	testcheck.NoErr(t, err)
	if got.State != "running" || got.RunningCount != 2 || strings.Join(got.Running, ",") != "map:a,map:b" || len(got.Phases) != 1 || got.Phases[0].Started != 2 {
		t.Fatalf("got %+v", got)
	}
	f, err := os.OpenFile(filepath.Join(session, "subagents", "workflows", "wf_abc", "journal.jsonl"), os.O_APPEND|os.O_WRONLY, 0)
	testcheck.NoErr(t, err)
	_, _ = f.WriteString(resultA)
	_ = f.Close()
	got, _ = l.ReadWorkflow(id, folder, w)
	if got.Done != 1 || got.RunningCount != 1 || got.Phases[0].Done != 1 {
		t.Errorf("after a result: %+v", got)
	}
}

// Nothing of the run yet is fs.ErrNotExist; a bad id names no file.
func TestNoWorkflowFiles(t *testing.T) {
	l, id, folder, _ := fakeRun(t, "", "")
	if _, err := l.ReadWorkflow(id, folder, model.Workflow{RunID: "wf_abc"}); !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("none: %v", err)
	}
	for _, w := range []model.Workflow{{RunID: "wf_../x"}, {RunID: "x"}, {RunID: "wf_abc", TaskID: "../t"}} {
		if _, err := l.ReadWorkflow(id, folder, w); err == nil {
			t.Errorf("%+v: read", w)
		}
	}
	if _, err := l.ReadWorkflow("../x", folder, model.Workflow{RunID: "wf_abc"}); err == nil {
		t.Error("a session id with a path")
	}
}

// Links and pipes aren't read: a journal linked out of the session, a run
// folder that's a link, a journal that's a fifo or a folder.
func TestWorkflowFilesStayInside(t *testing.T) {
	outside := t.TempDir()
	testcheck.Write(t, filepath.Join(outside, "journal.jsonl"), launchedLine+startA)
	for name, mk := range map[string]func(run string){
		"a linked journal": func(run string) {
			testcheck.NoErr(t, os.MkdirAll(run, 0o700))
			testcheck.NoErr(t, os.Symlink(filepath.Join(outside, "journal.jsonl"), filepath.Join(run, "journal.jsonl")))
		},
		"a linked run": func(run string) {
			testcheck.NoErr(t, os.MkdirAll(filepath.Dir(run), 0o700))
			testcheck.NoErr(t, os.Symlink(outside, run))
		},
		"a fifo": func(run string) {
			testcheck.NoErr(t, os.MkdirAll(run, 0o700))
			testcheck.NoErr(t, syscall.Mkfifo(filepath.Join(run, "journal.jsonl"), 0o600))
		},
		"a folder": func(run string) { testcheck.NoErr(t, os.MkdirAll(filepath.Join(run, "journal.jsonl"), 0o700)) },
	} {
		l, id, folder, session := fakeRun(t, "", "")
		mk(filepath.Join(session, "subagents", "workflows", "wf_abc"))
		done := make(chan struct{})
		go func() {
			defer close(done)
			if got, err := l.ReadWorkflow(id, folder, model.Workflow{RunID: "wf_abc"}); err == nil {
				t.Errorf("%s: read %+v", name, got)
			}
		}()
		select {
		case <-done:
		case <-time.After(5 * time.Second):
			t.Fatalf("%s: the read hangs", name)
		}
	}
}

// The snapshot is the run's only when its task is the call's, or, when
// either names none, it was written after the run started; each status
// maps to an end, and the agents still running when it ended stopped.
func TestWorkflowSnapshots(t *testing.T) {
	launch := time.Now().Add(-time.Hour)
	for _, c := range []struct {
		name, snap string
		w          model.Workflow
		old        bool
		want, end  string
	}{
		{"its task", `{"status":"completed","taskId":"t1"}`, model.Workflow{TaskID: "t1"}, false, "done", "completed"},
		{"another task", `{"status":"completed","taskId":"t2"}`, model.Workflow{TaskID: "t1"}, false, "running", ""},
		{"no task, older", `{"status":"completed"}`, model.Workflow{TaskID: "t1", Started: launch}, true, "running", ""},
		{"no task, newer", `{"status":"completed"}`, model.Workflow{TaskID: "t1", Started: launch}, false, "done", "completed"},
		{"failed", `{"status":"failed","error":"Error: boom\n  at x"}`, model.Workflow{}, false, "failed", "failed"},
		{"killed", `{"status":"killed"}`, model.Workflow{}, false, "stopped", "killed"},
		{"stopped", `{"status":"stopped"}`, model.Workflow{}, false, "stopped", "stopped"},
		{"cancelled", `{"status":"cancelled"}`, model.Workflow{}, false, "stopped", "cancelled"},
		{"paused", `{"status":"paused"}`, model.Workflow{}, false, "unknown", "paused"},
		{"a new word", `{"status":"zapped"}`, model.Workflow{}, false, "unknown", "zapped"},
		{"no status", `{}`, model.Workflow{}, false, "unknown", ""},
		{"a number", `{"status":3}`, model.Workflow{}, false, "unknown", ""},
		{"half written", `{"status":"compl`, model.Workflow{}, false, "running", ""},
	} {
		l, id, folder, session := fakeRun(t, launchedLine+startA, c.snap)
		if c.old {
			p := filepath.Join(session, "workflows", "wf_abc.json")
			testcheck.NoErr(t, os.Chtimes(p, launch.Add(-time.Hour), launch.Add(-time.Hour)))
		}
		c.w.RunID = "wf_abc"
		got, err := l.ReadWorkflow(id, folder, c.w)
		testcheck.NoErr(t, err)
		if got.State != c.want || got.End != c.end {
			t.Errorf("%s: %q %q, want %q %q", c.name, got.State, got.End, c.want, c.end)
		}
		if c.name == "failed" && got.Summary != "Error: boom" {
			t.Errorf("failed: summary %q", got.Summary)
		}
		if agents, _ := l.WorkflowAgents(id, folder, c.w); c.want == "stopped" && (len(agents) != 1 || agents[0].State != "stopped") {
			t.Errorf("%s: agents %+v", c.name, agents)
		}
	}
}

// Loose types in a snapshot are read or zero, never an error; bad rows are
// passed over.
func TestSnapshotLooseTypes(t *testing.T) {
	s, err := readSnapshot([]byte(`{"status":"completed","durationMs":"1500","startTime":"2026-11-16T03:07:16Z","agentCount":"x",
		"phases":[{"title":"A"},{"detail":"no title"},7],"workflowProgress":[{"type":"workflow_phase"},{"type":"workflow_agent","agentId":"a1","tokens":-5,"durationMs":1e30},"x"]}`))
	testcheck.NoErr(t, err)
	if s.Duration != 1500*time.Millisecond || s.Started.IsZero() || s.AgentCount != 0 || len(s.Phases) != 1 || len(s.Agents) != 1 || s.Agents[0].Tokens != 0 || s.Agents[0].Duration != 0 {
		t.Errorf("got %+v", s)
	}
	if _, err := readSnapshot([]byte(`[]`)); err == nil {
		t.Error("not an object")
	}
}

// The summary on the item stays small whatever the run: phases, running
// labels and their length capped, a secret's shape masked.
func TestWorkflowSummaryBounds(t *testing.T) {
	var b strings.Builder
	b.WriteString(launchedLine)
	for i := range 500 {
		b.WriteString(`{"type":"started","key":"k` + itoa(i) + `","agentId":"a` + itoa(i) + `","label":"` + strings.Repeat("長", 80) + `","phase":"P` + itoa(i%40) + `"}` + "\n")
	}
	b.WriteString(`{"type":"started","key":"s","agentId":"as","label":"ghp_abcdefghijklmnopqrstuvwxyz0123","phase":"P0"}` + "\n")
	l, id, folder, _ := fakeRun(t, b.String(), "")
	got, err := l.ReadWorkflow(id, folder, model.Workflow{RunID: "wf_abc"})
	testcheck.NoErr(t, err)
	if len(got.Phases) != model.WorkflowPhases || len(got.Running) != model.WorkflowRunning || got.RunningCount != 501 {
		t.Errorf("phases %d, running %d of %d", len(got.Phases), len(got.Running), got.RunningCount)
	}
	if n := len([]rune(got.Running[0])); n > model.WorkflowLabel {
		t.Errorf("a label of %d runes", n)
	}
	if size := len(testcheck.JSON(t, got)); size > 4096 {
		t.Errorf("the summary is %d bytes", size)
	}
	agents, _ := l.WorkflowAgents(id, folder, model.Workflow{RunID: "wf_abc"})
	if last := agents[len(agents)-1].Label; strings.Contains(last, "abcdefghijklmnop") {
		t.Errorf("a secret's shape kept: %q", last)
	}
}

func itoa(i int) string { return strconv.Itoa(i) }
