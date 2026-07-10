package server

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"strconv"
	"strings"
	"time"
	"unicode"

	"github.com/gofiber/fiber/v2"

	"note-aura/internal/db"
)

// bulkDeleteNotes soft-deletes the selected notes that the user owns.
func (s *Server) bulkDeleteNotes(c *fiber.Ctx) error {
	u := currentUser(c)
	var ids []string
	if form, err := c.MultipartForm(); err == nil {
		ids = form.Value["ids"]
	}
	for _, raw := range ids {
		id, err := strconv.ParseInt(raw, 10, 64)
		if err != nil {
			continue
		}
		if n, err := s.db.GetNote(id); err == nil && n.OwnerID == u.ID {
			_ = s.db.SoftDeleteNote(id)
		}
	}
	return c.Redirect("/notes", fiber.StatusFound)
}

// ----- import / export -----

const maxImportBytes = 50 << 20 // 50 MB

type exportNote struct {
	Title       string   `json:"title"`
	BodyMd      string   `json:"body_md"`
	Summary     string   `json:"summary,omitempty"`
	SourceType  string   `json:"source_type,omitempty"`
	SourceRef   string   `json:"source_ref,omitempty"`
	SummaryLang string   `json:"summary_lang,omitempty"`
	Category    string   `json:"category,omitempty"`
	Tags        []string `json:"tags,omitempty"`
	EventDate   string   `json:"event_date,omitempty"`
	StartTime   string   `json:"start_time,omitempty"`
	EndTime     string   `json:"end_time,omitempty"`
	AllDay      bool     `json:"all_day,omitempty"`
	CreatedAt   string   `json:"created_at,omitempty"`
}

type exportFile struct {
	Version    int          `json:"version"`
	ExportedAt string       `json:"exported_at"`
	Notes      []exportNote `json:"notes"`
}

// exportNotes streams all of the user's notes as a JSON file download.
func (s *Server) exportNotes(c *fiber.Ctx) error {
	u := currentUser(c)
	list, _ := s.db.ListNotes(u.ID)
	out := exportFile{Version: 1, ExportedAt: time.Now().Format(time.RFC3339)}
	for _, item := range list {
		n, err := s.db.GetNote(item.ID)
		if err != nil {
			continue
		}
		out.Notes = append(out.Notes, exportNote{
			Title: n.Title, BodyMd: n.BodyMd, Summary: n.Summary,
			SourceType: n.SourceType, SourceRef: n.SourceRef, SummaryLang: n.SummaryLang,
			Category: n.CategoryName, Tags: n.Tags,
			EventDate: n.EventDate, StartTime: n.StartTime, EndTime: n.EndTime, AllDay: n.AllDay,
			CreatedAt: n.CreatedAt.Format(time.RFC3339),
		})
	}
	data, err := json.MarshalIndent(out, "", "  ")
	if err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, err.Error())
	}
	name := "note-aura-export-" + time.Now().Format("20060102") + ".json"
	c.Set("Content-Type", "application/json")
	c.Set("Content-Disposition", `attachment; filename="`+name+`"`)
	return c.Send(data)
}

// exportMarkdownZip streams all of the user's notes as a ZIP of Markdown files.
func (s *Server) exportMarkdownZip(c *fiber.Ctx) error {
	u := currentUser(c)
	list, _ := s.db.ListNotes(u.ID)

	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	for i, item := range list {
		n, err := s.db.GetNote(item.ID)
		if err != nil {
			continue
		}
		fname := fmt.Sprintf("%04d-%s.md", i+1, mdFilename(n.Title, n.ID))
		f, err := zw.Create(fname)
		if err != nil {
			continue
		}
		fmt.Fprint(f, buildMarkdownNote(n))
	}
	_ = zw.Close()

	name := "note-aura-md-" + time.Now().Format("20060102") + ".zip"
	c.Set("Content-Type", "application/zip")
	c.Set("Content-Disposition", `attachment; filename="`+name+`"`)
	return c.Send(buf.Bytes())
}

