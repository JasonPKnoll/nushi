package main

import (
	"os"
	"os/exec"
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

// A data folder that is a junction is refused, and the folder it points
// to keeps its DACL: whoever made the junction could point it elsewhere
// once the check had passed.
func TestOwnDataFolderRefusesAJunction(t *testing.T) {
	target := filepath.Join(t.TempDir(), "mine")
	testcheck.NoErr(t, os.MkdirAll(target, 0o700))
	link := filepath.Join(t.TempDir(), "nd")
	cmd := filepath.Join(os.Getenv("SystemRoot"), "System32", "cmd.exe")
	if out, err := exec.Command(cmd, "/c", "mklink", "/J", link, target).CombinedOutput(); err != nil { //nolint:gosec,noctx // the system's cmd.exe, two test folders
		t.Fatalf("mklink /J: %v: %s", err, out)
	}
	if err := ownDataFolder(link); err == nil {
		t.Fatal("a junction was taken as the data folder")
	}
	sd, err := windows.GetNamedSecurityInfo(target, windows.SE_FILE_OBJECT, windows.DACL_SECURITY_INFORMATION)
	testcheck.NoErr(t, err)
	if control, _, _ := sd.Control(); control&windows.SE_DACL_PROTECTED != 0 {
		t.Error("the junction's target was given the data folder's DACL")
	}
}

// So is a symbolic link to a folder, where the system lets the test make
// one (Developer Mode, or an administrator).
func TestOwnDataFolderRefusesALink(t *testing.T) {
	target := filepath.Join(t.TempDir(), "mine")
	testcheck.NoErr(t, os.MkdirAll(target, 0o700))
	link := filepath.Join(t.TempDir(), "nd")
	if err := os.Symlink(target, link); err != nil {
		t.Skipf("no symbolic link here: %v", err)
	}
	if err := ownDataFolder(link); err == nil {
		t.Fatal("a link was taken as the data folder")
	}
}
