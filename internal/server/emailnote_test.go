package server

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"note-aura/internal/ai"
	"note-aura/internal/config"
	"note-aura/internal/db"
	"note-aura/internal/emailin"
	"note-aura/internal/worker"
)

func TestHandleInboundEmail(t *testing.T) {
	dir := t.TempDir()
	database, err := db.Open(filepath.Join(dir, "t.db"))
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer database.Close()

	// A role that allows document uploads (so .txt is kept, .mp4 is skipped).
	if err := database.UpsertRole(db.Role{Slug: "docs", Label: "Docs", UploadTypes: "document"}); err != nil {
		t.Fatalf("upsert role: %v", err)
	}
	uid, err := database.CreateUser("u@x.com", "x", false, true, "")
	if err != nil {
		t.Fatalf("create user: %v", err)
	}
	_ = database.SetUserRole(uid, "docs")

	s := &Server{
		cfg:        &config.Config{},
		db:         database,
		uploadsDir: filepath.Join(dir, "uploads"),
		worker:     worker.New(database, ai.GlobalConfig{}, time.Second),
	}

	msg := &emailin.Message{
		Subject: "My subject",
		Text:    "body text here",
		Attachments: []emailin.Attachment{
			{Filename: "a.txt", Mime: "text/plain", Data: []byte("hi")},
			{Filename: "v.mp4", Mime: "video/mp4", Data: []byte("xxxx")}, // not allowed by role
		},
	}
	if err := s.HandleInboundEmail(uid, msg); err != nil {
		t.Fatalf("handle: %v", err)
	}

	notes, _ := database.ListNotes(uid)
	if len(notes) != 1 {
		t.Fatalf("want 1 note, got %d", len(notes))
	}
	n := notes[0]
	if n.Title != "My subject" {
		t.Errorf("title = %q", n.Title)
	}
	if !strings.Contains(n.BodyMd, "body text here") {
		t.Errorf("body missing text: %q", n.BodyMd)
	}
	if !strings.Contains(n.BodyMd, "a.txt") {
		t.Errorf("body missing allowed attachment link: %q", n.BodyMd)
	}
	if strings.Contains(n.BodyMd, "v.mp4") {
		t.Errorf("disallowed attachment should be skipped: %q", n.BodyMd)
	}

	atts, _ := database.AttachmentsForNote(n.ID)
	if len(atts) != 1 {
		t.Fatalf("want 1 saved attachment, got %d", len(atts))
	}
	if _, err := os.Stat(atts[0].Path); err != nil {
		t.Errorf("attachment file not on disk: %v", err)
	}
}

func TestSingleURL(t *testing.T) {
	cases := map[string]string{
		"https://example.com/page":     "https://example.com/page",
		"  http://x.com  ":             "http://x.com",
		"www.example.com":              "www.example.com",
		"https://x.com\nSent from iOS": "", // signature → not URL-only
		"user@example.com":             "", // email address, not a URL
		"just some text":               "",
		"":                             "",
	}
	for in, want := range cases {
		if got := singleURL(in); got != want {
			t.Errorf("singleURL(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestHandleInboundEmail_urlOnly(t *testing.T) {
	dir := t.TempDir()
	database, err := db.Open(filepath.Join(dir, "t.db"))
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer database.Close()
	uid, _ := database.CreateUser("u@x.com", "x", false, true, "")
	s := &Server{
		cfg: &config.Config{}, db: database,
		uploadsDir: filepath.Join(dir, "uploads"),
		worker:     worker.New(database, ai.GlobalConfig{}, time.Second),
	}

	for _, tc := range []struct{ body, wantType, wantRef string }{
		{"https://example.com/article", "url", "https://example.com/article"},
		{"https://youtu.be/abc123", "youtube", "https://youtu.be/abc123"},
		{"www.example.org", "url", "https://www.example.org"}, // scheme prepended
	} {
		if err := s.HandleInboundEmail(uid, &emailin.Message{Subject: "", Text: tc.body}); err != nil {
			t.Fatalf("handle %q: %v", tc.body, err)
		}
	}
	notes, _ := database.ListNotes(uid)
	if len(notes) != 3 {
		t.Fatalf("want 3 notes, got %d", len(notes))
	}
	// Notes list is newest-first; check the set of (type,ref).
	got := map[string]string{}
	for _, n := range notes {
		got[n.SourceRef] = n.SourceType
	}
	want := map[string]string{
		"https://example.com/article": "url",
		"https://youtu.be/abc123":     "youtube",
		"https://www.example.org":     "url",
	}
	for ref, typ := range want {
		if got[ref] != typ {
			t.Errorf("note %q: source_type = %q, want %q", ref, got[ref], typ)
		}
	}
}
