package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"golang.org/x/sys/windows"

	"github.com/afrase/nushi/internal/testcheck"
)

// testOwners is machineOwners and the account the tests run as, which
// owns the files they make (or Administrators does, when elevated).
func testOwners(t *testing.T) []string {
	t.Helper()
	u, err := windows.GetCurrentProcessToken().GetTokenUser()
	testcheck.NoErr(t, err)
	return append([]string{u.User.Sid.String()}, machineOwners...)
}

// fragmentAt writes text as fragment name of source under root, as
// %ProgramData% holds one, and says its path.
func fragmentAt(t *testing.T, root, source, name, text string) string {
	t.Helper()
	path := filepath.Join(append(append([]string{root}, fragmentWay...), source, name)...)
	testcheck.Write(t, path, text)
	return path
}

// letEveryone adds an ACE letting Everyone have mask on path, beside
// what it inherits.
func letEveryone(t *testing.T, path string, mask windows.ACCESS_MASK) {
	t.Helper()
	everyone, err := windows.CreateWellKnownSid(windows.WinWorldSid)
	testcheck.NoErr(t, err)
	acl, err := windows.ACLFromEntries([]windows.EXPLICIT_ACCESS{{
		AccessPermissions: mask,
		AccessMode:        windows.GRANT_ACCESS,
		Inheritance:       windows.NO_INHERITANCE,
		Trustee:           windows.TRUSTEE{TrusteeForm: windows.TRUSTEE_IS_SID, TrusteeType: windows.TRUSTEE_IS_WELL_KNOWN_GROUP, TrusteeValue: windows.TrusteeValueFromSID(everyone)},
	}}, nil)
	testcheck.NoErr(t, err)
	testcheck.NoErr(t, windows.SetNamedSecurityInfo(path, windows.SE_FILE_OBJECT,
		windows.DACL_SECURITY_INFORMATION|windows.UNPROTECTED_DACL_SECURITY_INFORMATION, nil, nil, acl, nil))
}

// A fragment whose file and folders only trusted accounts may change is
// read, through handles; one owned by an account not trusted, one
// Everyone may write, one with a second hard link, one in a folder
// Everyone may add to, or one reached through a junction is refused.
func TestMachineFragmentRead(t *testing.T) {
	owners := testOwners(t)
	root := t.TempDir()
	fragmentAt(t, root, "Git", "a.json", "ok")
	if b, err := readMachineFragment(root, "Git", "a.json", owners); err != nil || string(b) != "ok" {
		t.Fatalf("a fragment only trusted accounts may change: %q, %v", b, err)
	}
	if _, err := readMachineFragment(root, "Git", "a.json", machineOwners[:1]); err == nil {
		t.Error("a fragment of an account not trusted was read")
	}

	letEveryone(t, fragmentAt(t, root, "Writable", "a.json", "x"), windows.FILE_WRITE_DATA)
	if _, err := readMachineFragment(root, "Writable", "a.json", owners); err == nil {
		t.Error("a fragment Everyone may write was read")
	}

	linked := fragmentAt(t, root, "Linked", "a.json", "x")
	testcheck.NoErr(t, os.Link(linked, filepath.Join(t.TempDir(), "b.json")))
	if _, err := readMachineFragment(root, "Linked", "a.json", owners); err == nil {
		t.Error("a fragment with a second hard link was read")
	}

	open := fragmentAt(t, root, "Open", "a.json", "x")
	letEveryone(t, filepath.Dir(open), windows.FILE_WRITE_DATA) // FILE_ADD_FILE on a folder
	if _, err := readMachineFragment(root, "Open", "a.json", owners); err == nil {
		t.Error("a fragment in a folder Everyone may add to was read")
	}

	// root2's Microsoft is a junction to root's.
	root2 := t.TempDir()
	out, err := exec.Command(filepath.Join(os.Getenv("SystemRoot"), "System32", "cmd.exe"), "/c", "mklink", "/J", //nolint:gosec,noctx // the system's cmd.exe making a junction between the test's own folders
		filepath.Join(root2, "Microsoft"), filepath.Join(root, "Microsoft")).CombinedOutput()
	if err != nil {
		t.Fatalf("making a junction: %v, %s", err, out)
	}
	if _, err := readMachineFragment(root2, "Git", "a.json", owners); err == nil {
		t.Error("a fragment reached through a junction was read")
	}
}

// This user's fragments come before everyone's, as Windows Terminal reads
// them, and everyone's that another account may write is left out.
func TestMachineFragmentOrder(t *testing.T) {
	local, machine := t.TempDir(), t.TempDir()
	fragmentAt(t, machine, "Git", "a.json", "machine")
	fragmentAt(t, local, "Git", "a.json", "local")
	letEveryone(t, fragmentAt(t, machine, "Git", "b.json", "planted"), windows.FILE_WRITE_DATA)
	got := fragmentsUnder(local, machine, testOwners(t))["Git"]
	if len(got) != 2 || string(got[0]) != "local" || string(got[1]) != "machine" {
		t.Errorf("fragments %q, want local's then machine's, without the one Everyone may write", got)
	}
}
