package chats

import (
	"testing"

	"github.com/afrase/nushi/internal/model"
)

// Close waits for what a request left running: here a reopen's deleting of
// the chat's thrown-away sessions, which saves the chat's record once done.
// Nothing of it may still run (and write) once Close has returned, as the
// daemon's data folder may go then; Close cancels it instead, and the
// record keeps the session for the next open.
func TestCloseWaitsForDiscarding(t *testing.T) {
	agent := &fakeAgent{deletes: true, deleteGate: make(chan struct{}), entered: make(chan string, 16)}
	t.Cleanup(func() { close(agent.deleteGate) }) // frees a delete Close left running
	sessions := &memSessions{stored: []model.ChatRecord{{ID: "c0", SessionID: "s0", Agent: "a", Folder: t.TempDir(), Throwaway: []string{"fork"}}}}
	svc := newService(t, []Kind{{Harness: model.HarnessInfo{Name: "a"}, Launch: agent.launch}}, Options{Sessions: sessions})
	if _, err := svc.Get("c0"); err != nil { // reopens the chat, which discards its throwaway
		t.Fatal(err)
	}
	for gate := range agent.entered {
		if gate == "delete" {
			break
		}
	}
	svc.Close()
	agent.mu.Lock()
	defer agent.mu.Unlock()
	if agent.gatedDeletes != 0 {
		t.Errorf("Close returned with %d deletes still running", agent.gatedDeletes)
	}
	if len(agent.deleted) != 0 {
		t.Errorf("deleted %v: Close cancels the discarding, and the record keeps them", agent.deleted)
	}
}
