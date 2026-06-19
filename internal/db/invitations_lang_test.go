package db

import "testing"

func TestInvitationLangRoundTrip(t *testing.T) {
	d, err := Open(":memory:")
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer d.Close()

	uid, err := d.CreateUser("inviter@example.com", "hash", true, true, "")
	if err != nil {
		t.Fatalf("create user: %v", err)
	}
	if err := d.CreateInvitation(uid, "new@example.com", "tok-1", "zh-Hant"); err != nil {
		t.Fatalf("create invitation: %v", err)
	}

	byTok, err := d.GetInvitationByToken("tok-1")
	if err != nil {
		t.Fatalf("get by token: %v", err)
	}
	if byTok.Lang != "zh-Hant" {
		t.Errorf("GetInvitationByToken lang = %q, want zh-Hant", byTok.Lang)
	}

	byID, err := d.GetInvitation(byTok.ID)
	if err != nil {
		t.Fatalf("get by id: %v", err)
	}
	if byID.Lang != "zh-Hant" {
		t.Errorf("GetInvitation lang = %q, want zh-Hant", byID.Lang)
	}

	if err := d.CreateInvitation(uid, "two@example.com", "tok-2", ""); err != nil {
		t.Fatalf("create invitation empty lang: %v", err)
	}
	inv2, _ := d.GetInvitationByToken("tok-2")
	if inv2.Lang != "" {
		t.Errorf("empty lang = %q, want \"\"", inv2.Lang)
	}

	// ListInvitationsBy — scan path must also carry lang.
	byInviter, err := d.ListInvitationsBy(uid)
	if err != nil {
		t.Fatalf("ListInvitationsBy: %v", err)
	}
	foundByInviter := false
	for _, inv := range byInviter {
		if inv.Token == "tok-1" {
			foundByInviter = true
			if inv.Lang != "zh-Hant" {
				t.Errorf("ListInvitationsBy tok-1 lang = %q, want zh-Hant", inv.Lang)
			}
		}
	}
	if !foundByInviter {
		t.Error("ListInvitationsBy: tok-1 not found in results")
	}

	// ListAllInvitations — admin scan path must also carry lang.
	all, err := d.ListAllInvitations()
	if err != nil {
		t.Fatalf("ListAllInvitations: %v", err)
	}
	foundInAll := false
	for _, inv := range all {
		if inv.Token == "tok-1" {
			foundInAll = true
			if inv.Lang != "zh-Hant" {
				t.Errorf("ListAllInvitations tok-1 lang = %q, want zh-Hant", inv.Lang)
			}
		}
	}
	if !foundInAll {
		t.Error("ListAllInvitations: tok-1 not found in results")
	}
}
