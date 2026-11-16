package main

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"unsafe"

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
// access back) or the DACL can't be set. The folder is held open as
// itself meanwhile, not shared for delete, so the one checked is the one
// given the DACL: it can't be renamed, removed or swapped for a junction
// in between.
//
// What another account put in the folder while it was open stays that
// account's: an entry's owner can always change its DACL, and the
// folder's reaches neither an entry's own ACEs nor one whose DACL is
// protected. An agents folder planted there would run its code as the
// user, a socket the app's token. So when the folder wasn't already the
// user's alone, every entry in it is checked once the DACL is set
// (entryMine), links and junctions as themselves, and the start stops on
// one that fails, naming it. A clean check leaves checkedFile, and a
// folder already the user's alone with that file in it isn't walked
// again: nothing another account could have put there since.
func ownDataFolder(dir string) error {
	user, err := windows.GetCurrentProcessToken().GetTokenUser()
	if err != nil {
		return fmt.Errorf("reading who runs the daemon: %w", err)
	}
	me := user.User.Sid
	system, err := windows.CreateWellKnownSid(windows.WinLocalSystemSid)
	if err != nil {
		return err
	}
	h, err := openFolder(dir)
	if err != nil {
		return err
	}
	defer windows.CloseHandle(h) //nolint:errcheck // a handle only read through
	sd, err := windows.GetSecurityInfo(h, windows.SE_FILE_OBJECT,
		windows.OWNER_SECURITY_INFORMATION|windows.DACL_SECURITY_INFORMATION)
	if err != nil {
		return fmt.Errorf("reading the data folder %s's security: %w", dir, err)
	}
	if err := ownedByMe(sd, dir, me); err != nil {
		return fmt.Errorf("%w: Nushi keeps its token there, so it won't start on it", err)
	}
	checked := filepath.Join(dir, checkedFile)
	if alone(sd, me, system) && isMine(checked, me) {
		return nil
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
	if err := os.Remove(checked); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return err
	}
	err = filepath.WalkDir(dir, func(path string, _ fs.DirEntry, err error) error {
		switch {
		case err != nil:
			return fmt.Errorf("checking the data folder: %w", err)
		case path == dir:
			return nil
		}
		return entryMine(path, me, system)
	})
	if err != nil {
		return err
	}
	return os.WriteFile(checked, nil, 0o600)
}

// checkedFile, in the data folder, says every entry in it was found the
// user's (entryMine) once its DACL was set.
const checkedFile = "entries-checked"

// entryMine says the entry at path in the data folder, a link or junction
// as itself, is the user's, and lets in no one the folder doesn't: owned
// by me (ownedByMe), its DACL not protected from the folder's, and none of
// its ACEs letting in anyone but me and system. An error naming it
// otherwise.
func entryMine(path string, me, system *windows.SID) error {
	sd, err := securityOf(path)
	if err != nil {
		return fmt.Errorf("reading the security of %s, in the data folder: %w", path, err)
	}
	const remove = "another account could change it, so Nushi won't start until it's removed"
	if err := ownedByMe(sd, path, me); err != nil {
		return fmt.Errorf("%w: %s", err, remove)
	}
	control, _, err := sd.Control()
	if err != nil || control&windows.SE_DACL_PROTECTED != 0 || !letsInOnly(sd, me, system) {
		return fmt.Errorf("%s, in the data folder, has access of its own, not only the folder's: %s", path, remove)
	}
	return nil
}

// alone says the folder's security, sd, is the DACL ownDataFolder sets,
// or as tight: protected, and letting in me and system only.
func alone(sd *windows.SECURITY_DESCRIPTOR, me, system *windows.SID) bool {
	control, _, err := sd.Control()
	return err == nil && control&windows.SE_DACL_PROTECTED != 0 && letsInOnly(sd, me, system)
}

// letsInOnly says sd has a DACL (none lets everyone in) whose every ACE
// either denies or allows one of sids. Any other kind of ACE counts as
// letting in someone else.
func letsInOnly(sd *windows.SECURITY_DESCRIPTOR, sids ...*windows.SID) bool {
	dacl, _, err := sd.DACL()
	if err != nil || dacl == nil {
		return false
	}
	for i := range uint32(dacl.AceCount) {
		var ace *windows.ACCESS_ALLOWED_ACE
		if windows.GetAce(dacl, i, &ace) != nil {
			return false
		}
		switch ace.Header.AceType {
		case windows.ACCESS_DENIED_ACE_TYPE:
		case windows.ACCESS_ALLOWED_ACE_TYPE:
			sid := (*windows.SID)(unsafe.Pointer(&ace.SidStart)) //nolint:gosec // the ACE's SID starts there, as the system lays it out
			if !slices.ContainsFunc(sids, sid.Equals) {
				return false
			}
		default:
			return false
		}
	}
	return true
}

// isMine says what's at path, a link or junction as itself, is there and
// owned by me.
func isMine(path string, me *windows.SID) bool {
	sd, err := securityOf(path)
	return err == nil && ownedByMe(sd, path, me) == nil
}

// securityOf is the owner and DACL of what's at path, a link or junction
// as itself.
func securityOf(path string) (*windows.SECURITY_DESCRIPTOR, error) {
	h, err := openAsItself(path, windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE|windows.FILE_SHARE_DELETE)
	if err != nil {
		return nil, err
	}
	defer windows.CloseHandle(h) //nolint:errcheck // a handle only read through
	return windows.GetSecurityInfo(h, windows.SE_FILE_OBJECT,
		windows.OWNER_SECURITY_INFORMATION|windows.DACL_SECURITY_INFORMATION)
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

// ownedByMe says the owner in path's security, sd, is me, or a group
// this process makes what it creates belong to (an elevated
// administrator's Administrators); an error saying whose it is
// otherwise.
func ownedByMe(sd *windows.SECURITY_DESCRIPTOR, path string, me *windows.SID) error {
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
	return fmt.Errorf("%s belongs to %s, not to you", path, name)
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
