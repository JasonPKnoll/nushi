package chats

import (
	"context"
	"encoding/json"
	"errors"
	"io/fs"
	"slices"
	"sync"
	"testing"
	"time"

	acp "github.com/afrase/acp-go/unstable"

	"github.com/afrase/nushi/internal/model"
	"github.com/afrase/nushi/internal/testcheck"
)

// fakeWorkflows reads a run as the test says: read, by the run as the chat
// has it; it counts the reads and says each on reads.
type fakeWorkflows struct {
	mu     sync.Mutex
	read   func(w model.Workflow) (model.Workflow, error)
	n      int
	reads  chan int
	agents []model.WorkflowAgent
	items  []model.Item
}

func (f *fakeWorkflows) ReadWorkflow(_, _ string, w model.Workflow) (model.Workflow, error) {
	f.mu.Lock()
	f.n++
	n, read := f.n, f.read
	f.mu.Unlock()
	got, err := read(w)
	select {
	case f.reads <- n:
	default:
	}
	return got, err
}

func (f *fakeWorkflows) WorkflowAgents(string, string, model.Workflow) ([]model.WorkflowAgent, error) {
	return f.agents, nil
}

func (f *fakeWorkflows) WorkflowAgentItems(_, _ string, _ model.Workflow, id string, _ int) ([]model.Item, int, error) {
	if id == "" {
		return nil, 0, fs.ErrNotExist
	}
	return f.items, 0, nil
}

func (f *fakeWorkflows) set(read func(w model.Workflow) (model.Workflow, error)) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.read = read
}

// waitReads waits until the follower has read n more times.
func (f *fakeWorkflows) waitReads(t *testing.T, n int) {
	t.Helper()
	for range n {
		select {
		case <-f.reads:
		case <-time.After(5 * time.Second):
			t.Fatal("no read")
		}
	}
}

// workflowHarness is Claude's harness with a fake of its workflow files.
type workflowHarness struct {
	testHarness
	*fakeWorkflows
}

// running is a run as read while it goes, its files just written.
func runningAs(w model.Workflow) (model.Workflow, error) {
	w.State, w.Active, w.Agents, w.RunningCount, w.Running = "running", time.Now(), 2, 2, []string{"map:a", "map:b"}
	w.Phases = []model.WorkflowPhase{{Title: "Map", Started: 2}}
	return w, nil
}

func endedAs(state, end string) func(model.Workflow) (model.Workflow, error) {
	return func(w model.Workflow) (model.Workflow, error) {
		w.State, w.End, w.Ended, w.Agents, w.Done = state, end, time.Now(), 2, 2
		return w, nil
	}
}

// workflowFixture is a chat of Claude's whose agent launched a workflow.
type workflowFixture struct {
	svc    *Service
	agent  *fakeAgent
	c      *chat
	files  *fakeWorkflows
	events chan model.Event
}

const wfCall = "toolu_wf"

func workflowChat(t *testing.T, timing Timing, read func(model.Workflow) (model.Workflow, error)) workflowFixture {
	t.Helper()
	agent := &fakeAgent{}
	testcheck.NoErr(t, json.Unmarshal([]byte(claudeSession), &agent.session))
	files := &fakeWorkflows{read: read, reads: make(chan int, 1000)}
	timing.WorkflowPoll = cmpDuration(timing.WorkflowPoll, 5*time.Millisecond)
	events := make(chan model.Event, 10000)
	svc := newService(t, []Kind{{Harness: workflowHarness{testHarness{Name: "claude"}, files}, Launch: agent.launch}}, Options{Timing: timing,
		Publish: func(e model.Event) {
			select {
			case events <- e:
			default:
			}
		}})
	t.Cleanup(svc.Close)
	id, err := svc.Start(t.Context(), "", t.TempDir(), false)
	testcheck.NoErr(t, err)
	c := svc.chats[id]
	launch(c)
	return workflowFixture{svc, agent, c, files, events}
}

func cmpDuration(a, b time.Duration) time.Duration {
	if a != 0 {
		return a
	}
	return b
}

