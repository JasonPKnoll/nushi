package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"unsafe"

	"golang.org/x/sys/windows"

	"github.com/afrase/nushi/internal/regularfile"
)

// fragmentWay is the folders from %ProgramData% down to a fragment's
// source's folder, which follows them.
var fragmentWay = []string{"Microsoft", "Windows Terminal", "Fragments"}

// readMachineFragment reads the fragment name in source's folder under
// root (%ProgramData%) when machineFragmentTrusted takes it, with trusted
// the accounts it may belong to (machineOwners). Every folder on the way
// is opened as itself (a link or junction is opened, not followed) and
// held, not shared for delete, so none can be renamed or swapped until
// the file is read; the file is opened once, a link refused, and its
// links, security and resolved path are read from that handle, the text
// read from it too.
func readMachineFragment(root, source, name string, trusted []string) ([]byte, error) {
	var facts fragmentFacts
	var held []windows.Handle
	defer func() {
		for _, h := range held {
			_ = windows.CloseHandle(h)
		}
	}()
	dir := root
	for i, part := range append(append([]string{""}, fragmentWay...), source) {
		if part != "" {
			dir = filepath.Join(dir, part)
		}
		h, err := openAsItself(dir, windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE)
		if err != nil {
			return nil, fmt.Errorf("opening %s: %w", dir, err)
		}
		held = append(held, h)
		d, err := folderFactsOf(h)
		if err != nil {
			return nil, fmt.Errorf("reading %s: %w", dir, err)
		}
		if i == 0 {
			facts.Root = d
			final, err := finalPath(h)
			if err != nil {
				return nil, fmt.Errorf("resolving %s: %w", dir, err)
			}
			facts.Expected = strings.TrimSuffix(final, `\`) // a drive's root ends in one
			continue
		}
		facts.Folders = append(facts.Folders, d)
		facts.Expected += `\` + part
	}
	path := filepath.Join(dir, name)
	facts.Expected += `\` + name
	f, fi, err := regularfile.Open(path, false)
	if err != nil {
		return nil, err
	}
	defer f.Close() //nolint:errcheck // read-only
	h := windows.Handle(f.Fd())
	var info windows.ByHandleFileInformation
	if err := windows.GetFileInformationByHandle(h, &info); err != nil {
		return nil, fmt.Errorf("reading %s: %w", path, err)
	}
	facts.Links = info.NumberOfLinks
	facts.Reparse = info.FileAttributes&windows.FILE_ATTRIBUTE_REPARSE_POINT != 0
	// os.Open asks for GENERIC_READ, which holds READ_CONTROL.
	sd, err := windows.GetSecurityInfo(h, windows.SE_FILE_OBJECT,
		windows.OWNER_SECURITY_INFORMATION|windows.DACL_SECURITY_INFORMATION)
	if err != nil {
		return nil, fmt.Errorf("reading the security of %s: %w", path, err)
	}
	if facts.File, err = securityFacts(sd); err != nil {
		return nil, fmt.Errorf("reading the security of %s: %w", path, err)
	}
	if facts.Final, err = finalPath(h); err != nil {
		return nil, fmt.Errorf("resolving %s: %w", path, err)
	}
	if !machineFragmentTrusted(facts, trusted) {
		return nil, errors.New("another account could have put it there or could change it")
	}
	return regularfile.ReadAll(f, fi, maxWTSettings)
}

// folderFactsOf is the folder open at h as the trust check weighs it; an
// error when it is neither a folder nor a link or junction.
func folderFactsOf(h windows.Handle) (folderFacts, error) {
	var info windows.ByHandleFileInformation
	if err := windows.GetFileInformationByHandle(h, &info); err != nil {
		return folderFacts{}, err
	}
	reparse := info.FileAttributes&windows.FILE_ATTRIBUTE_REPARSE_POINT != 0
	if !reparse && info.FileAttributes&windows.FILE_ATTRIBUTE_DIRECTORY == 0 {
		return folderFacts{}, errors.New("not a folder")
	}
	sd, err := windows.GetSecurityInfo(h, windows.SE_FILE_OBJECT,
		windows.OWNER_SECURITY_INFORMATION|windows.DACL_SECURITY_INFORMATION)
	if err != nil {
		return folderFacts{}, err
	}
	sec, err := securityFacts(sd)
	return folderFacts{Reparse: reparse, Sec: sec}, err
}

// finalPath is the path the system resolved the open file or folder at h
// to (GetFinalPathNameByHandle, normalised, with its drive letter: the
// `\\?\` form).
func finalPath(h windows.Handle) (string, error) {
	const normalisedDOS = 0 // FILE_NAME_NORMALIZED | VOLUME_NAME_DOS
	buf := make([]uint16, windows.MAX_LONG_PATH)
	n, err := windows.GetFinalPathNameByHandle(h, &buf[0], uint32(len(buf)), normalisedDOS) //nolint:gosec // MAX_LONG_PATH fits
	if err != nil {
		return "", err
	}
	if int(n) >= len(buf) { // the size it needs, longer than any path
		return "", os.ErrInvalid
	}
	return windows.UTF16ToString(buf[:n]), nil
}

// securityFacts is sd's owner and DACL as the trust check weighs them.
func securityFacts(sd *windows.SECURITY_DESCRIPTOR) (security, error) {
	owner, _, err := sd.Owner()
	if err != nil {
		return security{}, err
	}
	if owner == nil {
		return security{}, errors.New("no owner")
	}
	s := security{Owner: owner.String()}
	dacl, _, err := sd.DACL()
	switch {
	case errors.Is(err, windows.ERROR_OBJECT_NOT_FOUND) || err == nil && dacl == nil:
		// No DACL, or a NULL one: everyone may do anything.
		s.NoDACL = true
		return s, nil
	case err != nil:
		return security{}, err
	}
	for i := range uint32(dacl.AceCount) {
		var a *windows.ACCESS_ALLOWED_ACE
		if err := windows.GetAce(dacl, i, &a); err != nil {
			return security{}, err
		}
		s.DACL = append(s.DACL, aceFacts(a))
	}
	return s, nil
}

// aceFacts is one ACE as the trust check weighs it. Only an allow or deny
// ACE has its SID right after the mask (ACCESS_ALLOWED_ACE's and
// ACCESS_DENIED_ACE's layout); any other kind is aceOther, its SID unread.
func aceFacts(a *windows.ACCESS_ALLOWED_ACE) ace {
	out := ace{InheritOnly: a.Header.AceFlags&windows.INHERIT_ONLY_ACE != 0, Mask: uint32(a.Mask)}
	switch a.Header.AceType {
	case windows.ACCESS_ALLOWED_ACE_TYPE:
		out.Kind = aceAllow
	case windows.ACCESS_DENIED_ACE_TYPE:
		out.Kind = aceDeny
	default:
		out.Kind = aceOther
		return out
	}
	out.SID = (*windows.SID)(unsafe.Pointer(&a.SidStart)).String() //nolint:gosec // the ACE's SID starts there, as the system lays it out
	return out
}
