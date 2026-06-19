package db

import (
	"path/filepath"
	"testing"
)

func TestVerificationAndInviteLimits(t *testing.T) {
	d, err := Open(filepath.Join(t.TempDir(), "t.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close()

	// Email verification token round-trip.
	uid, _ := d.CreateUser("u@x.com", "h", false, false, "tok123")
	if u, _ := d.GetUser(uid); u.EmailVerified || u.VerifyToken != "tok123" {
		t.Fatal("user should be unverified with the token set")
	}
	if gu, err := d.GetUserByVerifyToken("tok123"); err != nil || gu.ID != uid {
		t.Fatalf("GetUserByVerifyToken: %v", err)
	}
	if err := d.SetEmailVerified(uid); err != nil {
		t.Fatal(err)
	}
	if u, _ := d.GetUser(uid); !u.EmailVerified || u.VerifyToken != "" {
		t.Error("after verify, flag set and token cleared")
	}
	if _, err := d.GetUserByVerifyToken("tok123"); err != ErrNotFound {
		t.Error("token should no longer resolve")
	}
	if _, err := d.GetUserByVerifyToken(""); err != ErrNotFound {
		t.Error("empty token must not match")
	}

	// Invite limits: default role 'user' = 3; override; admin unlimited.
	inviter, _ := d.CreateUser("inv@x.com", "h", false, true, "")
	if lim := d.InviteLimit(inviter); lim != 3 {
		t.Errorf("default invite limit = %d, want 3", lim)
	}
	_ = d.CreateInvitation(inviter, "a@x.com", "t1", "")
	_ = d.CreateInvitation(inviter, "b@x.com", "t2", "")
	if n, _ := d.CountInvitationsBy(inviter); n != 2 {
		t.Errorf("count invitations = %d, want 2", n)
	}
	ov := int64(10)
	_ = d.SetUserInviteOverride(inviter, &ov)
	if lim := d.InviteLimit(inviter); lim != 10 {
		t.Errorf("override invite limit = %d, want 10", lim)
	}
	adm, _ := d.CreateUser("adm@x.com", "h", true, true, "")
	if lim := d.InviteLimit(adm); lim != -1 {
		t.Errorf("admin invite limit = %d, want -1 (unlimited)", lim)
	}

	// Invitation lookup + accept.
	inv, err := d.GetInvitationByToken("t1")
	if err != nil || inv.Email != "a@x.com" || inv.Accepted {
		t.Fatalf("invitation lookup: %+v err=%v", inv, err)
	}
	_ = d.MarkInvitationAccepted("t1")
	if inv2, _ := d.GetInvitationByToken("t1"); !inv2.Accepted {
		t.Error("invitation should be accepted")
	}
}
