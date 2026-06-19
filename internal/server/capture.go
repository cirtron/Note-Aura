package server

import (
	"fmt"
	"io"
	"mime/multipart"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/gofiber/fiber/v2"

	"note-aura/internal/db"
	"note-aura/internal/ingest"
)

// maxUploadTextBytes caps how much of a text/markdown upload is read into the
// note body.
const maxUploadTextBytes = 1 << 20 // 1 MB

// capture handles non-manual sources: a URL (auto-detected as web or YouTube),
// or an uploaded image/file. The note is created in "processing" state and
// filled in by the worker (or, for binary files, immediately marked ready).
func (s *Server) capture(c *fiber.Ctx) error {
	u := currentUser(c)
	kind := c.FormValue("kind")

	switch kind {
	case "url":
		raw := strings.TrimSpace(c.FormValue("url"))
		if raw == "" {
			return c.Redirect("/notes/new", fiber.StatusFound)
		}
		if !strings.HasPrefix(raw, "http://") && !strings.HasPrefix(raw, "https://") {
			raw = "https://" + raw
		}
		sourceType := "url"
		if ingest.IsYouTube(raw) {
			sourceType = "youtube"
		}
		note := &db.Note{
			OwnerID:     u.ID,
			Title:       "",
			SourceType:  sourceType,
			SourceRef:   raw,
			Status:      "processing",
			SummaryLang: summaryLang(c),
		}
		id, err := s.db.CreateNote(note)
		if err != nil {
			return fiber.NewError(fiber.StatusInternalServerError, err.Error())
		}
		_ = s.db.EnqueueJob(id, "process", "title,summary,tags,category")
		s.worker.Kick()
		return c.Redirect("/notes/"+strconv.FormatInt(id, 10), fiber.StatusFound)

	case "image":
		fh, err := c.FormFile("image")
		if err != nil {
			return c.Redirect("/notes/new", fiber.StatusFound)
		}
		if !s.userCanUploadExt(u, filepath.Ext(fh.Filename)) {
			return c.Redirect("/notes/new?error=filetype", fiber.StatusFound)
		}
		if s.overCapacity(u, fh.Size) {
			return c.Redirect("/notes/new?error=capacity", fiber.StatusFound)
		}
		return s.captureImage(c, u, fh, summaryLang(c))

	case "file":
		return s.captureFile(c, u)

	default:
		return fiber.NewError(fiber.StatusBadRequest, fmt.Sprintf("unknown capture kind %q", kind))
	}
}

// captureFile stores an uploaded file as a note, routing by extension: images go
// through OCR, text/markdown files are read in as the note body, and everything
// else is stored as a downloadable attachment with no AI.
func (s *Server) captureFile(c *fiber.Ctx, u *db.User) error {
	fh, err := c.FormFile("file")
	if err != nil {
		return c.Redirect("/notes/new", fiber.StatusFound)
	}
	ext := normalizeExt(filepath.Ext(fh.Filename))
	if !s.userCanUploadExt(u, ext) {
		return c.Redirect("/notes/new?error=filetype", fiber.StatusFound)
	}
	if s.overCapacity(u, fh.Size) {
		return c.Redirect("/notes/new?error=capacity", fiber.StatusFound)
	}
	lang := summaryLang(c)

	switch {
	case extCategory(ext) == "image":
		return s.captureImage(c, u, fh, lang)

	case textExts[ext]:
		data, err := readMultipart(fh, maxUploadTextBytes)
		if err != nil {
			return fiber.NewError(fiber.StatusInternalServerError, err.Error())
		}
		body := string(data)
		note := &db.Note{
			OwnerID: u.ID, Title: fh.Filename, BodyMd: body, BodyText: body,
			SourceType: "manual", SourceRef: fh.Filename, Status: "processing", SummaryLang: lang,
		}
		id, err := s.db.CreateNote(note)
		if err != nil {
			return fiber.NewError(fiber.StatusInternalServerError, err.Error())
		}
		_ = s.db.EnqueueJob(id, "process", "title,summary,tags,category")
		s.worker.Kick()
		return c.Redirect("/notes/"+strconv.FormatInt(id, 10), fiber.StatusFound)

	default: // binary: store as a downloadable attachment, ready immediately
		// Stored as "manual" (the notes table constrains source_type).
		note := &db.Note{
			OwnerID: u.ID, Title: fh.Filename, SourceType: "manual",
			SourceRef: fh.Filename, Status: "processing", SummaryLang: lang,
		}
		id, err := s.db.CreateNote(note)
		if err != nil {
			return fiber.NewError(fiber.StatusInternalServerError, err.Error())
		}
		dest, err := s.saveNoteUpload(id, fh)
		if err != nil {
			return fiber.NewError(fiber.StatusInternalServerError, err.Error())
		}
		if _, err := s.db.CreateAttachment(id, dest, fh.Header.Get("Content-Type"), fh.Size); err != nil {
			return fiber.NewError(fiber.StatusInternalServerError, err.Error())
		}
		rel := "/uploads/notes/" + strconv.FormatInt(id, 10) + "/" + filepath.Base(dest)
		body := fmt.Sprintf("**Uploaded file:** [%s](%s)  \n_%s · %.1f MB_",
			fh.Filename, rel, extCategory(ext), float64(fh.Size)/bytesPerMB)
		_ = s.db.ApplyAIResult(id, fh.Filename, "", body, fh.Filename)
		return c.Redirect("/notes/"+strconv.FormatInt(id, 10), fiber.StatusFound)
	}
}

