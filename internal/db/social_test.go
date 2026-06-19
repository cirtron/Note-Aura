package db

import (
	"path/filepath"
	"testing"
)

func TestInvitesLeaveAndBlocks(t *testing.T) {
	d, err := Open(filepath.Join(t.TempDir(), "t.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close()

	owner, _ := d.CreateUser("o@x.com", "h", false, true, "")
	b, _ := d.CreateUser("b@x.com", "h", false, true, "")
	gid, _ := d.CreateGroup(owner, "G")

	// Invite does NOT make a member; it creates a pending invite.
	if err := d.InviteToGroup(gid, b, true); err != nil {
		t.Fatal(err)
	}
	if m, _ := d.IsGroupMember(gid, b); m {
		t.Error("invited user should not be a member until accepting")
	}
	if invs, _ := d.GroupInvitesForUser(b); len(invs) != 1 {
		t.Errorf("invitee should see 1 invite, got %d", len(invs))
	}
	if pend, _ := d.PendingInvitesForGroup(gid); len(pend) != 1 {
		t.Errorf("owner should see 1 pending invite, got %d", len(pend))
	}

	// Accept → member, invite cleared, can_write carried over.
	if err := d.AcceptInvite(gid, b); err != nil {
		t.Fatal(err)
	}
	if m, cw := d.IsGroupMember(gid, b); !m || !cw {
		t.Errorf("after accept: member=%v canWrite=%v, want true,true", m, cw)
	}
	if invs, _ := d.GroupInvitesForUser(b); len(invs) != 0 {
		t.Error("invite should be cleared after accept")
	}

	// Leave revokes membership.
	if err := d.LeaveGroup(gid, b); err != nil {
		t.Fatal(err)
	}
	if m, _ := d.IsGroupMember(gid, b); m {
		t.Error("after leave, should not be a member")
	}

	// Reject (DeleteInvite) clears an invite without joining.
	_ = d.InviteToGroup(gid, b, false)
	_ = d.DeleteInvite(gid, b)
	if invs, _ := d.GroupInvitesForUser(b); len(invs) != 0 {
		t.Error("rejected invite should be gone")
	}
	if m, _ := d.IsGroupMember(gid, b); m {
		t.Error("reject must not make a member")
	}

	// Blocks are symmetric for interaction checks.
	c, _ := d.CreateUser("c@x.com", "h", false, true, "")
	if err := d.AddBlock(b, c); err != nil {
		t.Fatal(err)
	}
	if !d.BlockedEitherWay(b, c) || !d.BlockedEitherWay(c, b) {
		t.Error("block should apply in both directions for interaction checks")
	}
	if bl, _ := d.ListBlocked(b); len(bl) != 1 || bl[0].ID != c {
		t.Error("ListBlocked should return the blocked user")
	}
	if bl, _ := d.ListBlocked(c); len(bl) != 0 {
		t.Error("the blocked user hasn't blocked anyone")
	}
	_ = d.RemoveBlock(b, c)
	if d.BlockedEitherWay(b, c) {
		t.Error("after unblock, no block should remain")
	}
}
