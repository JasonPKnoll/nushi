package main

import "testing"

func TestMachineFragmentTrusted(t *testing.T) {
	const (
		me     = "S-1-5-21-1-2-3-1001"
		other  = "S-1-5-21-1-2-3-1002"
		users  = "S-1-5-32-545"
		owner  = "S-1-3-0" // CREATOR OWNER
		read   = 0x1200a9  // FILE_GENERIC_READ | FILE_EXECUTE
		full   = 0x1f01ff
		create = 0x2 | 0x4 | 0x10 | 0x100 // add file and folder, write EA and attributes
		admins = "S-1-5-32-544"
		system = "S-1-5-18"
		path   = `\\?\C:\ProgramData\Microsoft\Windows Terminal\Fragments\Git\git-bash.json`
	)
	adminOwned := func(extra ...ace) security {
		return security{Owner: admins, DACL: append([]ace{
			{Kind: aceAllow, SID: system, Mask: full},
			{Kind: aceAllow, SID: admins, Mask: full},
			{Kind: aceAllow, SID: owner, Mask: full, InheritOnly: true},
			{Kind: aceAllow, SID: users, Mask: read},
		}, extra...)}
	}
	folder := folderFacts{Sec: adminOwned()}
	// The Git for Windows layout as installed for everyone, if the folders
	// it makes let Users only read: every folder and the file the
	// administrators', Users reading. C:\ProgramData itself lets Users
	// create files and folders in it, as it does by default.
	gitBash := func() fragmentFacts {
		return fragmentFacts{
			Root:    folderFacts{Sec: adminOwned(ace{Kind: aceAllow, SID: users, Mask: create})},
			Folders: []folderFacts{folder, folder, folder, folder},
			File:    adminOwned(),
			Links:   1,
			Final:   path,
			// As the system writes it, case aside.
			Expected: `\\?\C:\ProgramData\Microsoft\Windows Terminal\Fragments\git\git-bash.json`,
		}
	}
	for _, c := range []struct {
		name   string
		change func(*fragmentFacts)
		want   bool
	}{
		{"Git for Windows's, installed for everyone", func(*fragmentFacts) {}, true},
		{"a junction on the way", func(f *fragmentFacts) { f.Folders[2].Reparse = true }, false},
		{"resolved elsewhere", func(f *fragmentFacts) { f.Final = `\\?\C:\Users\Public\Fragments\Git\git-bash.json` }, false},
		{"no resolved path", func(f *fragmentFacts) { f.Final, f.Expected = "", "" }, false},
		{"%ProgramData% a junction", func(f *fragmentFacts) { f.Root.Reparse = true }, false},
		{"Users may delete in %ProgramData%", func(f *fragmentFacts) {
			f.Root.Sec.DACL = append(f.Root.Sec.DACL, ace{Kind: aceAllow, SID: users, Mask: 0x40})
		}, false},
		{"a folder on the way another account's", func(f *fragmentFacts) { f.Folders[1].Sec.Owner = other }, false},
		{"Users may create files in a folder on the way", func(f *fragmentFacts) {
			f.Folders[0].Sec = adminOwned(ace{Kind: aceAllow, SID: users, Mask: create})
		}, false},
		{"Users may create files in the source's folder (inherited from %ProgramData%)", func(f *fragmentFacts) {
			f.Folders[3].Sec = adminOwned(ace{Kind: aceAllow, SID: users, Mask: create})
		}, false},
		{"Users' GENERIC_WRITE on a folder", func(f *fragmentFacts) {
			f.Folders[3].Sec = adminOwned(ace{Kind: aceAllow, SID: users, Mask: 0x40000000})
		}, false},
		{"no folders", func(f *fragmentFacts) { f.Folders = nil }, false},
		{"a hard-linked file", func(f *fragmentFacts) { f.Links = 2 }, false},
		{"a link", func(f *fragmentFacts) { f.Reparse = true }, false},
		{"a file of the user's", func(f *fragmentFacts) { f.File.Owner = me }, false},
		{"a file another account owns", func(f *fragmentFacts) { f.File.Owner = other }, false},
		{"Users may write the file", func(f *fragmentFacts) { f.File = adminOwned(ace{Kind: aceAllow, SID: users, Mask: 0x2}) }, false},
		{"another may change its access", func(f *fragmentFacts) { f.File = adminOwned(ace{Kind: aceAllow, SID: other, Mask: 0x40000}) }, false},
		{"a write only what's inside inherits", func(f *fragmentFacts) {
			f.File = adminOwned(ace{Kind: aceAllow, SID: users, Mask: full, InheritOnly: true})
		}, true},
		{"a write denied", func(f *fragmentFacts) { f.File = adminOwned(ace{Kind: aceDeny, SID: users, Mask: full}) }, true},
		{"an ACE of another kind", func(f *fragmentFacts) { f.File = adminOwned(ace{Kind: aceOther, Mask: read}) }, false},
		{"a NULL DACL", func(f *fragmentFacts) { f.File = security{Owner: admins, NoDACL: true} }, false},
		{"TrustedInstaller's", func(f *fragmentFacts) { f.File.Owner = machineOwners[2] }, true},
	} {
		f := gitBash()
		f.Folders = append([]folderFacts(nil), f.Folders...)
		c.change(&f)
		if got := machineFragmentTrusted(f, machineOwners); got != c.want {
			t.Errorf("%s: %v, want %v", c.name, got, c.want)
		}
	}
}