// mdFilename returns a filesystem-safe base name (no extension) for a note.
func mdFilename(title string, id int64) string {
	safe := strings.Map(func(r rune) rune {
		if unicode.IsLetter(r) || unicode.IsDigit(r) || r == '-' || r == '_' || r == '.' || r == ' ' {
			return r
		}
		return '_'
	}, title)
	safe = strings.TrimSpace(safe)
	if safe == "" {
		return fmt.Sprintf("note-%d", id)
	}
	if len(safe) > 60 {
		safe = strings.TrimSpace(safe[:60])
	}
	return safe
}

// buildMarkdownNote formats a note as Markdown with YAML frontmatter.
func buildMarkdownNote(n *db.Note) string {
	var sb strings.Builder
	sb.WriteString("---\n")
	sb.WriteString(fmt.Sprintf("title: %q\n", n.Title))
	if len(n.Tags) > 0 {
		sb.WriteString(fmt.Sprintf("tags: [%s]\n", strings.Join(n.Tags, ", ")))
	}
	if n.CategoryName != "" {
		sb.WriteString(fmt.Sprintf("category: %q\n", n.CategoryName))
	}
	if n.SourceType != "" && n.SourceType != "manual" {
		sb.WriteString(fmt.Sprintf("source_type: %s\n", n.SourceType))
	}
	if n.SourceRef != "" {
		sb.WriteString(fmt.Sprintf("source_ref: %q\n", n.SourceRef))
	}
	sb.WriteString(fmt.Sprintf("created: %s\n", n.CreatedAt.Format("2006-01-02")))
	sb.WriteString("---\n\n")
	if n.Summary != "" {
		sb.WriteString("> " + n.Summary + "\n\n")
	}
	sb.WriteString(n.BodyMd)
	if !strings.HasSuffix(n.BodyMd, "\n") {
		sb.WriteByte('\n')
	}
	return sb.String()
}

// importNotes creates notes from an uploaded export file. Notes come in as plain
// content (no AI re-processing); tags, category, and schedule are preserved.
func (s *Server) importNotes(c *fiber.Ctx) error {
	u := currentUser(c)
	fh, err := c.FormFile("file")
	if err != nil {
		return c.Redirect("/settings?imperr=file", fiber.StatusFound)
	}
	f, err := fh.Open()
	if err != nil {
		return c.Redirect("/settings?imperr=file", fiber.StatusFound)
	}
	defer f.Close()
	data, err := io.ReadAll(io.LimitReader(f, maxImportBytes))
	if err != nil {
		return c.Redirect("/settings?imperr=file", fiber.StatusFound)
	}
	var in exportFile
	if err := json.Unmarshal(data, &in); err != nil || in.Notes == nil {
		return c.Redirect("/settings?imperr=format", fiber.StatusFound)
	}

	imported := 0
	for _, en := range in.Notes {
		if en.BodyMd == "" && en.Title == "" {
			continue
		}
		if s.overCapacity(u, int64(len(en.BodyMd))) {
			return c.Redirect("/settings?imperr=capacity&imported="+strconv.Itoa(imported), fiber.StatusFound)
		}
		note := &db.Note{
			OwnerID: u.ID, Title: en.Title, BodyMd: en.BodyMd, BodyText: en.BodyMd,
			Summary: en.Summary, SourceType: "manual", SourceRef: en.SourceRef,
			Status: "ready", SummaryLang: en.SummaryLang,
		}
		id, err := s.db.CreateNote(note)
		if err != nil {
			continue
		}
		if len(en.Tags) > 0 {
			_ = s.db.SetNoteTags(u.ID, id, "manual", en.Tags)
		}
		if en.Category != "" {
			if cid, err := s.db.UpsertCategory(u.ID, en.Category); err == nil && cid > 0 {
				_ = s.db.SetNoteCategory(id, &cid)
			}
		}
		if en.EventDate != "" {
			_ = s.db.SetNoteSchedule(id, en.EventDate, en.StartTime, en.EndTime, en.AllDay, nil)
		}
		imported++
	}
	return c.Redirect("/settings?imported="+strconv.Itoa(imported), fiber.StatusFound)
}