// captureImage creates an image note (OCR'd by the worker) from an uploaded file.
func (s *Server) captureImage(c *fiber.Ctx, u *db.User, fh *multipart.FileHeader, lang string) error {
	note := &db.Note{
		OwnerID: u.ID, Title: "", SourceType: "image",
		SourceRef: fh.Filename, Status: "processing", SummaryLang: lang,
	}
	id, err := s.db.CreateNote(note)
	if err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, err.Error())
	}
	dest, err := s.saveNoteUpload(id, fh)
	if err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, err.Error())
	}
	if _, err := s.db.CreateAttachment(id, dest, fh.Header.Get("Content-Type"), fh.Size); err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, err.Error())
	}
	_ = s.db.EnqueueJob(id, "process", "title,summary,tags,category")
	s.worker.Kick()
	return c.Redirect("/notes/"+strconv.FormatInt(id, 10), fiber.StatusFound)
}

// saveNoteUpload writes an uploaded file under uploads/notes/<id>/ and returns
// the destination path.
func (s *Server) saveNoteUpload(noteID int64, fh *multipart.FileHeader) (string, error) {
	dir := filepath.Join(s.uploadsDir, "notes", strconv.FormatInt(noteID, 10))
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	dest := filepath.Join(dir, sanitizeFilename(fh.Filename))
	src, err := fh.Open()
	if err != nil {
		return "", err
	}
	defer src.Close()
	out, err := os.Create(dest)
	if err != nil {
		return "", err
	}
	defer out.Close()
	if _, err := io.Copy(out, src); err != nil {
		return "", err
	}
	return dest, nil
}

// saveNoteBytes writes in-memory bytes (e.g. an email attachment) under
// uploads/notes/<id>/ and returns the destination path.
func (s *Server) saveNoteBytes(noteID int64, filename string, data []byte) (string, error) {
	dir := filepath.Join(s.uploadsDir, "notes", strconv.FormatInt(noteID, 10))
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	dest := filepath.Join(dir, sanitizeFilename(filename))
	if err := os.WriteFile(dest, data, 0o644); err != nil {
		return "", err
	}
	return dest, nil
}

// readMultipart reads up to max bytes from an uploaded file.
func readMultipart(fh *multipart.FileHeader, max int64) ([]byte, error) {
	f, err := fh.Open()
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return io.ReadAll(io.LimitReader(f, max))
}

// sanitizeFilename keeps a safe basename for stored uploads.
func sanitizeFilename(name string) string {
	name = filepath.Base(name)
	name = strings.ReplaceAll(name, " ", "_")
	var b strings.Builder
	for _, r := range name {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9',
			r == '.', r == '_', r == '-':
			b.WriteRune(r)
		}
	}
	out := b.String()
	if out == "" || out == "." {
		out = "upload"
	}
	return out
}