// launch sends the chat a Workflow call as Claude's agent does: the call,
// its toolResponse, then its result (docs/facts/claude.md).
func launch(c *chat) {
	text := []acp.ToolCallContent{{Type: "content", Content: &acp.Content{Content: acp.ContentBlock{Type: "text", Text: &acp.TextContent{
		Text: "Workflow launched in background. Task ID: w9u1xb6ik\nSummary: Ask four helpers\nRun ID: wf_769a004c-2aa\n"}}}}}
	resp := map[string]any{"claudeCode": map[string]any{"toolName": "Workflow", "toolResponse": map[string]any{"status": "async_launched",
		"taskId": "w9u1xb6ik", "taskType": "local_workflow", "workflowName": "two-phases", "runId": "wf_769a004c-2aa", "summary": "Ask four helpers"}}}
	for _, u := range []acp.SessionUpdate{
		{SessionUpdate: "tool_call", ToolCall: &acp.ToolCall{ToolCallID: wfCall, Title: "Workflow", Kind: "other", Status: "pending",
			Meta: map[string]any{"claudeCode": map[string]any{"toolName": "Workflow"}}}},
		{SessionUpdate: "tool_call_update", ToolCallUpdate: &acp.ToolCallUpdate{ToolCallID: wfCall, Meta: resp}},
		{SessionUpdate: "tool_call_update", ToolCallUpdate: &acp.ToolCallUpdate{ToolCallID: wfCall, Status: "completed", Content: text}},
		{SessionUpdate: "usage_update", UsageUpdate: &acp.UsageUpdate{Used: 1, Size: 2}}, // the turn's end: it came outside one of Nushi's
	} {
		_ = c.SessionUpdate(context.Background(), &acp.SessionNotification{SessionID: acp.SessionID(c.sid), Update: u})
	}
}

func workflowOf(c *chat) *model.Workflow {
	c.mu.Lock()
	defer c.mu.Unlock()
	it := c.itemLocked(wfCall)
	if it == nil || it.Workflow == nil {
		return nil
	}
	w := *it.Workflow
	return &w
}

// waitStatus waits for a status that holds.
func (f workflowFixture) waitStatus(t *testing.T, ok func(model.Status) bool) {
	t.Helper()
	for {
		select {
		case e := <-f.events:
			if e.Status != nil && ok(*e.Status) {
				return
			}
		case <-time.After(5 * time.Second):
			t.Fatal("no such status")
		}
	}
}

// A launched run is followed: running, it counts as the chat's background
// work and as a workflow; its snapshot ends it, and the follower with it.
func TestAWorkflowIsFollowedToItsEnd(t *testing.T) {
	f := workflowChat(t, Timing{}, runningAs)
	f.waitStatus(t, func(st model.Status) bool { return st.Workflows == 1 && st.Background == 1 })
	w := workflowOf(f.c)
	if w.State != "running" || w.RunID != "wf_769a004c-2aa" || w.TaskID != "w9u1xb6ik" || w.Name != "two-phases" || w.Started.IsZero() {
		t.Fatalf("running: %+v", w)
	}
	f.c.mu.Lock()
	quiet, task := f.c.quietLocked(), f.c.itemLocked(wfCall).TaskID
	f.c.mu.Unlock()
	if quiet || task != "w9u1xb6ik" {
		t.Errorf("quiet %v, task %q while it runs", quiet, task)
	}
	f.files.set(endedAs("done", "completed"))
	f.waitStatus(t, func(st model.Status) bool { return st.Workflows == 0 && st.Background == 0 })
	eventually(t, f.c, func() bool { return len(f.c.following) == 0 })
	if w := workflowOf(f.c); w.State != "done" || w.Ended.IsZero() {
		t.Errorf("ended: %+v", w)
	}
}

// Reads that change nothing publish nothing.
func TestAnUnchangedWorkflowSaysNothing(t *testing.T) {
	f := workflowChat(t, Timing{}, runningAs)
	f.waitStatus(t, func(st model.Status) bool { return st.Workflows == 1 })
	f.files.waitReads(t, 2)
	for len(f.events) > 0 {
		<-f.events
	}
	f.files.waitReads(t, 10)
	if events := len(f.events); events != 0 {
		t.Errorf("%d events from reads that changed nothing", events)
	}
}

