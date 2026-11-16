package claudehistory

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/afrase/nushi/internal/model"
	"github.com/afrase/nushi/internal/testcheck"
)

// notification is a line of the session's transcript carrying a
// task-notification, as Claude 2.1.325 writes one (the workflow parity
// recording): a user line whose origin says so, or the queue's enqueue.
func notification(t *testing.T, kind, task, status, summary string) string {
	t.Helper()
	text := "<task-notification>\n<task-id>" + task + "</task-id>\n<tool-use-id>toolu_1</tool-use-id>\n<status>" + status +
		"</status>\n<summary>" + summary + "</summary>\n<result>{}</result>\n</task-notification>"
	var line map[string]any
	switch kind {
	case "user":
		line = map[string]any{"type": "user", "message": map[string]any{"role": "user", "content": text},
			"timestamp": "2026-11-16T03:07:16.232Z", "origin": map[string]any{"kind": "task-notification", "producer": "session-task"}}
	case "enqueue":
		line = map[string]any{"type": "queue-operation", "operation": "enqueue", "timestamp": "2026-11-16T03:07:16.157Z", "content": text}
	case "assistant": // the model quoting one: no end
		line = map[string]any{"type": "assistant", "message": map[string]any{"role": "assistant", "content": []any{map[string]any{"type": "text", "text": text}}}}
	case "typed": // the user's own text: no origin
		line = map[string]any{"type": "user", "message": map[string]any{"role": "user", "content": text}}
	}
	return string(testcheck.JSON(t, line)) + "\n"
}

// writeTranscript appends lines to the session's transcript, beside its
// folder.
func writeTranscript(t *testing.T, session string, lines ...string) {
	t.Helper()
	f, err := os.OpenFile(session+".jsonl", os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
	testcheck.NoErr(t, err)
	defer f.Close() //nolint:errcheck // a test's file
	for _, l := range lines {
		_, err := f.WriteString(l)
		testcheck.NoErr(t, err)
	}
}

// The notification of the run's task ends it when no snapshot comes: its
// status is the end (failed, its summary said), the agents still running
// stopped with it. One of another task, the model's quote of one, the
// user's own text and a status that is no end don't.
func TestAWorkflowEndedByItsNotification(t *testing.T) {
	l, id, folder, session := fakeRun(t, launchedLine+startA+startB+resultA, "")
	l.Workflows.snapshotWait = 0
	w := model.Workflow{RunID: "wf_abc", TaskID: "t1"}
	writeTranscript(t, session, `{"type":"user","message":{"role":"user","content":"hi"}}`+"\n", "{bad\n",
		notification(t, "user", "t2", "completed", "another"), notification(t, "assistant", "t1", "completed", "quoted"),
		notification(t, "typed", "t1", "completed", "typed"), notification(t, "user", "t1", "running", "not an end"))
	if got, err := l.ReadWorkflow(id, folder, w); err != nil || got.State != "running" {
		t.Fatalf("no notification of its own yet: %+v, %v", got, err)
	}
	writeTranscript(t, session, notification(t, "user", "t1", "failed", `Dynamic workflow "x" failed: ghp_abcdefghijklmnopqrstuvwxyz0123`))
	got, err := l.ReadWorkflow(id, folder, w)
	testcheck.NoErr(t, err)
	if got.State != "failed" || got.End != "failed" || got.Ended.IsZero() || got.RunningCount != 0 || len(got.Running) != 0 || got.Done != 1 {
		t.Errorf("ended by it: %+v", got)
	}
	if !strings.HasPrefix(got.Summary, `Dynamic workflow "x" failed: `) || strings.Contains(got.Summary, "abcdefghijklmnop") {
		t.Errorf("its summary: %q", got.Summary)
	}
	agents, _ := l.WorkflowAgents(id, folder, w)
	if len(agents) != 2 || agents[0].State != "done" || agents[1].State != "stopped" {
		t.Errorf("its agents: %+v", agents)
	}
}

// The queue's enqueue line is enough; and a run whose files never came
// (the session's folder not there) still ends by it.
func TestAWorkflowEndedByItsQueuedNotification(t *testing.T) {
	l, id, folder, session := fakeRun(t, "", "")
	l.Workflows.snapshotWait = 0
	testcheck.NoErr(t, os.MkdirAll(filepath.Dir(session), 0o700))
	writeTranscript(t, session, notification(t, "enqueue", "t1", "killed", "stopped"))
	got, err := l.ReadWorkflow(id, folder, model.Workflow{RunID: "wf_abc", TaskID: "t1"})
	testcheck.NoErr(t, err)
	if got.State != "stopped" || got.End != "killed" {
		t.Errorf("got %+v", got)
	}
}

// Claude sends the notification without waiting for the snapshot's
// write: an end the notification told waits snapshotWait for it, and the
// snapshot, there in time, gives the detail.
func TestANotificationWaitsForTheSnapshot(t *testing.T) {
	l, id, folder, session := fakeRun(t, launchedLine+startA+resultA, "")
	l.Workflows.snapshotWait = 1 << 62
	w := model.Workflow{RunID: "wf_abc", TaskID: "t1"}
	writeTranscript(t, session, notification(t, "user", "t1", "completed", "done"))
	if got, _ := l.ReadWorkflow(id, folder, w); got.State != "running" {
		t.Fatalf("in the wait: %+v", got)
	}
	snap := map[string]any{"status": "completed", "taskId": "t1", "phases": []any{map[string]any{"title": "Map", "detail": "the detail"}}}
	data, err := json.Marshal(snap)
	testcheck.NoErr(t, err)
	testcheck.Write(t, filepath.Join(session, "workflows", "wf_abc.json"), string(data))
	if got, _ := l.ReadWorkflow(id, folder, w); got.State != "done" || len(got.Phases) != 1 || got.Phases[0].Detail != "the detail" {
		t.Errorf("with the snapshot: %+v", got)
	}
}
