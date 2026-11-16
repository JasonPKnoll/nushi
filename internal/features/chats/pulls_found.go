package chats

import (
	"slices"
	"time"

	"github.com/afrase/nushi/internal/model"
)

// A pull request found by its branch's head is the chat's own only when
// the chat pushed the branch and it was opened in the chat's life
// (ChatRecord.Lived): the owner, 2026-11-16, "Let's make sure the PR link
// is attached to that specific chat better" (ADR 0059's amendment). Of
// two chats that pushed the branch, the one whose push came last before it
// was opened takes it. An archived chat finds nothing new.

// FoundPull takes what the chat's folder is on: its branch, the pull
// request whose head that is (nil for none) and when that was opened
// (zero when not known). It's linked when the chat may have it
// (mayFindLocked); the ones found for another branch go, and so does that
// one when opened outside the chat's life (one opened in the chat or
// linked by hand stays).
func (s *Service) FoundPull(chatID, branch string, pull *model.ChatGitHub, created time.Time) error {
	c, err := s.listed(chatID)
	if err != nil {
		return err
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	r := c.record
	if r.Archived {
		return nil
	}
	linked := r.LinkedPulls()
	kept := slices.DeleteFunc(slices.Clone(linked), func(p model.ChatGitHub) bool {
		return p.Branch != "" && (p.Branch != branch || pull != nil && p.Same(*pull) && !r.Lived(created))
	})
	if len(kept) != len(linked) {
		c.saveLocked(func(s *model.ChatRecord) { setPulls(s, kept) })
	}
	if pull == nil || pull.Branch != branch || !s.mayFindLocked(c, branch, *pull, created) {
		return nil
	}
	p := *pull
	p.Opened = false
	return c.addPullLocked(p, false)
}

// mayFindLocked says the chat may link pull, found by branch and opened at
// created: one it has already, or one opened in its life on a branch it
// pushed, unless another chat of the same repository that pushed the
// branch in its own life did so later before created.
func (s *Service) mayFindLocked(c *chat, branch string, pull model.ChatGitHub, created time.Time) bool {
	r := c.record
	if !r.PushedBranch(branch) || !r.Lived(created) {
		return false
	}
	if pullIndex(r.LinkedPulls(), pull) >= 0 {
		return true // linked before: kept, and brought up to date
	}
	others, err := s.sessions.Sessions()
	if err != nil {
		return true
	}
	mine := r.Claim(branch, created)
	repo := repoOf(r)
	for _, o := range others {
		if o.ID != r.ID && repoOf(o) == repo && o.PushedBranch(branch) && o.Lived(created) && o.Claim(branch, created).After(mine) {
			c.logLocked().Info("pull request left to the chat that pushed last", "number", pull.Number, "chat", o.ID)
			return false
		}
	}
	return true
}

// repoOf is the repository a chat's folder belongs to, as far as its
// record says: its worktree's repository, else its folder.
func repoOf(r model.ChatRecord) string {
	if r.Worktree != nil && r.Worktree.Repo != "" {
		return r.Worktree.Repo
	}
	return r.Folder
}

// DropStrayPulls drops the chat's pull requests found by their branch's
// head that items (as GitHub said, with when each was opened) show were
// opened outside the chat's life: before it began, or after it was
// archived. They aren't remembered as unlinked, so linking one by hand
// still works.
func (s *Service) DropStrayPulls(chatID string, items []model.GitHubItem) {
	c, err := s.listed(chatID)
	if err != nil {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	r := c.record
	linked := r.LinkedPulls()
	kept := slices.DeleteFunc(slices.Clone(linked), func(p model.ChatGitHub) bool {
		if p.Branch == "" || p.Opened {
			return false
		}
		i := slices.IndexFunc(items, func(it model.GitHubItem) bool {
			return it.Pull && p.Same(model.ChatGitHub{Host: it.Host, Repo: it.Repo, Number: it.Number, ID: it.ID})
		})
		if i < 0 || r.Lived(items[i].Created) {
			return false
		}
		c.logLocked().Info("found pull request dropped: opened outside the chat's life", "number", p.Number)
		return true
	})
	if len(kept) != len(linked) {
		c.saveLocked(func(s *model.ChatRecord) { setPulls(s, kept) })
	}
}
