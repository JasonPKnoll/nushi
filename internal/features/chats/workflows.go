package chats

import (
	"errors"
	"fmt"
	"io/fs"
	"reflect"
	"time"

	"github.com/afrase/nushi/internal/model"
)

// A workflow (ADR 0127) is a run a tool call launched, which goes on past
// the call, its agents writing only to the agent's own files: Claude's
// Workflow. The call answers at once and the turn ends, so the chat
// follows the run from those files (WorkflowReader) and counts it as
// working while it runs. Nothing in what the agent sends says reliably that
// a run ended: Claude's snapshot of it does, and when that never comes,
// the agent's going does.

// follow is a workflow being followed: the agent that ran it (nil when
// the chat reopened since: no agent of this daemon's runs it), and a poke
// that has it read at once.
type follow struct {
	owner Agent
	poke  chan struct{}
	found bool      // its files were read once
	since time.Time // when following began
	gone  time.Time // when its owner was first seen gone
}

// workflowLocked notices a call that launched a workflow and follows it,
// once the call completed: its run (model.Workflow) as the harness read
// it from the call, or now from its output (a replay has no more).
func (c *chat) workflowLocked(it *model.Item) {
	r := c.kind.workflows
	if r == nil || it.Kind != model.ToolCall || it.Tool != "Workflow" || it.Status != "completed" || it.Parent != "" || c.quiet {
		return
	}
	if it.Workflow == nil {
		if it.Workflow = r.LaunchedWorkflow(it.Text); it.Workflow == nil {
			return
		}
		it.TaskID = it.Workflow.TaskID
	}
	if it.Workflow.Final() || c.following[it.ID] != nil {
		return
	}
	f := &follow{poke: make(chan struct{}, 1), since: time.Now()}
	if !c.reopeningLocked() && c.agent != nil && it.Workflow.State == "" {
		f.owner = c.agent
		w := *it.Workflow
		w.Started = time.Now()
		it.Workflow = &w
	}
	if c.following == nil {
		c.following = map[string]*follow{}
	}
	c.following[it.ID] = f
	go c.followWorkflow(it.ID, f)
}

// followWorkflowsLocked follows the workflows of the chat's items that
// may still run: after a reopen or a copy read back, none is followed.
func (c *chat) followWorkflowsLocked() {
	for i := range c.items {
		if it := &c.items[i]; it.Tool == "Workflow" && !it.Workflow.Final() {
			c.workflowLocked(it)
		}
	}
}

// pokeWorkflowsLocked has each workflow followed read its files now:
// something happened that may be its end (an own turn starting, as its
// notification does; the agent going; a task stopped).
func (c *chat) pokeWorkflowsLocked() {
	for _, f := range c.following {
		select {
		case f.poke <- struct{}{}:
		default:
		}
	}
}

// followWorkflow reads the run of call callID until it ended, the call is
// gone (a rewind) or the service closes.
func (c *chat) followWorkflow(callID string, f *follow) {
	defer func() {
		c.mu.Lock()
		if c.following[callID] == f {
			delete(c.following, callID)
		}
		c.mu.Unlock()
	}()
	for {
		c.mu.Lock()
		it := c.itemLocked(callID)
		if it == nil || it.Workflow == nil || c.ctx.Err() != nil {
			c.mu.Unlock()
			return
		}
		w, sid, folder, gone := *it.Workflow, c.sid, c.record.Folder, f.owner != nil && c.agent != f.owner
		c.mu.Unlock()
		got, err := c.kind.workflows.ReadWorkflow(sid, folder, w)
		c.mu.Lock()
		it = c.itemLocked(callID)
		if it == nil || it.Workflow == nil {
			c.mu.Unlock()
			return
		}
		next, wait := c.judgeWorkflow(*it.Workflow, got, err, f, gone, time.Now())
		c.setWorkflowLocked(it, next)
		final := next == nil || next.Final()
		c.mu.Unlock()
		if final {
			return
		}
		select {
		case <-time.After(wait):
		case <-f.poke:
		case <-c.ctx.Done():
			return
		}
	}
}

// judgeWorkflow is what a read of a run (got, err) makes of it (cur), and
// how long to wait for the next read; nil when it is no workflow to show
// after all (its files never came: the call shows as a plain one).
func (c *chat) judgeWorkflow(cur, got model.Workflow, err error, f *follow, gone bool, now time.Time) (*model.Workflow, time.Duration) {
	poll := c.timing.workflowPoll()
	switch {
	case err != nil && !f.found && now.Sub(f.since) >= c.timing.workflowFind():
		return nil, 0
	case err != nil && !f.found && !errors.Is(err, fs.ErrNotExist) && f.owner == nil:
		return nil, 0 // a past run whose files can't be read
	case err != nil: // not there yet, or a read that failed: the next one may not
		return &cur, poll
	}
	f.found = true
	if got.Final() {
		return &got, 0
	}
	got.Stalled = false
	switch {
	case f.owner != nil && gone && f.gone.IsZero():
		f.gone = now // a last read after a grace, for the snapshot it may have left
		return &got, c.timing.workflowGrace()
	case f.owner != nil && gone:
		return quit(got, now), 0
	case f.owner == nil && now.Sub(got.Active) >= c.timing.workflowQuiet():
		return quit(got, now), 0 // no agent of this daemon's runs it, and its files are still
	case f.owner != nil && now.Sub(got.Active) >= c.timing.workflowStall():
		got.Stalled = true
		return &got, c.timing.workflowStalledPoll()
	}
	return &got, poll
}

