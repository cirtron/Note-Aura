package server

import (
	"strconv"
	"strings"

	"github.com/gofiber/fiber/v2"

	"note-aura/internal/db"
)

func (s *Server) listNotes(c *fiber.Ctx) error {
	u := currentUser(c)
	q := strings.TrimSpace(c.Query("q"))
	category := strings.TrimSpace(c.Query("category"))
	tag := strings.TrimSpace(c.Query("tag"))

	var (
		notes []*db.Note
		err   error
	)
	if q != "" {
		notes, err = s.db.SearchNotes(u.ID, q)
	} else {
		notes, err = s.db.ListNotes(u.ID)
	}
	if err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, err.Error())
	}

	// Apply category / tag filters. Selecting a category also includes its
	// sub-categories (path prefix, e.g. "Work" matches "Work/Project A").
	if category != "" || tag != "" {
		var f []*db.Note
		for _, n := range notes {
			if category != "" && !categoryMatches(n.CategoryName, category) {
				continue
			}
			if tag != "" && !hasTag(n.Tags, tag) {
				continue
			}
			f = append(f, n)
		}
		notes = f
	}

	// Sorting and page size persist per user; a query param updates them.
	settings, _ := s.db.GetUserSettings(u.ID)
	sortKey := validSort(firstNonEmptyStr(c.Query("sort"), settings["notes_sort"]))
	if c.Query("sort") != "" {
		_ = s.db.SetUserSetting(u.ID, "notes_sort", sortKey)
	}
	per := validPerPage(firstNonEmptyStr(c.Query("per"), settings["notes_per_page"]))
	if c.Query("per") != "" {
		_ = s.db.SetUserSetting(u.ID, "notes_per_page", strconv.Itoa(per))
	}
	sortNotes(notes, sortKey)

	// Paginate.
	total := len(notes)
	totalPages := (total + per - 1) / per
	if totalPages < 1 {
		totalPages = 1
	}
	page, _ := strconv.Atoi(c.Query("page", "1"))
	if page < 1 {
		page = 1
	}
	if page > totalPages {
		page = totalPages
	}
	start := (page - 1) * per
	end := start + per
	if start > total {
		start = total
	}
	if end > total {
		end = total
	}
	pageNotes := notes[start:end]

	tags, _ := s.db.TagsWithCounts(u.ID)

	m := baseMap(c, "Your notes")
	m["Notes"] = pageNotes
	m["Query"] = q
	m["CatTree"] = s.buildCatTree(u.ID)
	m["TagCounts"] = tags
	m["ActiveCategory"] = category
	m["ActiveTag"] = tag
	m["HasFilter"] = category != "" || tag != "" || q != ""
	m["Sort"] = sortKey
	m["SortOptions"] = noteSortOptions
	m["Per"] = per
	m["PerOptions"] = perPageOptions
	m["Page"] = page
	m["TotalPages"] = totalPages
	m["Total"] = total
	if page > 1 {
		m["PrevURL"] = notesURL(q, category, tag, sortKey, per, page-1)
	}
	if page < totalPages {
		m["NextURL"] = notesURL(q, category, tag, sortKey, per, page+1)
	}
	m["Nav"] = "notes"
	return c.Render("notes_list", m, "layout")
}

func hasTag(tags []string, want string) bool {
	for _, t := range tags {
		if strings.EqualFold(t, want) {
			return true
		}
	}
	return false
}

func (s *Server) newNoteForm(c *fiber.Ctx) error {
	m := baseMap(c, "New note")
	m["Nav"] = "new"
	m["Categories"], _ = s.db.CategoriesWithCounts(currentUser(c).ID)
	m["ReminderVal"] = ""
	m["CapacityError"] = c.Query("error") == "capacity"
	m["FileTypeError"] = c.Query("error") == "filetype"
	accept, hint, canUpload := s.userUploadInfo(currentUser(c))
	m["UploadAccept"] = accept
	m["UploadHint"] = hint
	m["CanUploadFile"] = canUpload
	return c.Render("note_edit", m, "layout")
}