// A run whose files never come shows as the plain call it was.
func TestAWorkflowNeverFoundIsAPlainCall(t *testing.T) {
	f := workflowChat(t, Timing{WorkflowFind: 30 * time.Millisecond}, func(model.Workflow) (model.Workflow, error) {
		return model.Workflow{}, fs.ErrNotExist
	})
	eventually(t, f.c, func() bool { return f.c.itemLocked(wfCall).Workflow == nil && len(f.c.following) == 0 })
	f.c.mu.Lock()
	defer f.c.mu.Unlock()
	if f.c.workflowsLocked() != 0 || f.c.itemLocked(wfCall).TaskID != "" {
		t.Error("still counted")
	}
}

// Files that come late, and reads that fail after they came, keep it
// running: a read error is no end.
func TestWorkflowReadsThatFail(t *testing.T) {
	var mu sync.Mutex
	n := 0
	f := workflowChat(t, Timing{}, func(w model.Workflow) (model.Workflow, error) {
		mu.Lock()
		defer mu.Unlock()
		n++
		switch {
		case n < 3:
			return w, fs.ErrNotExist
		case n < 6 || n%2 == 0:
			return runningAs(w)
		}
		return w, errors.New("permission denied")
	})
	f.files.waitReads(t, 12)
	if w := workflowOf(f.c); w == nil || w.State != "running" {
		t.Errorf("got %+v", w)
	}
}

// A run whose files, once found, can't be read again still ends: its
// agent gone, after the grace, stopped; its agent alive, once they stay
// gone for WorkflowFind, ended in Nushi's words. Its follower goes.
func TestAWorkflowWhoseFilesGo(t *testing.T) {
	for name, agentGoes := range map[string]bool{"its agent goes": true, "its agent stays": false} {
		t.Run(name, func(t *testing.T) {
			f := workflowChat(t, Timing{WorkflowGrace: time.Millisecond, WorkflowFind: 20 * time.Millisecond}, runningAs)
			f.waitStatus(t, func(st model.Status) bool { return st.Workflows == 1 })
			f.files.set(func(w model.Workflow) (model.Workflow, error) { return w, fs.ErrNotExist })
			f.c.mu.Lock()
			agent := f.c.agent
			if agentGoes {
				f.c.agent = nil // halted, as Delete does
			}
			f.c.pokeWorkflowsLocked()
			f.c.mu.Unlock()
			eventually(t, f.c, func() bool { return f.c.itemLocked(wfCall).Workflow.Final() && len(f.c.following) == 0 })
			w := workflowOf(f.c)
			switch {
			case agentGoes && (w.State != "stopped" || w.End != "Claude quit while it ran"):
				t.Errorf("its agent gone: %+v", w)
			case !agentGoes && (w.State != "unknown" || w.End != "its files are gone" || w.RunningCount != 0):
				t.Errorf("its files gone: %+v", w)
			}
			f.c.mu.Lock()
			f.c.agent = agent
			n := f.c.workflowsLocked()
			f.c.mu.Unlock()
			if n != 0 {
				t.Errorf("%d still counted", n)
			}
		})
	}
}

// Each end the snapshot says is the run's.
func TestWorkflowEnds(t *testing.T) {
	for _, state := range []string{"done", "failed", "stopped", "unknown"} {
		f := workflowChat(t, Timing{}, endedAs(state, "x"))
		eventually(t, f.c, func() bool { return f.c.itemLocked(wfCall).Workflow.State == state && len(f.c.following) == 0 })
	}
}

// The agent that ran it gone, a last read after the grace: its snapshot if
// it left one, else stopped, in Nushi's words.
func TestAWorkflowEndsWithItsAgent(t *testing.T) {
	for name, snapshot := range map[string]bool{"no snapshot": false, "a snapshot in the grace": true} {
		t.Run(name, func(t *testing.T) {
			f := workflowChat(t, Timing{WorkflowGrace: 20 * time.Millisecond}, runningAs)
			f.waitStatus(t, func(st model.Status) bool { return st.Workflows == 1 })
			f.c.mu.Lock()
			agent := f.c.agent
			f.c.agent = &fakeAgent{} // another process: the one that ran it went
			f.c.pokeWorkflowsLocked()
			f.c.mu.Unlock()
			f.files.waitReads(t, 1)
			if w := workflowOf(f.c); w.State != "running" {
				t.Errorf("in the grace: %+v", w)
			}
			if snapshot {
				f.files.set(endedAs("done", "completed"))
			}
			eventually(t, f.c, func() bool { return f.c.itemLocked(wfCall).Workflow.Final() })
			w := workflowOf(f.c)
			switch {
			case snapshot && w.State != "done":
				t.Errorf("with a snapshot: %+v", w)
			case !snapshot && (w.State != "stopped" || w.End != "Claude quit while it ran" || w.RunningCount != 0):
				t.Errorf("without one: %+v", w)
			}
			f.c.mu.Lock()
			f.c.agent = agent
			f.c.mu.Unlock()
		})
	}
}

