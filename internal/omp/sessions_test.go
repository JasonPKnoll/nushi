package omp

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/afrase/nushi/internal/testcheck"
)

func TestForked(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "--work--")
	testcheck.NoErr(t, os.MkdirAll(dir, 0o700))
	write := func(id, header string) {
		body := `{"type":"title","v":1,"title":""}` + "\n" + header + "\n"
		if err := os.WriteFile(filepath.Join(dir, "2026-10-25T00-14-05-823Z_"+id+".jsonl"), []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	write("fork", `{"type":"session","version":3,"id":"fork","cwd":"/work","parentSession":"/x/2026_parent.jsonl"}`)
	write("plain", `{"type":"session","version":3,"id":"plain","cwd":"/work"}`)
	if got := forked(root, []string{"fork", "plain"}); len(got) != 1 || !got["fork"] {
		t.Errorf("forked = %v, want fork only", got)
	}
}
