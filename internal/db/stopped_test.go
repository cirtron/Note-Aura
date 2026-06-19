package db

import "testing"

func TestStopNoteAndRetryClears(t *testing.T) {
	d, err := Open(":memory:")
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer d.Close()

	uid, _ := d.CreateUser("u@example.com", "h", true, true, "")
	nid, err := d.CreateNote(&Note{OwnerID: uid, Title: "X", BodyMd: "hi", BodyText: "hi", SourceType: "url", Status: "processing"})
	if err != nil {
		t.Fatalf("create note: %v", err)
	}
	if err := d.EnqueueJob(nid, "process", "summary,tags"); err != nil {
		t.Fatalf("enqueue: %v", err)
	}

	if err := d.StopNote(nid); err != nil {
		t.Fatalf("stop: %v", err)
	}
	n, _ := d.GetNote(nid)
	if n.Status != "failed" || !n.Stopped {
		t.Fatalf("after stop: status=%q stopped=%v, want failed/true", n.Status, n.Stopped)
	}

	if err := d.DeleteJobsForNote(nid); err != nil {
		t.Fatalf("delete jobs: %v", err)
	}
	if _, err := d.ClaimJob(); err != ErrNotFound {
		t.Fatalf("expected no claimable job after delete, got %v", err)
	}

	if err := d.ClearStopped(nid); err != nil {
		t.Fatalf("clear: %v", err)
	}
	n, _ = d.GetNote(nid)
	if n.Stopped {
		t.Fatalf("ClearStopped left stopped=true")
	}
}
