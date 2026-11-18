package main

import "slices"

// A Windows Terminal fragment under %ProgramData% is everyone's: any
// account may create a file in its folders (C:\ProgramData lets Users add
// files and folders below it), and one there could name the shell Nushi
// runs as you (ADR 0132). Windows Terminal itself checks nothing; Nushi
// takes such a fragment only when an administrator, the system or you
// wrote it and no one else can change it. This file weighs a file's and
// its folder's security, as plain facts, on any OS so CI's Mac and Linux
// jobs test it; fragmenttrust_windows.go reads the facts.

// Accounts whose files a fragment may come from: SYSTEM, Administrators
// and TrustedInstaller, beside the user running the daemon.
const (
	sidSystem           = "S-1-5-18"
	sidAdministrators   = "S-1-5-32-544"
	sidTrustedInstaller = "S-1-5-80-956008885-3418522649-1831038044-1853292631-2271478464"
)

// Access rights, as an ACE's mask holds them, that let someone change a
// file's text or who may change it.
const (
	rightWriteData    = 0x2 // FILE_WRITE_DATA; FILE_ADD_FILE on a folder
	rightAppendData   = 0x4 // FILE_APPEND_DATA; FILE_ADD_SUBDIRECTORY on a folder
	rightDeleteChild  = 0x40
	rightDelete       = 0x10000
	rightWriteDAC     = 0x40000
	rightWriteOwner   = 0x80000
	rightGenericAll   = 0x10000000
	rightGenericWrite = 0x40000000
)

// fileWrites is what no one else may hold on a fragment file: writing
// it, deleting it (and so putting another in its place), or changing its
// owner or access.
const fileWrites = rightWriteData | rightAppendData | rightDelete | rightWriteDAC | rightWriteOwner | rightGenericAll | rightGenericWrite

// folderWrites is what no one else may hold on a fragment's folder:
// deleting or renaming what's in it, or changing its owner or access.
// Adding a file isn't among them: everyone may by default, and a file
// added by another account is its own and so fails the file's check.
const folderWrites = rightDeleteChild | rightDelete | rightWriteDAC | rightWriteOwner | rightGenericAll

// aceKind is what an ACE does: allow, deny, or a kind Nushi doesn't weigh.
type aceKind int

const (
	aceAllow aceKind = iota
	aceDeny
	aceOther
)

// ace is an access control entry as the trust check weighs it.
type ace struct {
	Kind        aceKind
	InheritOnly bool   // it applies only to what's created inside, not to this
	SID         string // whom it names, "S-1-…"
	Mask        uint32
}

// security is a file's or folder's owner and DACL, as facts.
type security struct {
	Owner  string // "S-1-…"
	NoDACL bool   // a NULL DACL, which lets everyone do anything
	DACL   []ace
}

// machineFragmentTrusted says a fragment under %ProgramData%, file, in
// folder (its source's), may be read for user's shell: each owned by
// SYSTEM, Administrators, TrustedInstaller or user, and neither letting
// anyone else change it (fileWrites on the file, folderWrites on the
// folder). An ACE of a kind it doesn't know counts as letting anyone in.
func machineFragmentTrusted(file, folder security, user string) bool {
	return trustedWith(file, user, fileWrites) && trustedWith(folder, user, folderWrites)
}

// trustedWith says s is owned by a trusted account and none of its ACEs
// gives anyone else any of rights.
func trustedWith(s security, user string, rights uint32) bool {
	trusted := []string{sidSystem, sidAdministrators, sidTrustedInstaller}
	if user != "" {
		trusted = append(trusted, user)
	}
	if s.NoDACL || !slices.Contains(trusted, s.Owner) {
		return false
	}
	for _, a := range s.DACL {
		switch {
		case a.InheritOnly, a.Kind == aceDeny:
		case a.Kind != aceAllow:
			return false
		case a.Mask&rights != 0 && !slices.Contains(trusted, a.SID):
			return false
		}
	}
	return true
}
