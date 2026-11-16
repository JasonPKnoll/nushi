package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
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

// setDACL gives path a DACL of entries, protected from what its folder
// gives it or not.
func setDACL(t *testing.T, path string, protected bool, entries ...windows.EXPLICIT_ACCESS) {
	t.Helper()
	acl, err := windows.ACLFromEntries(entries, nil)
	testcheck.NoErr(t, err)
	info := windows.SECURITY_INFORMATION(windows.DACL_SECURITY_INFORMATION | windows.UNPROTECTED_DACL_SECURITY_INFORMATION)
	if protected {
		info = windows.DACL_SECURITY_INFORMATION | windows.PROTECTED_DACL_SECURITY_INFORMATION
	}
	testcheck.NoErr(t, windows.SetNamedSecurityInfo(path, windows.SE_FILE_OBJECT, info, nil, nil, acl, nil))
}

func currentUser(t *testing.T) *windows.SID {
	t.Helper()
	user, err := windows.GetCurrentProcessToken().GetTokenUser()
	testcheck.NoErr(t, err)
	return user.User.Sid
}

// What another account could have left in the folder while it was open,
// and kept control of, stops the start, and goes on stopping it until
// it's removed: a subfolder whose DACL is protected from the folder's,
// and a file with an ACE of its own for everyone.
func TestOwnDataFolderRefusesWhatItsDACLMissed(t *testing.T) {
	everyone, err := windows.CreateWellKnownSid(windows.WinWorldSid)
	testcheck.NoErr(t, err)
	for _, tc := range []struct {
		name  string
		plant func(t *testing.T, path string)
	}{
		{"agents", func(t *testing.T, path string) {
			testcheck.NoErr(t, os.Mkdir(path, 0o700))
			setDACL(t, path, true, fullAccess(currentUser(t), windows.TRUSTEE_IS_USER))
		}},
		{"servers.json", func(t *testing.T, path string) {
			testcheck.Write(t, path, "{}")
			setDACL(t, path, false, fullAccess(everyone, windows.TRUSTEE_IS_WELL_KNOWN_GROUP))
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := filepath.Join(t.TempDir(), "nd")
			testcheck.NoErr(t, os.MkdirAll(dir, 0o700))
			planted := filepath.Join(dir, tc.name)
			tc.plant(t, planted)
			for range 2 {
				if err := ownDataFolder(dir); err == nil || !strings.Contains(err.Error(), planted) {
					t.Fatalf("ownDataFolder = %v, want a refusal naming %s", err, planted)
				}
			}
			testcheck.NoErr(t, os.RemoveAll(planted))
			testcheck.NoErr(t, ownDataFolder(dir))
			if _, err := os.Stat(filepath.Join(dir, checkedFile)); err != nil {
				t.Errorf("no %s after a clean check: %v", checkedFile, err)
			}
		})
	}
}

// An entry another account owns stops the start, where the test may give
// one to SYSTEM (an administrator's, with the privilege to restore files).
func TestOwnDataFolderRefusesAnotherAccountsEntry(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "nd")
	testcheck.NoErr(t, os.MkdirAll(filepath.Join(dir, "agents"), 0o700))
	system, err := windows.CreateWellKnownSid(windows.WinLocalSystemSid)
	testcheck.NoErr(t, err)
	agents := filepath.Join(dir, "agents")
	if err := windows.SetNamedSecurityInfo(agents, windows.SE_FILE_OBJECT, windows.OWNER_SECURITY_INFORMATION, system, nil, nil, nil); err != nil {
		t.Skipf("can't give the test's folder to SYSTEM: %v", err)
	}
	if err := ownDataFolder(dir); err == nil || !strings.Contains(err.Error(), agents) {
		t.Fatalf("ownDataFolder = %v, want a refusal naming %s", err, agents)
	}
}