// A run no agent of this daemon's runs (the chat reopened since) is
// followed while its files change, and ends stopped once they are still.
func TestAWorkflowAfterAReopen(t *testing.T) {
	var mu sync.Mutex
	active := time.Now()
	f := workflowChat(t, Timing{WorkflowQuiet: time.Hour}, func(w model.Workflow) (model.Workflow, error) {
		mu.Lock()
		defer mu.Unlock()
		w, _ = runningAs(w)
		w.Active = active
		return w, nil
	})
	f.waitStatus(t, func(st model.Status) bool { return st.Workflows == 1 })
	f.c.mu.Lock()
	f.c.following[wfCall].owner = nil // as followWorkflowsLocked follows it after a reopen
	f.c.mu.Unlock()
	f.files.waitReads(t, 5)
	if w := workflowOf(f.c); w.State != "running" {
		t.Fatalf("its files fresh: %+v", w)
	}
	mu.Lock()
	active = time.Now().Add(-2 * time.Hour)
	mu.Unlock()
	eventually(t, f.c, func() bool { return f.c.itemLocked(wfCall).Workflow.State == "stopped" })
}

// A run whose files stay still for an hour, its agent alive, stalls: no
// longer the chat's work, not ended; a write clears it, and the snapshot
// still ends it.
func TestAWorkflowStalls(t *testing.T) {
	var mu sync.Mutex
	active := time.Now().Add(-2 * time.Hour)
	f := workflowChat(t, Timing{WorkflowStalledPoll: 5 * time.Millisecond}, func(w model.Workflow) (model.Workflow, error) {
		mu.Lock()
		defer mu.Unlock()
		w, _ = runningAs(w)
		w.Active = active
		return w, nil
	})
	eventually(t, f.c, func() bool { return f.c.itemLocked(wfCall).Workflow.Stalled })
	f.c.mu.Lock()
	quiet, n := f.c.quietLocked(), f.c.workflowsLocked()
	st := f.c.statusLocked()
	f.c.mu.Unlock()
	if !quiet || n != 0 || st.Background != 0 || workflowOf(f.c).Final() {
		t.Errorf("stalled: quiet %v, %d running, %+v", quiet, n, st)
	}
	mu.Lock()
	active = time.Now()
	mu.Unlock()
	f.waitStatus(t, func(st model.Status) bool { return st.Workflows == 1 })
	if w := workflowOf(f.c); w.Stalled {
		t.Errorf("after a write: %+v", w)
	}
	f.files.set(endedAs("done", "completed"))
	eventually(t, f.c, func() bool { return f.c.itemLocked(wfCall).Workflow.State == "done" })
}

// A run that goes on and on, writing as it goes, keeps running: never
// stalled, never ended.
func TestALongWorkflowKeepsRunning(t *testing.T) {
	f := workflowChat(t, Timing{}, runningAs)
	f.files.waitReads(t, 50)
	if w := workflowOf(f.c); w.State != "running" || w.Stalled {
		t.Errorf("got %+v", w)
	}
}

