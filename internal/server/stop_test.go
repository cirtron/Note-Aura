package server

import (
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v2"

	"note-aura/internal/db"
)

func TestStopNoteHandler(t *testing.T) {
	database, err := db.Open(":memory:")
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { database.Close() })

	uid, _ := database.CreateUser("u@example.com", "h", true, true, "")
	nid, _ := database.CreateNote(&db.Note{OwnerID: uid, Title: "X", BodyMd: "hi", BodyText: "hi", SourceType: "url", Status: "processing"})
	_ = database.EnqueueJob(nid, "process", "summary")
	user, _ := database.GetUser(uid)

	s := &Server{db: database, worker: nil}

	app := fiber.New()
	app.Use(func(c *fiber.Ctx) error { c.Locals(userLocalKey, user); return c.Next() })
	app.Post("/notes/:id/stop", s.stopNote)

	req := httptest.NewRequest("POST", "/notes/1/stop", nil)
	resp, _ := app.Test(req)
	if resp.StatusCode != fiber.StatusFound {
		t.Fatalf("status = %d, want 302", resp.StatusCode)
	}
	n, _ := database.GetNote(nid)
	if n.Status != "failed" || !n.Stopped {
		t.Fatalf("note not stopped: status=%q stopped=%v", n.Status, n.Stopped)
	}
	if _, err := database.ClaimJob(); err != db.ErrNotFound {
		t.Fatalf("jobs not deleted after stop")
	}
}
