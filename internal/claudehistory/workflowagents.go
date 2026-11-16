package claudehistory

import (
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"

	"github.com/afrase/claude-acp-go/claudeagent/sessions"

	"github.com/afrase/nushi/internal/model"
)

// WorkflowAgents lists the agents of the run w names, in the order they
// started: the snapshot's rows once the run ended, the journal's while it
// runs (as last read by ReadWorkflow, or now when nothing was).
func (l Local) WorkflowAgents(sid, folder string, w model.Workflow) ([]model.WorkflowAgent, error) {
	var agents []model.WorkflowAgent
	err := l.readRun(sid, folder, w, func(r *run, _ *os.Root) error {
		agents = r.agents()
		return nil
	})
	return agents, err
}

func (r *run) agents() []model.WorkflowAgent {
	var out []model.WorkflowAgent
	if s := r.snap; s != nil && len(s.Agents) > 0 {
		for _, a := range s.Agents {
			wa := model.WorkflowAgent{ID: a.ID, Label: label(a.Label, a.ID), Phase: label(a.Phase, ""), State: agentState(a.State),
				Model: clipText(a.Model, 100), Prompt: clipText(a.Prompt, 2048), Result: clipText(a.Result, 2048),
				Started: a.Started, Tokens: a.Tokens, ToolCalls: a.ToolCalls}
			if !a.Started.IsZero() && a.Duration > 0 {
				wa.Ended = a.Started.Add(a.Duration)
			}
			out = append(out, wa)
		}
		return out
	}
	for _, row := range r.j.rows {
		state := row.State
		if (r.snap != nil || r.noted() != nil) && state == "running" {
			state = "stopped" // the run ended without it
		}
		out = append(out, model.WorkflowAgent{ID: row.AgentID, Label: label(row.Label, row.AgentID), Phase: label(row.Phase, ""), State: state})
	}
	return out
}

// WorkflowAgentItems reads the transcript of agent id of the run w names
// as transcript items, the last limit of them, and how many came before
// those. Only an agent the run's journal or snapshot lists is read, so an
// id never names a file of its own; one it doesn't is fs.ErrNotExist.
func (l Local) WorkflowAgentItems(sid, folder string, w model.Workflow, id string, limit int) ([]model.Item, int, error) {
	var items []model.Item
	err := l.readRun(sid, folder, w, func(r *run, root *os.Root) error {
		listed := false
		for _, a := range r.agents() {
			listed = listed || a.ID == id && id != ""
		}
		if !listed || !agentID.MatchString(id) || root == nil {
			return fmt.Errorf("agent %q of workflow %s: %w", id, w.RunID, fs.ErrNotExist)
		}
		fi, err := root.Lstat(path.Join("subagents", "workflows", w.RunID, "agent-"+id+".jsonl"))
		if err != nil {
			return err
		}
		if !fi.Mode().IsRegular() {
			return fmt.Errorf("agent %q of workflow %s: %w", id, w.RunID, fs.ErrNotExist)
		}
		s, ok := r.steps[id]
		if !ok || s.size != fi.Size() || !s.mod.Equal(fi.ModTime()) {
			msgs, err := sessions.Store{ConfigDir: filepath.Dir(l.Projects)}.SubagentMessages(sid, folder, id)
			if err != nil {
				return err
			}
			s = steps{size: fi.Size(), mod: fi.ModTime(), items: stepItems("wf/"+id+"/", msgs)}
			if r.steps == nil {
				r.steps = map[string]steps{}
			}
			r.steps[id] = s
		}
		items = s.items
		return nil
	})
	if err != nil {
		return nil, 0, err
	}
	earlier := 0
	if limit > 0 && len(items) > limit {
		earlier = len(items) - limit
		items = items[earlier:]
	}
	return append([]model.Item(nil), items...), earlier, nil
}
