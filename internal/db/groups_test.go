package db

import (
	"path/filepath"
	"testing"
)

func TestGroupAccessControl(t *testing.T) {
	d, err := Open(filepath.Join(t.TempDir(), "t.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close()

	owner, _ := d.CreateUser("owner@x.com", "h", false, true, "")
	rw, _ := d.CreateUser("rw@x.com", "h", false, true, "")
	ro, _ := d.CreateUser("ro@x.com", "h", false, true, "")
	outsider, _ := d.CreateUser("out@x.com", "h", false, true, "")

	nid, _ := d.CreateNote(&Note{OwnerID: owner, Title: "N", BodyMd: "hi", BodyText: "hi", SourceType: "manual", Status: "ready"})
	gid, _ := d.CreateGroup(owner, "Team")
	if err := d.AddGroupMember(gid, "rw@x.com", true); err != nil {
		t.Fatal(err)
	}
	if err := d.AddGroupMember(gid, "ro@x.com", false); err != nil {
		t.Fatal(err)
	}
	if err := d.ShareNoteToGroup(nid, gid); err != nil {
		t.Fatal(err)
	}

	check := func(name string, uid int64, wantR, wantE bool) {
		r, e, _ := d.NoteAccess(nid, uid)
		if r != wantR || e != wantE {
			t.Errorf("%s: access=(read=%v,edit=%v), want (read=%v,edit=%v)", name, r, e, wantR, wantE)
		}
	}
	check("owner", owner, true, true)
	check("member read/write", rw, true, true)
	check("member read-only", ro, true, false)
	check("outsider", outsider, false, false)

	// RAG scope: members see the note's chunks; outsider does not.
	if err := d.ReplaceChunks(nid, []Chunk{{NoteID: nid, Index: 0, Text: "x", Embedding: []byte{1, 2, 3, 4}}}); err != nil {
		t.Fatal(err)
	}
	has := func(uid int64) bool { cs, _ := d.ChunksAccessibleBy(uid); return len(cs) > 0 }
	if !has(rw) || !has(ro) {
		t.Error("group members should see shared-note chunks in RAG scope")
	}
	if has(outsider) {
		t.Error("outsider must NOT see shared-note chunks")
	}

	// Shared-with-me lists the note for members but not the outsider.
	listed := func(uid int64) bool {
		ns, _ := d.ListSharedWithUser(uid)
		for _, n := range ns {
			if n.ID == nid {
				return true
			}
		}
		return false
	}
	if !listed(rw) || !listed(ro) {
		t.Error("group members should see the note under Shared with me")
	}
	if listed(outsider) {
		t.Error("outsider must not see the note under Shared with me")
	}

	// Removing a member revokes access.
	_ = d.RemoveGroupMember(gid, ro)
	if r, _, _ := d.NoteAccess(nid, ro); r {
		t.Error("removed member should lose access")
	}
}