// quit is w ended with the agent that ran it: a run lives in the agent's
// process, and Claude itself calls a run it left so stopped.
func quit(w model.Workflow, now time.Time) *model.Workflow {
	w.State, w.End, w.Ended, w.Running, w.RunningCount = "stopped", "Claude quit while it ran", now, nil, 0
	return &w
}

// setWorkflowLocked sets the call's run and publishes it when it changed
// (its files' time aside, which changes with every write).
func (c *chat) setWorkflowLocked(it *model.Item, w *model.Workflow) {
	if w != nil && it.Workflow != nil {
		was, now := *it.Workflow, *w
		was.Active, now.Active = time.Time{}, time.Time{}
		if reflect.DeepEqual(was, now) {
			it.Workflow.Active = w.Active
			return
		}
	}
	it.Workflow = w
	if w == nil {
		it.TaskID = ""
	}
	c.publishLocked(*it)
	if w == nil || w.Final() {
		c.copyLocked(false)
	}
}

// stopWorkflow is the message that asks the agent to stop the
// workflow call started, by its task's id.
func stopWorkflow(call model.Item) *model.ToSubagent {
	if call.Workflow == nil || call.TaskID == "" {
		return nil
	}
	return &model.ToSubagent{Agent: "Workflow", AgentID: call.TaskID, Stop: true}
}

// ErrNoWorkflow says the chat has no such workflow, or the workflow no
// such agent, or the agent's files no longer have it.
var ErrNoWorkflow = errors.New("no such workflow or agent in this chat")

// maxWorkflowAgents is the most of a workflow's agents GetWorkflow lists.
const maxWorkflowAgents = 500

// Workflow returns the run tool call callID of chat chatID launched, and
// its agents (maxWorkflowAgents at most; more says there are others), as
// the agent's files say now.
func (s *Service) Workflow(chatID, callID string) (w model.Workflow, agents []model.WorkflowAgent, more bool, err error) {
	c, call, err := s.workflowCall(chatID, callID)
	if err != nil {
		return model.Workflow{}, nil, false, err
	}
	agents, err = c.kind.workflows.WorkflowAgents(c.sidFolder(call))
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return model.Workflow{}, nil, false, err
	}
	if len(agents) > maxWorkflowAgents {
		agents, more = agents[:maxWorkflowAgents], true
	}
	return *call.Workflow, agents, more, nil
}

// WorkflowAgent returns the steps of agent agentID of the run call callID
// launched, the last limit of them, how many came before, and the agent.
func (s *Service) WorkflowAgent(chatID, callID, agentID string, limit int) ([]model.Item, int, model.WorkflowAgent, error) {
	c, call, err := s.workflowCall(chatID, callID)
	if err != nil {
		return nil, 0, model.WorkflowAgent{}, err
	}
	sid, folder, w := c.sidFolder(call)
	agents, _ := c.kind.workflows.WorkflowAgents(sid, folder, w)
	var agent model.WorkflowAgent
	for _, a := range agents {
		if a.ID == agentID {
			agent = a
		}
	}
	if agent.ID == "" {
		return nil, 0, agent, fmt.Errorf("agent %q of that workflow: %w", agentID, ErrNoWorkflow)
	}
	items, earlier, err := c.kind.workflows.WorkflowAgentItems(sid, folder, w, agentID, limit)
	if errors.Is(err, fs.ErrNotExist) {
		err = fmt.Errorf("the agent's transcript: %w", ErrNoWorkflow)
	}
	return items, earlier, agent, err
}

// workflowCall is chat chatID's call callID that launched a workflow, as
// it stands.
func (s *Service) workflowCall(chatID, callID string) (*chat, model.Item, error) {
	c, err := s.find(chatID)
	if err != nil {
		return nil, model.Item{}, err
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	c.itemsBackLocked()
	it := c.itemLocked(callID)
	if it == nil || it.Workflow == nil || c.kind.workflows == nil {
		return nil, model.Item{}, fmt.Errorf("workflow %q: %w", callID, ErrNoWorkflow)
	}
	return c, *it, nil
}

// sidFolder is what the reader needs of the chat for call's run.
func (c *chat) sidFolder(call model.Item) (sid, folder string, w model.Workflow) {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.sid, c.record.Folder, *call.Workflow
}
