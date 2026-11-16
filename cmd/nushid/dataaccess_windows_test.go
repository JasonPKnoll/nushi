package main

import (
	"os"
	"path/filepath"
	"testing"

	"golang.org/x/sys/windows"

	"github.com/afrase/nushi/internal/testcheck"
)

// The data folder ends with a protected DACL of two entries, the user's
// and SYSTEM's, and the files already in it lose what they inherited
// from above.
func TestOwnDataFolderProtectsIt(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "nd")
	testcheck.NoErr(t, os.MkdirAll(dir, 0o700))
	testcheck.Write(t, filepath.Join(dir, "token"), "x")
	testcheck.NoErr(t, ownDataFolder(dir))

	for _, path := range []string{dir, filepath.Join(dir, "token")} {
		sd, err := windows.GetNamedSecurityInfo(path, windows.SE_FILE_OBJECT, windows.DACL_SECURITY_INFORMATION)
		testcheck.NoErr(t, err)
		control, _, err := sd.Control()
		testcheck.NoErr(t, err)
		dacl, _, err := sd.DACL()
		testcheck.NoErr(t, err)
		if path == dir && control&windows.SE_DACL_PROTECTED == 0 {
			t.Errorf("%s: DACL not protected", path)
		}
		if dacl == nil || dacl.AceCount != 2 {
			t.Errorf("%s: DACL has %v entries, want the user's and SYSTEM's", path, dacl)
		}
	}
	if _, err := os.Stat(filepath.Join(dir, "token")); err != nil {
		t.Errorf("the user's own token was removed: %v", err)
	}
}
