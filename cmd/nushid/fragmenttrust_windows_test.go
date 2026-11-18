package main

import (
	"path/filepath"
	"testing"

	"golang.org/x/sys/windows"

	"github.com/afrase/nushi/internal/testcheck"
)

// A fragment is read from a file of the user's that no one else may
// change, and refused once Everyone may write it, its security read from
// the open file and its folder's as itself.
func TestMachineFragmentRead(t *testing.T) {
	user := currentUserSID()
	if user == "" {
		t.Fatal("no user SID")
	}
	dir := t.TempDir()
	path := filepath.Join(dir, "a.json")
	testcheck.Write(t, path, `{"profiles": []}`)
	me, err := windows.StringToSid(user)
	testcheck.NoErr(t, err)
	everyone, err := windows.CreateWellKnownSid(windows.WinWorldSid)
	testcheck.NoErr(t, err)
	setDACL := func(everyoneMay windows.ACCESS_MASK) {
		t.Helper()
		access := []windows.EXPLICIT_ACCESS{fullAccess(me, windows.TRUSTEE_IS_USER)}
		access[0].Inheritance = windows.NO_INHERITANCE
		access = append(access, windows.EXPLICIT_ACCESS{
			AccessPermissions: everyoneMay,
			AccessMode:        windows.GRANT_ACCESS,
			Inheritance:       windows.NO_INHERITANCE,
			Trustee:           windows.TRUSTEE{TrusteeForm: windows.TRUSTEE_IS_SID, TrusteeType: windows.TRUSTEE_IS_WELL_KNOWN_GROUP, TrusteeValue: windows.TrusteeValueFromSID(everyone)},
		})
		acl, err := windows.ACLFromEntries(access, nil)
		testcheck.NoErr(t, err)
		testcheck.NoErr(t, windows.SetNamedSecurityInfo(path, windows.SE_FILE_OBJECT,
			windows.DACL_SECURITY_INFORMATION|windows.PROTECTED_DACL_SECURITY_INFORMATION, nil, nil, acl, nil))
	}

	setDACL(windows.FILE_GENERIC_READ)
	b, err := readMachineFragment(path, user)
	if err != nil || string(b) != `{"profiles": []}` {
		t.Fatalf("a fragment only its owner may change: %q, %v", b, err)
	}
	setDACL(windows.FILE_GENERIC_READ | windows.FILE_WRITE_DATA)
	if _, err := readMachineFragment(path, user); err == nil {
		t.Error("a fragment Everyone may write was read")
	}
}

// This user's fragments come before everyone's, as Windows Terminal reads
// them, and everyone's that another account may write is left out.
func TestMachineFragmentOrder(t *testing.T) {
	local, machine := t.TempDir(), t.TempDir()
	t.Setenv("LOCALAPPDATA", local)
	t.Setenv("ProgramData", machine)
	at := func(root, name string) string {
		return filepath.Join(root, "Microsoft", "Windows Terminal", "Fragments", "Git", name)
	}
	testcheck.Write(t, at(machine, "a.json"), "machine")
	testcheck.Write(t, at(local, "a.json"), "local")
	open := at(machine, "b.json")
	testcheck.Write(t, open, "planted")
	everyone, err := windows.CreateWellKnownSid(windows.WinWorldSid)
	testcheck.NoErr(t, err)
	acl, err := windows.ACLFromEntries([]windows.EXPLICIT_ACCESS{{
		AccessPermissions: windows.FILE_GENERIC_READ | windows.FILE_GENERIC_WRITE,
		AccessMode:        windows.GRANT_ACCESS,
		Inheritance:       windows.NO_INHERITANCE,
		Trustee:           windows.TRUSTEE{TrusteeForm: windows.TRUSTEE_IS_SID, TrusteeType: windows.TRUSTEE_IS_WELL_KNOWN_GROUP, TrusteeValue: windows.TrusteeValueFromSID(everyone)},
	}}, nil)
	testcheck.NoErr(t, err)
	// Added to what the file inherits, not in its place.
	testcheck.NoErr(t, windows.SetNamedSecurityInfo(open, windows.SE_FILE_OBJECT,
		windows.DACL_SECURITY_INFORMATION|windows.UNPROTECTED_DACL_SECURITY_INFORMATION, nil, nil, acl, nil))
	got := wtFragments()["Git"]
	if len(got) != 2 || string(got[0]) != "local" || string(got[1]) != "machine" {
		t.Errorf("fragments %q, want local's then machine's, without the one Everyone may write", got)
	}
}
