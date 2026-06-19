package db

import (
	"path/filepath"
	"testing"
)

func TestUserAdminAndGroupAdmin(t *testing.T) {
	d, err := Open(filepath.Join(t.TempDir(), "t.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close()

	owner, _ := d.CreateUser("o@x.com", "h", false, true, "")
	m, _ := d.CreateUser("m@x.com", "h", false, true, "")

	// Platform admin promote/demote.
	u, _ := d.GetUser(m)
	if u.IsAdmin {
		t.Fatal("new user should not be admin")
	}
	if err := d.SetUserAdmin(m, true); err != nil {
		t.Fatal(err)
	}
	if u, _ := d.GetUser(m); !u.IsAdmin {
		t.Error("SetUserAdmin(true) should promote")
	}
	if err := d.SetUserAdmin(m, false); err != nil {
		t.Fatal(err)
	}
	if u, _ := d.GetUser(m); u.IsAdmin {
		t.Error("SetUserAdmin(false) should demote")
	}

	// Group co-admins: owner is always a group admin; a member can be promoted.
	gid, _ := d.CreateGroup(owner, "G")
	if !d.IsGroupAdmin(gid, owner) {
		t.Error("owner must be a group admin")
	}
	if err := d.AddGroupMember(gid, "m@x.com", true); err != nil {
		t.Fatal(err)
	}
	if d.IsGroupAdmin(gid, m) {
		t.Error("plain member should not be a group admin")
	}
	if err := d.SetGroupMemberAdmin(gid, m, true); err != nil {
		t.Fatal(err)
	}
	if !d.IsGroupAdmin(gid, m) {
		t.Error("promoted member should be a group admin")
	}
	// The promoted member shows the admin flag in the member list.
	ms, _ := d.GroupMembers(gid)
	if len(ms) != 1 || !ms[0].IsAdmin {
		t.Errorf("member list should reflect group-admin flag: %+v", ms)
	}
	if err := d.SetGroupMemberAdmin(gid, m, false); err != nil {
		t.Fatal(err)
	}
	if d.IsGroupAdmin(gid, m) {
		t.Error("demoted member should no longer be a group admin")
	}
}
