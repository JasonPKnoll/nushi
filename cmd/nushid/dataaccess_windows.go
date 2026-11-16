package main

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"golang.org/x/sys/windows"
)

// ownDataFolder makes the data folder the user's alone, as 0700 does on
// Unix, which Windows's mode bits can't: a protected DACL that lets in
// the user and SYSTEM only, and nothing inherited from the folder above.
// A NUSHI_DATA outside the profile (C:\nd) would otherwise inherit what
// the drive gives Authenticated Users, and any account on the machine
// could read the token or plant one, and so run anything as the user
// through the socket. Setting the DACL redoes what the files inside
// inherit. The start stops when the folder belongs to another account
// (it could put the access back) or the DACL can't be set; a token
// another account owns, put there while the folder was open, is removed,
// so a new one is made.
func ownDataFolder(dir string) error {
	user, err := windows.GetCurrentProcessToken().GetTokenUser()
	if err != nil {
		return fmt.Errorf("reading who runs the daemon: %w", err)
	}
	me := user.User.Sid
	if err := ownedByMe(dir, me); err != nil {
		return err
	}
	system, err := windows.CreateWellKnownSid(windows.WinLocalSystemSid)
	if err != nil {
		return err
	}
	acl, err := windows.ACLFromEntries([]windows.EXPLICIT_ACCESS{
		fullAccess(me, windows.TRUSTEE_IS_USER),
		fullAccess(system, windows.TRUSTEE_IS_WELL_KNOWN_GROUP),
	}, nil)
	if err != nil {
		return err
	}
	if err := windows.SetNamedSecurityInfo(dir, windows.SE_FILE_OBJECT,
		windows.DACL_SECURITY_INFORMATION|windows.PROTECTED_DACL_SECURITY_INFORMATION, nil, nil, acl, nil); err != nil {
		return fmt.Errorf("making the data folder %s yours alone: %w", dir, err)
	}
	token := filepath.Join(dir, "token")
	switch err := ownedByMe(token, me); {
	case errors.Is(err, fs.ErrNotExist):
	case err != nil:
		if err := os.Remove(token); err != nil {
			return fmt.Errorf("removing a token another account put in the data folder: %w", err)
		}
	}
	return nil
}

// fileAllAccess is FILE_ALL_ACCESS: every right on a file or folder, by
// its own bits, so the ACE is inherited as it is rather than split in two
// as a generic right's is.
const fileAllAccess = windows.STANDARD_RIGHTS_ALL | windows.SYNCHRONIZE | 0x1ff

// fullAccess is an ACE giving sid every right on the folder and what's in
// it.
func fullAccess(sid *windows.SID, kind windows.TRUSTEE_TYPE) windows.EXPLICIT_ACCESS {
	return windows.EXPLICIT_ACCESS{
		AccessPermissions: fileAllAccess,
		AccessMode:        windows.GRANT_ACCESS,
		Inheritance:       windows.SUB_CONTAINERS_AND_OBJECTS_INHERIT,
		Trustee:           windows.TRUSTEE{TrusteeForm: windows.TRUSTEE_IS_SID, TrusteeType: kind, TrusteeValue: windows.TrusteeValueFromSID(sid)},
	}
}

// ownedByMe says path's owner is me, or a group this process makes what
// it creates belong to (an elevated administrator's Administrators); an
// error saying whose it is otherwise.
func ownedByMe(path string, me *windows.SID) error {
	sd, err := windows.GetNamedSecurityInfo(path, windows.SE_FILE_OBJECT, windows.OWNER_SECURITY_INFORMATION)
	if err != nil {
		if errors.Is(err, windows.ERROR_FILE_NOT_FOUND) {
			return fs.ErrNotExist
		}
		return fmt.Errorf("reading who owns %s: %w", path, err)
	}
	owner, _, err := sd.Owner()
	if err != nil {
		return fmt.Errorf("reading who owns %s: %w", path, err)
	}
	if owner.Equals(me) || ownerGroup(owner) {
		return nil
	}
	name := owner.String()
	if account, domain, _, err := owner.LookupAccount(""); err == nil {
		name = domain + `\` + account
	}
	return fmt.Errorf("%s belongs to %s, not to you: Nushi keeps its token there, so it won't start on it", path, name)
}

// ownerGroup says sid is a group of this process's token that may own
// what the process creates, enabled.
func ownerGroup(sid *windows.SID) bool {
	groups, err := windows.GetCurrentProcessToken().GetTokenGroups()
	if err != nil {
		return false
	}
	for _, g := range groups.AllGroups() {
		if g.Attributes&windows.SE_GROUP_OWNER != 0 && g.Attributes&windows.SE_GROUP_ENABLED != 0 && g.Sid.Equals(sid) {
			return true
		}
	}
	return false
}