// createNote handles a manual Markdown note. AI auto-organize runs in the
// background unless the user typed a title (we still summarize + embed).
func (s *Server) createNote(c *fiber.Ctx) error {
	u := currentUser(c)
	title := strings.TrimSpace(c.FormValue("title"))
	bodyMd := strings.TrimSpace(c.FormValue("body_md"))
	if bodyMd == "" && title == "" {
		return c.Redirect("/notes/new", fiber.StatusFound)
	}
	if s.overCapacity(u, int64(len(bodyMd))) {
		return c.Redirect("/notes/new?error=capacity", fiber.StatusFound)
	}
	// Users whose role lacks AI keep their notes, just without AI processing.
	useAI := canUseAI(c)
	status := "processing"
	if !useAI {
		status = "ready"
	}
	note := &db.Note{
		OwnerID:     u.ID,
		Title:       title,
		BodyMd:      bodyMd,
		BodyText:    bodyMd,
		SourceType:  "manual",
		Status:      status,
		SummaryLang: summaryLang(c),
	}
	id, err := s.db.CreateNote(note)
	if err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, err.Error())
	}
	// Save any manual tags immediately.
	if tags := splitTags(c.FormValue("tags")); len(tags) > 0 {
		_ = s.db.SetNoteTags(u.ID, id, "manual", tags)
	}
	// Assign category if provided (auto-created for this user).
	if cat := strings.TrimSpace(c.FormValue("category")); cat != "" {
		if cid, err := s.db.UpsertCategory(u.ID, cat); err == nil && cid > 0 {
			_ = s.db.SetNoteCategory(id, &cid)
		}
	}
	s.saveSchedule(c, id)
	if useAI {
		// Generate only the fields the user left empty: a title if blank, a summary
		// (no manual summary on the form), and tags unless the user typed some.
		parts := []string{"summary"}
		if title == "" {
			parts = append([]string{"title"}, parts...)
		}
		if len(splitTags(c.FormValue("tags"))) == 0 {
			parts = append(parts, "tags")
		}
		if strings.TrimSpace(c.FormValue("category")) == "" {
			parts = append(parts, "category")
		}
		if err := s.db.EnqueueJob(id, "process", strings.Join(parts, ",")); err != nil {
			return fiber.NewError(fiber.StatusInternalServerError, err.Error())
		}
		s.worker.Kick()
	}
	return c.Redirect("/notes/"+strconv.FormatInt(id, 10), fiber.StatusFound)
}

func (s *Server) viewNote(c *fiber.Ctx) error {
	u := currentUser(c)
	id, _ := strconv.ParseInt(c.Params("id"), 10, 64)
	canRead, canEdit, err := s.db.NoteAccess(id, u.ID)
	if err == db.ErrNotFound || !canRead {
		return fiber.NewError(fiber.StatusNotFound, "note not found")
	}
	if err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, err.Error())
	}
	note, err := s.db.GetNote(id)
	if err != nil {
		return fiber.NewError(fiber.StatusNotFound, "note not found")
	}
	shares, _ := s.db.SharesForNote(id)
	m := baseMap(c, note.Title)
	m["Note"] = note
	m["CanEdit"] = canEdit
	m["IsOwner"] = note.OwnerID == u.ID
	m["Shares"] = shares
	if note.OwnerID == u.ID {
		m["GroupShares"], _ = s.db.GroupSharesForNote(id)
		m["ShareGroups"], _ = s.db.GroupsUserCanShareTo(u.ID)
	}
	m["Nav"] = "notes"
	return c.Render("note_view", m, "layout")
}