// An own turn of the agent's starting (a workflow's notification) has the
// run read at once: its end shows without waiting for a poll.
func TestAnOwnTurnPokesTheWorkflow(t *testing.T) {
	f := workflowChat(t, Timing{WorkflowPoll: time.Hour}, runningAs)
	f.files.waitReads(t, 1)
	f.files.set(endedAs("done", "completed"))
	_ = f.c.SessionUpdate(context.Background(), &acp.SessionNotification{SessionID: acp.SessionID(f.c.sid), Update: acp.SessionUpdate{
		SessionUpdate: "agent_message_chunk", AgentMessageChunk: &acp.ContentChunk{Content: acp.ContentBlock{Type: "text", Text: &acp.TextContent{Text: "Done."}}}}})
	f.waitStatus(t, func(st model.Status) bool { return st.OwnTurn })
	eventually(t, f.c, func() bool { return f.c.itemLocked(wfCall).Workflow.State == "done" })
	_ = f.c.SessionUpdate(context.Background(), &acp.SessionNotification{SessionID: acp.SessionID(f.c.sid), Update: acp.SessionUpdate{
		SessionUpdate: "usage_update", UsageUpdate: &acp.UsageUpdate{Used: 1, Size: 2}}})
	f.waitStatus(t, func(st model.Status) bool { return !st.OwnTurn })
}

// A rewind past the call ends its follower.
func TestARewoundWorkflowIsLetGo(t *testing.T) {
	f := workflowChat(t, Timing{}, runningAs)
	f.waitStatus(t, func(st model.Status) bool { return st.Workflows == 1 })
	f.c.mu.Lock()
	f.c.items = slices.DeleteFunc(f.c.items, func(it model.Item) bool { return it.ID == wfCall })
	f.c.mu.Unlock()
	eventually(t, f.c, func() bool { return len(f.c.following) == 0 })
}

// Stop asks Claude to stop the run by its task's id, after the turn.
func TestStoppingAWorkflow(t *testing.T) {
	f := workflowChat(t, Timing{}, runningAs)
	f.waitStatus(t, func(st model.Status) bool { return st.Workflows == 1 })
	testcheck.NoErr(t, f.svc.StopTask(t.Context(), f.c.id, wfCall))
	eventually(t, f.c, func() bool { return !f.c.busy })
	if prompts, _ := f.agent.texts(); len(prompts) != 1 || prompts[0] != claudeHarness.StopTask("w9u1xb6ik") {
		t.Errorf("asked %q", prompts)
	}
}

// GetWorkflow's and GetWorkflowAgent's answers: the run, its agents, an
// agent's steps; an agent the run doesn't list is no agent.
func TestReadingAWorkflow(t *testing.T) {
	f := workflowChat(t, Timing{}, runningAs)
	f.files.agents = []model.WorkflowAgent{{ID: "a1", Label: "map:a", State: "running"}}
	f.files.items = []model.Item{{ID: "wf/a1/0-0", Kind: model.AgentMessage, Text: "alpha"}}
	f.waitStatus(t, func(st model.Status) bool { return st.Workflows == 1 })
	w, agents, more, err := f.svc.Workflow(f.c.id, wfCall)
	testcheck.NoErr(t, err)
	if w.State != "running" || len(agents) != 1 || more {
		t.Errorf("got %+v, %+v, %v", w, agents, more)
	}
	items, _, agent, err := f.svc.WorkflowAgent(f.c.id, wfCall, "a1", 400)
	testcheck.NoErr(t, err)
	if len(items) != 1 || agent.Label != "map:a" {
		t.Errorf("got %+v, %+v", items, agent)
	}
	if _, _, _, err := f.svc.WorkflowAgent(f.c.id, wfCall, "a2", 400); !errors.Is(err, ErrNoWorkflow) {
		t.Errorf("an agent not listed: %v", err)
	}
	if _, _, _, err := f.svc.Workflow(f.c.id, "nope"); !errors.Is(err, ErrNoWorkflow) {
		t.Errorf("no such call: %v", err)
	}
}

// An agent without workflow files (a plain ACP agent, or Claude's harness
// alone, as tests run it) follows nothing: the call stays plain.
func TestNoWorkflowWithoutItsFiles(t *testing.T) {
	c := &chat{kind: testKind(testHarness{}), ctx: t.Context(), publish: func(model.Event) {}}
	it := model.Item{ID: wfCall, Kind: model.ToolCall, Tool: "Workflow", Status: "completed", Text: "Workflow launched in background. Task ID: t1\nRun ID: wf_1\n"}
	c.mu.Lock()
	c.workflowLocked(&it)
	c.mu.Unlock()
	if it.Workflow != nil || len(c.following) != 0 {
		t.Errorf("followed: %+v", it.Workflow)
	}
}
