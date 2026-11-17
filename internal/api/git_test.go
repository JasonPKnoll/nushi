package api

import (
	"os"
	"path/filepath"
	"testing"

	"connectrpc.com/connect"

	"github.com/afrase/nushi/internal/features/agents"
	nushiv1 "github.com/afrase/nushi/internal/gen/nushi/v1"
	"github.com/afrase/nushi/internal/shellenv"
	"github.com/afrase/nushi/internal/testcheck"
)

func TestStageAndCommitThroughTheAPI(t *testing.T) {
	// A commit runs with the daemon's environment here: not the owner's config.
	t.Setenv("GIT_CONFIG_GLOBAL", "/dev/null")
	t.Setenv("GIT_CONFIG_NOSYSTEM", "1")
	repo := t.TempDir()
	testcheck.Git(t, repo, "init", "-q", "-b", "main")
	testcheck.Git(t, repo, "config", "user.name", "Test")
	testcheck.Git(t, repo, "config", "user.email", "test@example.com")
	testcheck.NoErr(t, os.WriteFile(filepath.Join(repo, "f.txt"), []byte("x\n"), 0o600))

	c := client(t, New(Services{Agents: agents.New(nil, nil, shellenv.Off(os.Environ()), "", nil)}))
	ctx := t.Context()
	if _, err := c.StageFiles(ctx, connect.NewRequest(&nushiv1.StageFilesRequest{Folder: repo, Paths: []string{"f.txt"}, Staged: true})); err != nil {
		t.Fatal(err)
	}
	diff, err := c.GetDiff(ctx, connect.NewRequest(&nushiv1.GetDiffRequest{Folder: repo}))
	if err != nil || diff.Msg.Index == "" || len(diff.Msg.Files) != 1 || diff.Msg.Files[0].Staged != "all" {
		t.Fatalf("diff: %v %v", diff, err)
	}
	if _, err := c.CommitStaged(ctx, connect.NewRequest(&nushiv1.CommitStagedRequest{Folder: repo, Message: "  ", Index: diff.Msg.Index})); connect.CodeOf(err) != connect.CodeInvalidArgument {
		t.Errorf("no message: %v", err)
	}
	if _, err := c.CommitStaged(ctx, connect.NewRequest(&nushiv1.CommitStagedRequest{Folder: repo, Message: "m", Index: "old"})); connect.CodeOf(err) != connect.CodeFailedPrecondition {
		t.Errorf("an old index: %v", err)
	}
	done, err := c.CommitStaged(ctx, connect.NewRequest(&nushiv1.CommitStagedRequest{Folder: repo, Message: "m", Index: diff.Msg.Index}))
	if err != nil || done.Msg.Short == "" {
		t.Fatalf("commit: %v %v", done, err)
	}

	d := client(t, ForDevices(Services{}))
	if _, err := d.StageFiles(ctx, connect.NewRequest(&nushiv1.StageFilesRequest{Folder: repo})); connect.CodeOf(err) != connect.CodePermissionDenied {
		t.Errorf("a device staged: %v", err)
	}
	if _, err := d.CommitStaged(ctx, connect.NewRequest(&nushiv1.CommitStagedRequest{Folder: repo})); connect.CodeOf(err) != connect.CodePermissionDenied {
		t.Errorf("a device committed: %v", err)
	}
}

// TestGetDiffOfOnePath: a file preview's GetDiff (path) answers that file
// alone, without the index's state, and refuses a path outside its folder.
func TestGetDiffOfOnePath(t *testing.T) {
	repo := t.TempDir()
	c := client(t, New(Services{}))
	ctx := t.Context()
	if _, err := c.GetDiff(ctx, connect.NewRequest(&nushiv1.GetDiffRequest{Folder: repo, Path: "a.txt"})); connect.CodeOf(err) != connect.CodeFailedPrecondition {
		t.Errorf("not a repository: %v", err)
	}
	testcheck.Git(t, repo, "init", "-q", "-b", "main")
	testcheck.Write(t, filepath.Join(repo, "a.txt"), "one\n")
	testcheck.Write(t, filepath.Join(repo, "b.txt"), "b\n")
	testcheck.Git(t, repo, "add", ".")
	testcheck.Git(t, repo, "commit", "-q", "-m", "init")
	testcheck.Write(t, filepath.Join(repo, "a.txt"), "one\ntwo\n")
	testcheck.Write(t, filepath.Join(repo, "b.txt"), "b2\n")
	testcheck.Git(t, repo, "add", "a.txt")

	diff, err := c.GetDiff(ctx, connect.NewRequest(&nushiv1.GetDiffRequest{Folder: repo, Path: "a.txt"}))
	if err != nil || diff.Msg.Index != "" || len(diff.Msg.Files) != 1 {
		t.Fatalf("one path: %v %v", diff, err)
	}
	if f := diff.Msg.Files[0]; f.Path != "a.txt" || f.OldText != "one\n" || f.Staged != "" {
		t.Errorf("its change: %v", f)
	}
	for _, p := range []string{"../a.txt", "/etc/hosts"} {
		if _, err := c.GetDiff(ctx, connect.NewRequest(&nushiv1.GetDiffRequest{Folder: repo, Path: p})); connect.CodeOf(err) != connect.CodeInvalidArgument {
			t.Errorf("%s: %v", p, err)
		}
	}
}
