package git

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/afrase/nushi/internal/model"
	"github.com/afrase/nushi/internal/testcheck"
)

// TestChangesOfOnePath is a file preview's question (Span.Path): only that
// file, with HEAD's text, and an answer that tells "unchanged" apart from
// "nothing to compare".
func TestChangesOfOnePath(t *testing.T) {
	ctx := t.Context()
	repo := t.TempDir()
	sub := filepath.Join(repo, "sub")
	one := func(folder, path string) []model.FileChange {
		t.Helper()
		got, err := Changes(ctx, folder, Span{Path: path})
		if err != nil {
			t.Fatalf("%s: %v", path, err)
		}
		return got
	}

	if _, err := Changes(ctx, repo, Span{Path: "a.txt"}); err == nil {
		t.Error("a folder outside git answered")
	}
	testcheck.Git(t, repo, "init", "-q", "-b", "main")
	write(t, filepath.Join(sub, "a.txt"), "one\n")
	if got := one(sub, "a.txt"); len(got) != 1 || got[0].Status != "added" || got[0].Skipped != "" {
		t.Errorf("before the first commit: %+v", got)
	}

	write(t, filepath.Join(repo, ".gitignore"), "*.log\n")
	write(t, filepath.Join(sub, "b.txt"), "b\n")
	write(t, filepath.Join(sub, "crlf.txt"), "x\r\ny\r\n")
	write(t, filepath.Join(sub, "old.txt"), "keep\n")
	write(t, filepath.Join(sub, "bin"), "a\x00b")
	write(t, filepath.Join(sub, "big.txt"), "x\n")
	testcheck.NoErr(t, os.Symlink("a.txt", filepath.Join(sub, "tracked-link")))
	testcheck.Git(t, repo, "add", ".")
	testcheck.Git(t, repo, "commit", "-q", "-m", "init")

	write(t, filepath.Join(sub, "a.txt"), "one\ntwo\n")
	write(t, filepath.Join(sub, "b.txt"), "b changed\n") // another change, left out
	want := []model.FileChange{{Path: "a.txt", Status: "modified", Old: "one\n", New: "one\ntwo\n", Added: 1}}
	if got := one(sub, "a.txt"); !reflect.DeepEqual(got, want) {
		t.Errorf("modified =\n%+v\nwant\n%+v", got, want)
	}
	if got := one(repo, "sub/a.txt"); len(got) != 1 || got[0].Path != "sub/a.txt" || got[0].Old != "one\n" {
		t.Errorf("from the top: %+v", got)
	}
	if got := one(sub, "old.txt"); len(got) != 0 {
		t.Errorf("unchanged: %+v", got)
	}

	write(t, filepath.Join(sub, "crlf.txt"), "x\r\nz\r\n")
	if got := one(sub, "crlf.txt"); len(got) != 1 || got[0].Old != "x\r\ny\r\n" {
		t.Errorf("CRLF: %+v", got)
	}

	write(t, filepath.Join(sub, "new.txt"), "new\n")
	want = []model.FileChange{{Path: "new.txt", Status: "untracked", New: "new\n", Added: 1}}
	if got := one(sub, "new.txt"); !reflect.DeepEqual(got, want) {
		t.Errorf("untracked: %+v", got)
	}
	write(t, filepath.Join(sub, "x.log"), "ignored\n")
	want = []model.FileChange{{Path: "x.log", Status: "untracked", Skipped: "not tracked"}}
	if got := one(sub, "x.log"); !reflect.DeepEqual(got, want) {
		t.Errorf("ignored: %+v", got)
	}

	testcheck.NoErr(t, os.Symlink("b.txt", filepath.Join(sub, "new-link")))
	if got := one(sub, "new-link"); len(got) != 1 || got[0].Skipped != "a link" {
		t.Errorf("an untracked link: %+v", got)
	}
	if got := one(sub, "tracked-link"); len(got) != 1 || got[0].Skipped != "a link" {
		t.Errorf("a tracked, unchanged link: %+v", got)
	}
	testcheck.NoErr(t, os.Remove(filepath.Join(sub, "tracked-link")))
	testcheck.NoErr(t, os.Symlink("b.txt", filepath.Join(sub, "tracked-link")))
	if got := one(sub, "tracked-link"); len(got) != 1 || got[0].Skipped != "a link" {
		t.Errorf("a changed link: %+v", got)
	}

	write(t, filepath.Join(sub, "bin"), "c\x00d")
	if got := one(sub, "bin"); len(got) != 1 || got[0].Skipped != "binary" {
		t.Errorf("binary: %+v", got)
	}
	write(t, filepath.Join(sub, "big.txt"), strings.Repeat("x\n", maxText))
	if got := one(sub, "big.txt"); len(got) != 1 || got[0].Skipped != "too large" {
		t.Errorf("too large: %+v", got)
	}

	testcheck.NoErr(t, os.Remove(filepath.Join(sub, "b.txt")))
	if got := one(sub, "b.txt"); len(got) != 1 || got[0].Status != "deleted" || got[0].Old != "b\n" {
		t.Errorf("deleted: %+v", got)
	}

	testcheck.Git(t, repo, "mv", "sub/old.txt", "sub/moved.txt")
	write(t, filepath.Join(sub, "moved.txt"), "keep\nmore\n")
	want = []model.FileChange{{Path: "moved.txt", OldPath: "old.txt", Status: "renamed", Old: "keep\n", New: "keep\nmore\n", Added: 1}}
	if got := one(sub, "moved.txt"); !reflect.DeepEqual(got, want) {
		t.Errorf("renamed (no staged part asked) =\n%+v\nwant\n%+v", got, want)
	}

	if got := one(sub, "missing.txt"); len(got) != 1 || got[0].Skipped != "not tracked" {
		t.Errorf("a file that isn't there: %+v", got)
	}
	for _, p := range []string{"../top.txt", "/etc/hosts"} {
		if _, err := Changes(ctx, sub, Span{Path: p}); err == nil {
			t.Errorf("%s: a path outside the folder was read", p)
		}
	}
}

// TestChangesOfOnePathAutocrlf: a file git converts on checkout isn't
// listed as changed, so it reads as unchanged rather than every line.
func TestChangesOfOnePathAutocrlf(t *testing.T) {
	ctx := t.Context()
	repo := t.TempDir()
	testcheck.Git(t, repo, "init", "-q", "-b", "main")
	testcheck.Git(t, repo, "config", "core.autocrlf", "true")
	write(t, filepath.Join(repo, "a.txt"), "x\ny\n")
	testcheck.Git(t, repo, "add", ".")
	testcheck.Git(t, repo, "commit", "-q", "-m", "init")
	write(t, filepath.Join(repo, "a.txt"), "x\r\ny\r\n") // as a checkout writes it
	if got, err := Changes(ctx, repo, Span{Path: "a.txt"}); err != nil || len(got) != 0 {
		t.Errorf("autocrlf: %+v, %v", got, err)
	}
}
