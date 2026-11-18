package main

import "testing"

func TestMachineFragmentTrusted(t *testing.T) {
	const (
		me    = "S-1-5-21-1-2-3-1001"
		other = "S-1-5-21-1-2-3-1002"
		users = "S-1-5-32-545"
		owner = "S-1-3-0" // CREATOR OWNER
		read  = 0x1200a9  // FILE_GENERIC_READ | FILE_EXECUTE
		full  = 0x1f01ff
	)
	// As an elevated installer leaves them under C:\ProgramData: the
	// folder lets Users add files (container-inherited, not to files),
	// CREATOR OWNER's ACE only for what's created inside.
	folder := security{Owner: sidAdministrators, DACL: []ace{
		{Kind: aceAllow, SID: sidSystem, Mask: full},
		{Kind: aceAllow, SID: sidAdministrators, Mask: full},
		{Kind: aceAllow, SID: owner, Mask: full, InheritOnly: true},
		{Kind: aceAllow, SID: users, Mask: read},
		{Kind: aceAllow, SID: users, Mask: 0x2 | 0x4 | 0x10 | 0x100},
	}}
	file := security{Owner: sidAdministrators, DACL: []ace{
		{Kind: aceAllow, SID: sidSystem, Mask: full},
		{Kind: aceAllow, SID: sidAdministrators, Mask: full},
		{Kind: aceAllow, SID: users, Mask: read},
	}}
	with := func(s security, a ...ace) security {
		s.DACL = append(append([]ace{}, s.DACL...), a...)
		return s
	}
	owned := func(s security, by string) security { s.Owner = by; return s }
	for _, c := range []struct {
		name         string
		file, folder security
		want         bool
	}{
		{"an installer's", file, folder, true},
		{"one of mine", owned(file, me), folder, true},
		{"another account's file", owned(file, other), folder, false},
		{"another account's folder", file, owned(folder, other), false},
		{"Users may write the file", with(file, ace{Kind: aceAllow, SID: users, Mask: 0x2}), folder, false},
		{"another may append to it", with(file, ace{Kind: aceAllow, SID: other, Mask: 0x4}), folder, false},
		{"another may change its access", with(file, ace{Kind: aceAllow, SID: other, Mask: 0x40000}), folder, false},
		{"Everyone's GENERIC_WRITE", with(file, ace{Kind: aceAllow, SID: "S-1-1-0", Mask: 0x40000000}), folder, false},
		{"Users may delete in the folder", file, with(folder, ace{Kind: aceAllow, SID: users, Mask: 0x40}), false},
		{"Users' full control of the folder", file, with(folder, ace{Kind: aceAllow, SID: users, Mask: 0x10000000}), false},
		{"a write only inherited by what's inside", with(file, ace{Kind: aceAllow, SID: users, Mask: full, InheritOnly: true}), folder, true},
		{"a write denied", with(file, ace{Kind: aceDeny, SID: users, Mask: full}), folder, true},
		{"mine to write", with(file, ace{Kind: aceAllow, SID: me, Mask: full}), folder, true},
		{"an ACE of another kind", with(file, ace{Kind: aceOther, Mask: read}), folder, false},
		{"a NULL DACL", security{Owner: sidAdministrators, NoDACL: true}, folder, false},
		{"TrustedInstaller's", owned(file, sidTrustedInstaller), owned(folder, sidSystem), true},
		{"no user known", owned(file, ""), folder, false},
	} {
		user := me
		if c.name == "no user known" {
			user = ""
		}
		if got := machineFragmentTrusted(c.file, c.folder, user); got != c.want {
			t.Errorf("%s: %v, want %v", c.name, got, c.want)
		}
	}
}
