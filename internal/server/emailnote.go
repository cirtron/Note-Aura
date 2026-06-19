package server

import (
	"fmt"
	"path/filepath"
	"strconv"
	"strings"

	"note-aura/internal/db"
	"note-aura/internal/emailin"
	"note-aura/internal/ingest"
)

// HandleInboundEmail turns a parsed inbound email into a note for the resolved
// user, mirroring a manual capture: the subject is the title, the body text is
// the note body, and allowed attachments are saved and linked into the body.
// Satisfies emailin.Handler.
func (s *Server) HandleInboundEmail(userID int64, m *emailin.Message) error {
	u, err := s.db.GetUser(userID)
	if err != nil {
		return err
	}
	title := strings.TrimSpace(m.Subject)
	body := strings.TrimSpace(m.Text)

	// If the email is just a link (and has no attachments), capture the linked
	// page/video instead of storing the bare URL — reuses the url/youtube pipeline.
	if len(m.Attachments) == 0 {
		if raw := singleURL(body); raw != "" {
			return s.captureEmailURL(userID, title, raw)
		}
	}

	// Stored as a "manual" note (the notes table constrains source_type); the
	// email origin is recorded in source_ref.
	note := &db.Note{
		OwnerID: userID, Title: title, BodyMd: body, BodyText: body,
		SourceType: "manual", SourceRef: "email", Status: "processing",
	}
	id, err := s.db.CreateNote(note)
	if err != nil {
		return err
	}

	// Save attachments the user's role permits and within their capacity, linking
	// each into the body (images inline, other files as download links).
	var links []string
	for _, att := range m.Attachments {
		ext := normalizeExt(filepath.Ext(att.Filename))
		if !s.userCanUploadExt(u, ext) || s.overCapacity(u, int64(len(att.Data))) {
			continue
		}
		dest, err := s.saveNoteBytes(id, att.Filename, att.Data)
		if err != nil {
			continue
		}
		if _, err := s.db.CreateAttachment(id, dest, att.Mime, int64(len(att.Data))); err != nil {
			continue
		}
		rel := "/uploads/notes/" + strconv.FormatInt(id, 10) + "/" + filepath.Base(dest)
		if extCategory(ext) == "image" {
			links = append(links, fmt.Sprintf("![%s](%s)", att.Filename, rel))
		} else {
			links = append(links, fmt.Sprintf("[%s](%s)", att.Filename, rel))
		}
	}
	if len(links) > 0 {
		body = strings.TrimSpace(body + "\n\n" + strings.Join(links, "\n\n"))
		_ = s.db.UpdateNoteContent(id, title, body, body)
	}

	// Nothing to summarize: mark ready with whatever title we have.
	if strings.TrimSpace(body) == "" {
		if title == "" {
			title = "(untitled)"
		}
		return s.db.ApplyAIResult(id, title, "", "", "")
	}

	// AI organize: keep the subject as the title when present.
	parts := "title,summary,tags,category"
	if title != "" {
		parts = "summary,tags,category"
	}
	if err := s.db.EnqueueJob(id, "process", parts); err != nil {
		return err
	}
	s.worker.Kick()
	return nil
}

// captureEmailURL creates a url/youtube note from a link-only email; the worker
// fetches and extracts the page/video content. The subject is kept as the title
// when present, otherwise one is generated from the fetched content.
func (s *Server) captureEmailURL(userID int64, title, rawURL string) error {
	if !strings.HasPrefix(rawURL, "http://") && !strings.HasPrefix(rawURL, "https://") {
		rawURL = "https://" + rawURL
	}
	sourceType := "url"
	if ingest.IsYouTube(rawURL) {
		sourceType = "youtube"
	}
	note := &db.Note{
		OwnerID: userID, Title: title, SourceType: sourceType, SourceRef: rawURL, Status: "processing",
	}
	id, err := s.db.CreateNote(note)
	if err != nil {
		return err
	}
	parts := "title,summary,tags,category"
	if title != "" {
		parts = "summary,tags,category"
	}
	if err := s.db.EnqueueJob(id, "process", parts); err != nil {
		return err
	}
	s.worker.Kick()
	return nil
}

// singleURL returns the URL when text is exactly one link (ignoring surrounding
// whitespace), or "" otherwise. Requires an http(s):// or www. prefix so it
// doesn't match email addresses or ordinary words.
func singleURL(text string) string {
	fields := strings.Fields(text)
	if len(fields) != 1 {
		return ""
	}
	s := fields[0]
	if strings.HasPrefix(s, "http://") || strings.HasPrefix(s, "https://") || strings.HasPrefix(s, "www.") {
		return s
	}
	return ""
}
