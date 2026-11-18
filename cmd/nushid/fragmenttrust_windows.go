package main

import (
	"errors"
	"fmt"
	"path/filepath"
	"unsafe"

	"golang.org/x/sys/windows"

	"github.com/afrase/nushi/internal/regularfile"
)

// readMachineFragment reads the fragment at path, under %ProgramData%,
// when machineFragmentTrusted takes it for user (a SID). The file is
// opened once, a link or junction as itself (and so refused), and its
// security is read from that handle, so the file checked is the file
// read; its folder's is read opened as itself (securityOf).
func readMachineFragment(path, user string) ([]byte, error) {
	f, fi, err := regularfile.Open(path, false)
	if err != nil {
		return nil, err
	}
	defer f.Close() //nolint:errcheck // read-only
	// os.Open asks for GENERIC_READ, which holds READ_CONTROL.
	sd, err := windows.GetSecurityInfo(windows.Handle(f.Fd()), windows.SE_FILE_OBJECT,
		windows.OWNER_SECURITY_INFORMATION|windows.DACL_SECURITY_INFORMATION)
	if err != nil {
		return nil, fmt.Errorf("reading the security of %s: %w", path, err)
	}
	file, err := securityFacts(sd)
	if err != nil {
		return nil, fmt.Errorf("reading the security of %s: %w", path, err)
	}
	dir := filepath.Dir(path)
	dirSD, err := securityOf(dir)
	if err != nil {
		return nil, fmt.Errorf("reading the security of %s: %w", dir, err)
	}
	folder, err := securityFacts(dirSD)
	if err != nil {
		return nil, fmt.Errorf("reading the security of %s: %w", dir, err)
	}
	if !machineFragmentTrusted(file, folder, user) {
		return nil, errors.New("another account could have written or could change " + path)
	}
	return regularfile.ReadAll(f, fi, maxWTSettings)
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

// currentUserSID is the SID of the account the daemon runs as, "" when
// it can't be read (and then no %ProgramData% fragment is read).
func currentUserSID() string {
	u, err := windows.GetCurrentProcessToken().GetTokenUser()
	if err != nil {
		return ""
	}
	return u.User.Sid.String()
}