// noteStatus is polled by the UI while a note is processing.
func (s *Server) noteStatus(c *fiber.Ctx) error {
	u := currentUser(c)
	id, _ := strconv.ParseInt(c.Params("id"), 10, 64)
	if canRead, _, err := s.db.NoteAccess(id, u.ID); err != nil || !canRead {
		return fiber.NewError(fiber.StatusNotFound, "note not found")
	}
	note, err := s.db.GetNote(id)
	if err != nil {
		return fiber.NewError(fiber.StatusNotFound, "note not found")
	}
	return c.JSON(fiber.Map{
		"status":  note.Status,
		"title":   note.Title,
		"summary": note.Summary,
		"error":   note.Error,
		"tags":    note.Tags,
	})
}

func (s *Server) editNoteForm(c *fiber.Ctx) error {
	u := currentUser(c)
	id, _ := strconv.ParseInt(c.Params("id"), 10, 64)
	_, canEdit, err := s.db.NoteAccess(id, u.ID)
	if err == db.ErrNotFound {
		return fiber.NewError(fiber.StatusNotFound, "note not found")
	}
	if err != nil || !canEdit {
		return fiber.NewError(fiber.StatusForbidden, "you cannot edit this note")
	}
	note, err := s.db.GetNote(id)
	if err != nil {
		return fiber.NewError(fiber.StatusNotFound, "note not found")
	}
	m := baseMap(c, "Edit note")
	m["Note"] = note
	m["Editing"] = true
	m["Categories"], _ = s.db.CategoriesWithCounts(note.OwnerID)
	m["ReminderVal"] = ""
	if note.ReminderMinutes.Valid {
		m["ReminderVal"] = strconv.FormatInt(note.ReminderMinutes.Int64, 10)
	}
	m["CapacityError"] = c.Query("error") == "capacity"
	m["Nav"] = "notes"
	return c.Render("note_edit", m, "layout")
}

func (s *Server) updateNote(c *fiber.Ctx) error {
	u := currentUser(c)
	id, _ := strconv.ParseInt(c.Params("id"), 10, 64)
	_, canEdit, err := s.db.NoteAccess(id, u.ID)
	if err == db.ErrNotFound {
		return fiber.NewError(fiber.StatusNotFound, "note not found")
	}
	if err != nil || !canEdit {
		return fiber.NewError(fiber.StatusForbidden, "you cannot edit this note")
	}
	note, err := s.db.GetNote(id)
	if err != nil {
		return fiber.NewError(fiber.StatusNotFound, "note not found")
	}
	title := strings.TrimSpace(c.FormValue("title"))
	bodyMd := strings.TrimSpace(c.FormValue("body_md"))
	// Enforce the owner's storage limit on any growth in note text.
	if owner, oerr := s.db.GetUser(note.OwnerID); oerr == nil {
		if delta := int64(len(bodyMd)) - int64(len(note.BodyMd)); s.overCapacity(owner, delta) {
			return c.Redirect("/notes/"+strconv.FormatInt(id, 10)+"/edit?error=capacity", fiber.StatusFound)
		}
	}
	if err := s.db.UpdateNoteContent(id, title, bodyMd, bodyMd); err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, err.Error())
	}
	if tags := splitTags(c.FormValue("tags")); tags != nil {
		_ = s.db.SetNoteTags(note.OwnerID, id, "manual", tags)
	}
	// Update category (blank clears it). Categories belong to the note owner.
	if cat := strings.TrimSpace(c.FormValue("category")); cat == "" {
		_ = s.db.SetNoteCategory(id, nil)
	} else if cid, err := s.db.UpsertCategory(note.OwnerID, cat); err == nil && cid > 0 {
		_ = s.db.SetNoteCategory(id, &cid)
	}
	s.saveSchedule(c, id)
	_ = s.db.SetNoteSummaryLang(id, summaryLang(c))
	// Only re-run the AI pipeline (regenerate summary/tags, refresh embeddings,
	// and a title if left blank) when the user opted in. Otherwise the edit is
	// saved as-is, preserving what they typed.
	if canUseAI(c) {
		var parts []string
		if c.FormValue("rerun_title") == "on" {
			parts = append(parts, "title")
		}
		if c.FormValue("rerun_summary") == "on" {
			parts = append(parts, "summary")
		}
		if c.FormValue("rerun_tags") == "on" {
			parts = append(parts, "tags")
		}
		if c.FormValue("rerun_category") == "on" {
			parts = append(parts, "category")
		}
		if len(parts) > 0 {
			if err := s.db.EnqueueJob(id, "process", strings.Join(parts, ",")); err == nil {
				_ = s.db.SetNoteStatus(id, "processing", "")
				s.worker.Kick()
			}
		}
	}
	return c.Redirect("/notes/"+strconv.FormatInt(id, 10), fiber.StatusFound)
}

