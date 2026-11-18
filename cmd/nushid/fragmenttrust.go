package main

import (
	"slices"
	"strings"
)

// A Windows Terminal fragment under %ProgramData% is everyone's: any
// account may create a file in C:\ProgramData, and one there could name
// the shell Nushi runs as you (ADR 0132). Windows Terminal itself checks
// nothing; Nushi takes such a fragment only when the system or an
// administrator wrote it and no one else could have put it there or can
// change it. This file weighs the facts, plain values, on any OS so CI's
// Mac and Linux jobs test it; fragmenttrust_windows.go reads them from
// handles it holds.

// machineOwners is the accounts a fragment under %ProgramData% and every
// folder on its way may belong to: SYSTEM, Administrators and
// TrustedInstaller. Not the user running the daemon: a file of theirs
// there proves nothing about who else could have written its folder.
var machineOwners = []string{
	"S-1-5-18",     // SYSTEM
	"S-1-5-32-544", // Administrators
	"S-1-5-80-956008885-3418522649-1831038044-1853292631-2271478464", // TrustedInstaller
}

// Access rights, as an ACE's mask holds them.
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

// replaceRights let their holder delete, rename or re-own what's in a
// folder, or change who may.
const replaceRights = rightDeleteChild | rightDelete | rightWriteDAC | rightWriteOwner | rightGenericAll

// fileWrites is what no one untrusted may hold on a fragment file.
const fileWrites = replaceRights | rightWriteData | rightAppendData | rightGenericWrite

// folderWrites is what no one untrusted may hold on a folder below
// %ProgramData% on the fragment's way: replaceRights, and adding a file
// or folder (a hard link to some other file of an administrator's, or a
// folder of a name not yet taken).
const folderWrites = replaceRights | rightWriteData | rightAppendData | rightGenericWrite

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

// folderFacts is a folder on a fragment's way, opened as itself.
type folderFacts struct {
	Reparse bool // a link or junction, not a folder
	Sec     security
}

// fragmentFacts is what the trust check weighs of a fragment under
// %ProgramData%, each read from a handle held until the file is read.
type fragmentFacts struct {
	Root     folderFacts   // %ProgramData% itself
	Folders  []folderFacts // each folder below it down to the fragment's: Microsoft, Windows Terminal, Fragments, its source's
	File     security
	Reparse  bool   // the file is a link
	Links    uint32 // the file's hard links
	Final    string // the file's path as the system resolved its open handle
	Expected string // the path it must be: %ProgramData%'s resolved, then the folders' and the file's names
}

// machineFragmentTrusted says the fragment f may be read, with trusted
// the accounts it and its folders may belong to (machineOwners): it is
// where it was looked for (no link or junction on the way, its resolved
// path the one expected), a plain file of one link, owned by a trusted
// account with no one else able to change it; each folder below
// %ProgramData% down to it a real folder owned by a trusted account that
// no one else can add to, delete from or change. %ProgramData% itself
// lets Users add files and folders, which can't touch a folder already
// there, so there only replaceRights count. An ACE of a kind it doesn't
// know counts as letting anyone in.
func machineFragmentTrusted(f fragmentFacts, trusted []string) bool {
	if f.Reparse || f.Links != 1 || f.Final == "" || !strings.EqualFold(f.Final, f.Expected) ||
		len(f.Folders) == 0 || f.Root.Reparse || !trustedWith(f.Root.Sec, trusted, replaceRights) {
		return false
	}
	for _, d := range f.Folders {
		if d.Reparse || !trustedWith(d.Sec, trusted, folderWrites) {
			return false
		}
	}
	return trustedWith(f.File, trusted, fileWrites)
}

// trustedWith says s is owned by one of trusted and none of its ACEs
// that applies to it gives anyone else any of rights.
func trustedWith(s security, trusted []string, rights uint32) bool {
	if s.NoDACL || s.Owner == "" || !slices.Contains(trusted, s.Owner) {
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
