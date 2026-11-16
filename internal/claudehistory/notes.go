package claudehistory

import (
	"bytes"
	"encoding/json"
	"os"
	"path"
	"regexp"
	"strings"
	"time"

	"github.com/afrase/nushi/internal/regularfile"
)

// When a run ends, Claude queues a <task-notification> naming its task and
// how it ended (`<status>`), and writes it to the session's transcript: a
// queue-operation enqueue line as it is queued, then a user line whose
// origin.kind is "task-notification" as it starts Claude's own turn
// (docs/facts/claude.md). That is Claude's end of a run even when it wrote
// no snapshot: its snapshot writer only logs a failed write, and a run whose
// runner throws sends a failed notification without one. So the reader
// takes the notification of the run's task as its end, and the snapshot,
// when there, for its detail.
const (
	maxNoteLine = 4 << 20  // a transcript line longer is passed over
	noteTail    = 16 << 20 // how far back the first read of a transcript starts
)

// snapshotWait is how long a run ended by its notification waits for its
// snapshot before it shows ended without one (Runs.snapshotWait in tests):
// Claude starts writing the snapshot and sends the notification without
// waiting for the write.
const snapshotWait = 3 * time.Second

// notes reads the session's transcript as it grows, for the notification
// of one task.
type notes struct {
	lines
	task string
	end  *note
}

// note is how a task-notification says the run ended: Claude's status
// word, its summary, and when it was written and first read.
type note struct {
	status, summary string
	at, seen        time.Time
}

// endWords are the statuses a notification ends a run with, as Claude
// writes them (completed, failed, killed) or names a stopped task
// elsewhere; any other is no end Nushi can trust.
var endWords = map[string]bool{"completed": true, "failed": true, "killed": true, "stopped": true, "cancelled": true}

var (
	noteBlock = regexp.MustCompile(`(?s)<task-notification>(.*?)(?:</task-notification>|$)`)
	noteTag   = func(tag string) *regexp.Regexp { return regexp.MustCompile(`(?s)<` + tag + `>(.*?)</` + tag + `>`) }
	taskTag   = noteTag("task-id")
	statusTag = noteTag("status")
	sumTag    = noteTag("summary")
)

// readNotes reads what session sid's transcript (in the history folder
// dir) added since the last read, for task's notification. Its first read
// starts noteTail from the end: a run's end comes after its launch.
func (r *run) readNotes(dir, sid, task string) {
	if task == "" || r.notes.end != nil {
		return
	}
	root, err := os.OpenRoot(dir)
	if err != nil {
		return
	}
	defer root.Close() //nolint:errcheck // read-only
	f, fi, err := regularfile.OpenIn(root, path.Clean(sid+".jsonl"), false)
	if err != nil {
		return
	}
	defer f.Close() //nolint:errcheck // read-only
	n := &r.notes
	if n.task != task || fi.Size() < n.off {
		*n = notes{task: task}
		if start := fi.Size() - noteTail; start > 0 {
			n.off, n.skipping = start, true // from the next line on
		}
	}
	_ = n.read(f, fi.Size(), maxNoteLine, func(line []byte) {
		if n.end == nil {
			n.end = noteOf(line, task)
		}
	})
	if n.end != nil {
		n.end.seen = time.Now()
	}
}

// noteOf is the end line says of task, when it is task's notification.
func noteOf(line []byte, task string) *note {
	if !bytes.Contains(line, []byte("task-notification")) || !bytes.Contains(line, []byte(task)) {
		return nil
	}
	var e struct {
		Type      string          `json:"type"`
		Operation string          `json:"operation"`
		Content   json.RawMessage `json:"content"`
		Timestamp string          `json:"timestamp"`
		Origin    struct {
			Kind string `json:"kind"`
		} `json:"origin"`
		Message struct {
			Content json.RawMessage `json:"content"`
		} `json:"message"`
	}
	if json.Unmarshal(line, &e) != nil {
		return nil
	}
	var text string
	switch {
	case e.Type == "queue-operation" && e.Operation == "enqueue":
		text = str(e.Content)
	case e.Type == "user" && e.Origin.Kind == "task-notification":
		text = str(e.Message.Content)
		if text == "" {
			text = resultText(e.Message.Content)
		}
	default:
		return nil // the model's text, or a tool's output, says nothing of an end
	}
	at, _ := time.Parse(time.RFC3339Nano, e.Timestamp)
	for _, m := range noteBlock.FindAllStringSubmatch(text, -1) {
		if tag(taskTag, m[1]) != task {
			continue
		}
		if status := tag(statusTag, m[1]); endWords[status] {
			return &note{status: status, summary: tag(sumTag, m[1]), at: at}
		}
	}
	return nil
}

func tag(re *regexp.Regexp, s string) string {
	if m := re.FindStringSubmatch(s); m != nil {
		return strings.TrimSpace(m[1])
	}
	return ""
}
