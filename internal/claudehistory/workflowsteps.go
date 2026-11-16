package claudehistory

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/afrase/claude-acp-go/claudeagent/sessions"

	"github.com/afrase/nushi/internal/model"
	"github.com/afrase/nushi/internal/secrets"
)

// stepCap is the most of a step's text or input kept: the pane shows a
// workflow agent's steps, not its whole output.
const stepCap = 8 << 10

// stepItems are a workflow agent's messages as transcript items, its steps
// as its pane shows them: the model's text and thinking, and each tool call
// with its input and result. Its prompt, the first user message, shows in
// the pane's head, so user text isn't a step. This is a plain reading for
// the pane only; a chat's own transcript comes from the agent's replay.
func stepItems(prefix string, msgs []sessions.Message) []model.Item {
	var out []model.Item
	calls := map[string]int{}
	for mi, m := range msgs {
		var body struct {
			ID      string          `json:"id"`
			Content json.RawMessage `json:"content"`
		}
		if json.Unmarshal(m.Message, &body) != nil {
			continue
		}
		var blocks []struct {
			Type      string          `json:"type"`
			Text      string          `json:"text"`
			Thinking  string          `json:"thinking"`
			ID        string          `json:"id"`
			Name      string          `json:"name"`
			Input     json.RawMessage `json:"input"`
			ToolUseID string          `json:"tool_use_id"`
			Content   json.RawMessage `json:"content"`
			IsError   bool            `json:"is_error"`
		}
		if json.Unmarshal(body.Content, &blocks) != nil {
			continue // a string: the user's text
		}
		sent := sentAt(m.Raw)
		for bi, b := range blocks {
			id := fmt.Sprintf("%s%d-%d", prefix, mi, bi)
			switch {
			case m.Type == "assistant" && b.Type == "text" && strings.TrimSpace(b.Text) != "":
				out = append(out, model.Item{ID: id, Kind: model.AgentMessage, Text: clipText(b.Text, stepCap), Sent: sent})
			case m.Type == "assistant" && b.Type == "thinking" && strings.TrimSpace(b.Thinking) != "":
				out = append(out, model.Item{ID: id, Kind: model.Thought, Text: clipText(b.Thinking, stepCap), Sent: sent})
			case m.Type == "assistant" && b.Type == "tool_use":
				calls[b.ID] = len(out)
				out = append(out, model.Item{ID: prefix + "call-" + b.ID, Kind: model.ToolCall, Tool: b.Name, Title: toolTitle(b.Name, b.Input),
					ToolKind: toolKind(b.Name), Status: "in_progress", Input: inputText(b.Input), Sent: sent})
			case m.Type == "user" && b.Type == "tool_result":
				if i, ok := calls[b.ToolUseID]; ok {
					out[i].Status, out[i].Text = "completed", clipText(resultText(b.Content), stepCap)
					if b.IsError {
						out[i].Status = "failed"
					}
				}
			}
		}
	}
	return out
}

func sentAt(raw json.RawMessage) time.Time {
	var r struct {
		Timestamp time.Time `json:"timestamp"`
	}
	_ = json.Unmarshal(raw, &r)
	return r.Timestamp
}

// toolTitle is a call's title as a row shows it: the tool and what it
// works on, when its input says.
func toolTitle(name string, input json.RawMessage) string {
	var in map[string]any
	_ = json.Unmarshal(input, &in)
	for _, k := range []string{"description", "command", "file_path", "path", "pattern", "url", "query", "prompt"} {
		if s, ok := in[k].(string); ok && strings.TrimSpace(s) != "" {
			first, _, _ := strings.Cut(strings.TrimSpace(s), "\n")
			return name + " " + label(first, "")
		}
	}
	return name
}

func toolKind(name string) string {
	switch name {
	case "Read", "NotebookRead":
		return "read"
	case "Edit", "Write", "MultiEdit", "NotebookEdit":
		return "edit"
	case "Bash":
		return "execute"
	case "Grep", "Glob", "ToolSearch":
		return "search"
	case "WebFetch", "WebSearch":
		return "fetch"
	}
	return "other"
}

func inputText(input json.RawMessage) string {
	var b bytes.Buffer
	if json.Indent(&b, input, "", "  ") != nil {
		return ""
	}
	return clipText(secrets.Mask(b.String(), "…", nil), stepCap)
}

// resultText is a tool result's text: a string, or its text blocks.
func resultText(raw json.RawMessage) string {
	if s := str(raw); s != "" {
		return s
	}
	var blocks []struct {
		Type, Text string
	}
	_ = json.Unmarshal(raw, &blocks)
	var parts []string
	for _, b := range blocks {
		if b.Type == "text" {
			parts = append(parts, b.Text)
		}
	}
	return strings.Join(parts, "\n")
}
