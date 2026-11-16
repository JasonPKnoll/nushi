package model

import (
	"slices"
	"time"
)

// BranchPush is a branch a tool call of a chat pushed: when it first did
// and when it last did.
type BranchPush struct {
	Branch      string
	First, Last time.Time
}

// MaxPushed is how many pushed branches a chat remembers (Pushed).
const MaxPushed = 20

// PushedBranch says a tool call of the chat pushed branch.
func (s ChatRecord) PushedBranch(branch string) bool {
	return branch != "" && slices.ContainsFunc(s.Pushed, func(p BranchPush) bool { return p.Branch == branch })
}

// Claim is how strongly the chat holds the pull request of branch opened
// at created: its latest push of the branch at or before then (only the
// first and the last are kept, so the first when the last came after),
// zero when it pushed only after, or never.
func (s ChatRecord) Claim(branch string, created time.Time) time.Time {
	i := slices.IndexFunc(s.Pushed, func(p BranchPush) bool { return p.Branch == branch })
	switch {
	case i < 0:
		return time.Time{}
	case !s.Pushed[i].Last.After(created):
		return s.Pushed[i].Last
	case !s.Pushed[i].First.After(created):
		return s.Pushed[i].First
	}
	return time.Time{}
}

// UnkeptArchiveGrace is how long after its last message or turn
// (Updated) a chat archived before ArchivedAt was kept is taken as still
// alive. Its archive time is unknown, and a common flow ends after the last
// turn: the user opens the pull request from their own terminal minutes
// or hours later and archives the chat once it merged. A day keeps those
// and still drops the ones opened long after.
const UnkeptArchiveGrace = 24 * time.Hour

// Lived says t is in the chat's life, as a pull request found by its
// branch must have been opened (ADR 0059's amendment of 2026-11-16): at
// or after the chat began (Since, else Created), and before it was
// archived (ArchivedAt; for one archived before that was kept, up to
// UnkeptArchiveGrace after its last message or turn, Updated). A zero t,
// not known, is taken as in it.
func (s ChatRecord) Lived(t time.Time) bool {
	if t.IsZero() {
		return true
	}
	began := s.Since
	if began.IsZero() {
		began = s.Created
	}
	if t.Before(began) {
		return false
	}
	if !s.Archived {
		return true
	}
	end := s.ArchivedAt
	if end.IsZero() && !s.Updated.IsZero() {
		end = s.Updated.Add(UnkeptArchiveGrace)
	}
	return end.IsZero() || !t.After(end)
}
