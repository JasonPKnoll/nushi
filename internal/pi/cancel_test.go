package pi

import (
	"context"
	"io"
	"testing"
	"time"

	acp "github.com/afrase/acp-go/unstable"

	"github.com/afrase/nushi/internal/pi/pitest"
)

// A session/cancel sent right after a session/prompt cancels it, however
// soon after: acp-go starts the prompt's handler first, but the cancel's
// runs beside it, before the turn may be registered or its prompt written
// to pi (acp.SequenceFromContext orders the two). Each round's "wait"
// would run until an abort pi never got.
func TestCancelRightAfterPrompt(t *testing.T) {
	store := &pitest.Store{Sessions: t.TempDir()}
	cancelRounds(t, store, 30, nil)
}

// A Stop that reaches pi before pi has taken the prompt it follows (pi
// handles each command as it reads it, and a prompt sets up before its
// run) still stops it: the abort waits for pi's answer to the prompt, so
// it finds the run. Before, pi answered the abort at once, the bridge let
// the session go, and the next round's prompt was refused as "already
// processing" while the stopped one ran on.
func TestCancelBeforePiTakesThePrompt(t *testing.T) {
	store := &pitest.Store{Sessions: t.TempDir(), PromptLag: 20 * time.Millisecond, Read: make(chan string)}
	cancelRounds(t, store, 5, func(ctx context.Context) { next(ctx, t, store.Read, "pi's read of the prompt") })
}

// cancelRounds runs rounds of a "wait" prompt and a Stop sent right
// after it (once read says the moment has come, if given), each answered
// cancelled, then a prompt that runs.
func cancelRounds(t *testing.T, store *pitest.Store, rounds int, read func(context.Context)) {
	t.Helper()
	c := start(t, store, io.Discard)
	s := openNew(t, c, t.TempDir())
	for i := range rounds {
		ctx, cancel := context.WithTimeout(t.Context(), turnTimeout)
		w, err := c.StartPrompt(ctx, &acp.PromptRequest{SessionID: s.SessionID, Prompt: []acp.ContentBlock{{Type: "text", Text: &acp.TextContent{Text: "wait"}}}})
		if err != nil {
			t.Fatal(err)
		}
		if read != nil {
			read(ctx)
		}
		if err := c.Cancel(context.Background(), &acp.CancelNotification{SessionID: s.SessionID}); err != nil {
			t.Fatal(err)
		}
		r, err := w(ctx)
		cancel()
		if err != nil || r.StopReason != "cancelled" {
			t.Fatalf("round %d: %+v, %v; want cancelled", i, r, err)
		}
	}
	if read != nil {
		go func() { <-store.Read }() // the last prompt's
	}
	if r := prompt(t, c, s.SessionID, "hello"); r.StopReason != "end_turn" {
		t.Errorf("the next prompt: %+v", r)
	}
}

// A Stop that finds nothing to stop (its prompt's turn ended, or hasn't
// begun: nextTurn cancels that) sends pi nothing. It used to send an
// abort, from a goroutine nothing ordered with the next prompt: one that
// went late stopped a later prompt's run, which is how
// TestCancelRightAfterPrompt's last prompt once came back cancelled.
func TestStopWithNothingToStop(t *testing.T) {
	store := &pitest.Store{Sessions: t.TempDir(), Waiting: make(chan string, 1)}
	held := holdAborts(t)
	c := start(t, store, io.Discard)
	s := openNew(t, c, t.TempDir())
	prompt(t, c, s.SessionID, "hello")
	if err := c.Cancel(context.Background(), &acp.CancelNotification{SessionID: s.SessionID}); err != nil {
		t.Fatal(err)
	}
	wait := begin(t, c, store, s.SessionID, "wait")
	if g, ok := within(held); ok {
		close(g)
		t.Error("a Stop with nothing to stop sent pi an abort")
	}
	if ok, err := c.Steer(t.Context(), s.SessionID, []acp.ContentBlock{{Type: "text", Text: &acp.TextContent{Text: "go on"}}}); !ok || err != nil {
		t.Fatalf("steer = %v, %v", ok, err)
	}
	if r := wait(); r.StopReason != "end_turn" {
		t.Errorf("the next prompt: %+v", r)
	}
}

// A Stop's abort goes to pi before the next prompt, also when its turn
// ends before the abort goes: here an extension's command, stopped while
// its dialog was open, is answered once the dialog is. Pi takes an abort
// that comes after a prompt as that prompt's Stop.
func TestStopsAbortBeforeTheNextPrompt(t *testing.T) {
	store := &pitest.Store{Sessions: t.TempDir()}
	held := holdAborts(t)
	c := start(t, store, io.Discard)
	c.asking = make(chan chan struct{})
	s := openNew(t, c, t.TempDir())
	ctx, cancel := context.WithTimeout(t.Context(), turnTimeout)
	defer cancel()
	send := func(text string) <-chan *acp.PromptResponse {
		w, err := c.StartPrompt(ctx, &acp.PromptRequest{SessionID: s.SessionID, Prompt: []acp.ContentBlock{{Type: "text", Text: &acp.TextContent{Text: text}}}})
		if err != nil {
			t.Fatal(err)
		}
		done := make(chan *acp.PromptResponse, 1)
		go func() {
			r, err := w(ctx)
			if err != nil {
				t.Errorf("%s: %v", text, err)
			}
			done <- r
		}()
		return done
	}
	asked := send("/ask")
	question := next(ctx, t, c.asking, "the question")
	if err := c.Cancel(ctx, &acp.CancelNotification{SessionID: s.SessionID}); err != nil {
		t.Fatal(err)
	}
	if r := next(ctx, t, asked, "the stopped command's answer"); r.StopReason != "cancelled" {
		t.Fatalf("the stopped command: %+v", r)
	}
	close(question) // pi answers the command now, which ends its turn
	abort := next(ctx, t, held, "the Stop's abort")
	hello := send("hello")
	r, early := within(hello)
	if early {
		t.Errorf("the next prompt ran (%+v) before the Stop's abort went to pi", r)
	}
	close(abort)
	if !early {
		r = next(ctx, t, hello, "the next prompt's answer")
	}
	if r.StopReason != "end_turn" {
		t.Errorf("the next prompt: %+v", r)
	}
}

// holdAborts holds each Stop's abort as it is about to go to pi, and
// hands the test a channel that lets it go.
func holdAborts(t *testing.T) <-chan chan struct{} {
	t.Helper()
	held := make(chan chan struct{}, 8)
	hold := func() {
		g := make(chan struct{})
		held <- g
		<-g
	}
	beforeAbort.Store(&hold)
	t.Cleanup(func() { beforeAbort.Store(nil) })
	return held
}

// within is what ch yields within a moment, for what must not happen:
// it can only be waited for so long. That never fails the code that
// holds; it only may miss a break.
func within[T any](ch <-chan T) (T, bool) {
	select {
	case v := <-ch:
		return v, true
	case <-time.After(100 * time.Millisecond):
		var zero T
		return zero, false
	}
}
