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
// inherit. The start stops when the folder is a link or junction (the
// DACL would land on what it points to, and whoever made it could point
// it elsewhere after), belongs to another account (it could put the
// access back) or the DACL can't be set; a token another account owns,
// put there while the folder was open, is removed, so a new one is made.
// The folder is held open as itself meanwhile, not shared for delete, so
// the one checked is the one given the DACL: it can't be renamed, removed
// or swapped for a junction in between.
func ownDataFolder(dir string) error {
	user, err := windows.GetCurrentProcessToken().GetTokenUser()
	if err != nil {
		return fmt.Errorf("reading who runs the daemon: %w", err)
	}
	me := user.User.Sid
	h, err := openFolder(dir)
	if err != nil {
		return err
	}
	defer windows.CloseHandle(h) //nolint:errcheck // a handle only read through
	if err := ownedByMe(h, dir, me); err != nil {
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
	switch err := pathOwnedByMe(token, me); {
	case errors.Is(err, fs.ErrNotExist):
	case err != nil:
		if err := os.Remove(token); err != nil {
			return fmt.Errorf("removing a token another account put in the data folder: %w", err)
		}
	}
	return nil
}

// openFolder opens the folder at path as itself (a link or junction is
// opened, not followed) and refuses anything but a plain folder. The
// handle reads its security, and isn't shared for delete, so the folder
// stays where it is while it's open.
func openFolder(path string) (windows.Handle, error) {
	h, err := openAsItself(path, windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE)
	if err != nil {
		return 0, fmt.Errorf("opening the data folder %s: %w", path, err)
	}
	var info windows.ByHandleFileInformation
	switch err := windows.GetFileInformationByHandle(h, &info); {
	case err != nil:
		err = fmt.Errorf("reading the data folder %s: %w", path, err)
	case info.FileAttributes&windows.FILE_ATTRIBUTE_REPARSE_POINT != 0:
		err = fmt.Errorf("the data folder %s is a link or junction: its target, not it, would be made yours, "+
			"and whoever made it could point it elsewhere, so Nushi won't start on it", path)
	case info.FileAttributes&windows.FILE_ATTRIBUTE_DIRECTORY == 0:
		err = fmt.Errorf("the data folder %s isn't a folder", path)
	}
	if err != nil {
		_ = windows.CloseHandle(h)
		return 0, err
	}
	return h, nil
}

// openAsItself opens path, a file or a folder, to read its attributes and
// security, a link or junction as itself, sharing it as share says.
func openAsItself(path string, share uint32) (windows.Handle, error) {
	name, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return 0, err
	}
	h, err := windows.CreateFile(name, windows.READ_CONTROL|windows.FILE_READ_ATTRIBUTES, share, nil,
		windows.OPEN_EXISTING, windows.FILE_FLAG_BACKUP_SEMANTICS|windows.FILE_FLAG_OPEN_REPARSE_POINT, 0)
	if errors.Is(err, windows.ERROR_FILE_NOT_FOUND) || errors.Is(err, windows.ERROR_PATH_NOT_FOUND) {
		return 0, fs.ErrNotExist
	}
	return h, err
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

// pathOwnedByMe is ownedByMe of what's at path, a link or junction as
// itself; fs.ErrNotExist when there's nothing.
func pathOwnedByMe(path string, me *windows.SID) error {
	h, err := openAsItself(path, windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE|windows.FILE_SHARE_DELETE)
	if errors.Is(err, fs.ErrNotExist) {
		return err
	}
	if err != nil {
		return fmt.Errorf("reading who owns %s: %w", path, err)
	}
	defer windows.CloseHandle(h) //nolint:errcheck // a handle only read through
	return ownedByMe(h, path, me)
}

// ownedByMe says the owner of path, open as h, is me, or a group this
// process makes what it creates belong to (an elevated administrator's
// Administrators); an error saying whose it is otherwise.
func ownedByMe(h windows.Handle, path string, me *windows.SID) error {
	sd, err := windows.GetSecurityInfo(h, windows.SE_FILE_OBJECT, windows.OWNER_SECURITY_INFORMATION)
	if err != nil {
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
