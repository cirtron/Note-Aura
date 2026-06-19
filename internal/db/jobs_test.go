package db

import (
	"path/filepath"
	"testing"
)

func TestJobParamsRoundTrip(t *testing.T) {
	d, err := Open(filepath.Join(t.TempDir(), "t.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close()

	uid, err := d.CreateUser("a@b.com", "hash", true, true, "")
	if err != nil {
		t.Fatal(err)
	}
	nid, err := d.CreateNote(&Note{OwnerID: uid, Title: "X", BodyMd: "hi", BodyText: "hi", SourceType: "manual", Status: "processing"})
	if err != nil {
		t.Fatal(err)
	}

	if err := d.EnqueueJob(nid, "process", "summary,tags"); err != nil {
		t.Fatal(err)
	}
	j, err := d.ClaimJob()
	if err != nil {
		t.Fatal(err)
	}
	if j.NoteID != nid {
		t.Errorf("note id = %d, want %d", j.NoteID, nid)
	}
	if j.Params != "summary,tags" {
		t.Errorf("params = %q, want %q", j.Params, "summary,tags")
	}

	// Empty params is allowed (worker treats it as "all").
	if err := d.EnqueueJob(nid, "process", ""); err != nil {
		t.Fatal(err)
	}
	j2, err := d.ClaimJob()
	if err != nil {
		t.Fatal(err)
	}
	if j2.Params != "" {
		t.Errorf("params = %q, want empty", j2.Params)
	}
}