// retryNote re-queues AI processing for a note that failed (e.g. the AI server
// was unreachable). Allowed for anyone who can edit the note.
func (s *Server) retryNote(c *fiber.Ctx) error {
	u := currentUser(c)
	id, _ := strconv.ParseInt(c.Params("id"), 10, 64)
	_, canEdit, err := s.db.NoteAccess(id, u.ID)
	if err == db.ErrNotFound {
		return fiber.NewError(fiber.StatusNotFound, "note not found")
	}
	if err != nil || !canEdit {
		return fiber.NewError(fiber.StatusForbidden, "you cannot retry this note")
	}
	if err := s.db.SetNoteStatus(id, "processing", ""); err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, err.Error())
	}
	_ = s.db.ClearStopped(id)
	// Regenerate all AI fields, but keep a user-set title.
	parts := "summary,tags,category"
	if n, err := s.db.GetNote(id); err == nil && n.Title == "" {
		parts = "title,summary,tags,category"
	}
	if err := s.db.EnqueueJob(id, "process", parts); err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, err.Error())
	}
	s.worker.Kick()
	return c.Redirect("/notes/"+strconv.FormatInt(id, 10), fiber.StatusFound)
}

// stopNote cancels a note's in-progress AI processing and marks it Stopped.
func (s *Server) stopNote(c *fiber.Ctx) error {
	u := currentUser(c)
	id, _ := strconv.ParseInt(c.Params("id"), 10, 64)
	_, canEdit, err := s.db.NoteAccess(id, u.ID)
	if err == db.ErrNotFound {
		return fiber.NewError(fiber.StatusNotFound, "note not found")
	}
	if err != nil || !canEdit {
		return fiber.NewError(fiber.StatusForbidden, "you cannot stop this note")
	}
	// Delete jobs first so the worker's FailJob path can't requeue; then cancel any
	// in-flight run; then record the stopped state.
	_ = s.db.DeleteJobsForNote(id)
	if s.worker != nil {
		s.worker.Cancel(id)
	}
	if err := s.db.StopNote(id); err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, err.Error())
	}
	return c.Redirect("/notes/"+strconv.FormatInt(id, 10), fiber.StatusFound)
}

func (s *Server) deleteNote(c *fiber.Ctx) error {
	u := currentUser(c)
	id, _ := strconv.ParseInt(c.Params("id"), 10, 64)
	note, err := s.db.GetNote(id)
	if err != nil || note.OwnerID != u.ID {
		return fiber.NewError(fiber.StatusForbidden, "only the owner can delete a note")
	}
	if err := s.db.SoftDeleteNote(id); err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, err.Error())
	}
	return c.Redirect("/notes", fiber.StatusFound)
}

// splitTags parses a comma-separated tag field. Returns an empty (non-nil)
// slice when the field is present but blank, so callers can clear tags.
func splitTags(s string) []string {
	s = strings.TrimSpace(s)
	out := []string{}
	for _, p := range strings.Split(s, ",") {
		if t := strings.TrimSpace(p); t != "" {
			out = append(out, t)
		}
	}
	return out
}
