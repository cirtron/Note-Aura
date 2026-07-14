package server

// REST JSON API consumed by the React Native mobile app.
// All routes are mounted under /api by server.go.
// Auth: POST /api/auth/login returns a Bearer token; all other routes require
// "Authorization: Bearer <token>" header (requireAPIAuth middleware).

import (
	"strconv"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"

	"note-aura/internal/auth"
	"note-aura/internal/db"
)

// ----- JSON shapes -----

type apiNoteItem struct {
	ID           int64    `json:"id"`
	Title        string   `json:"title"`
	Summary      string   `json:"summary"`
	Status       string   `json:"status"`
	SourceType   string   `json:"source_type"`
	Tags         []string `json:"tags"`
	CategoryName string   `json:"category"`
	CreatedAt    string   `json:"created_at"`
	UpdatedAt    string   `json:"updated_at"`
}

type apiNoteDetail struct {
	apiNoteItem
	BodyMd    string `json:"body_md"`
	SourceRef string `json:"source_ref,omitempty"`
	Error     string `json:"error,omitempty"`
	AIMillis  int64  `json:"ai_ms,omitempty"`
}

func noteToItem(n *db.Note) apiNoteItem {
	tags := n.Tags
	if tags == nil {
		tags = []string{}
	}
	return apiNoteItem{
		ID:           n.ID,
		Title:        n.Title,
		Summary:      n.Summary,
		Status:       n.Status,
		SourceType:   n.SourceType,
		Tags:         tags,
		CategoryName: n.CategoryName,
		CreatedAt:    n.CreatedAt.Format(time.RFC3339),
		UpdatedAt:    n.UpdatedAt.Format(time.RFC3339),
	}
}

func noteToDetail(n *db.Note) apiNoteDetail {
	return apiNoteDetail{
		apiNoteItem: noteToItem(n),
		BodyMd:      n.BodyMd,
		SourceRef:   n.SourceRef,
		Error:       n.Error,
		AIMillis:    n.AIMillis,
	}
}

// ----- auth -----

// apiLogin handles POST /api/auth/login.
// Body: {"email":"...","password":"...","device_name":"..."}
// Response: {"token":"...","user":{...}}
func (s *Server) apiLogin(c *fiber.Ctx) error {
	var body struct {
		Email      string `json:"email"`
		Password   string `json:"password"`
		DeviceName string `json:"device_name"`
	}
	if err := c.BodyParser(&body); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid JSON"})
	}
	email := strings.ToLower(strings.TrimSpace(body.Email))
	if email == "" || body.Password == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "email and password required"})
	}

	user, err := s.db.GetUserByEmail(email)
	if err != nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "invalid credentials"})
	}
	if user.LockedUntil.Valid && time.Now().Before(user.LockedUntil.Time) {
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"error": "account locked"})
	}
	if !auth.VerifyPassword(user.PasswordHash, body.Password) {
		app, _ := s.db.GetAppSettings()
		maxAttempts := appIntSetting(app, "login_max_attempts", 5)
		lockoutMins := appIntSetting(app, "login_lockout_minutes", 15)
		s.db.RecordFailedLogin(user.ID, maxAttempts, lockoutMins)
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "invalid credentials"})
	}
	if user.Suspended {
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"error": "account suspended"})
	}
	if !user.EmailVerified {
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"error": "email not verified"})
	}
	_ = s.db.ClearFailedLogins(user.ID)

	name := strings.TrimSpace(body.DeviceName)
	if name == "" {
		name = "mobile"
	}
	token, err := s.db.CreateAPIToken(user.ID, name)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "could not create token"})
	}
	return c.JSON(fiber.Map{
		"token": token,
		"user": fiber.Map{
			"id":       user.ID,
			"email":    user.Email,
			"is_admin": user.IsAdmin,
		},
	})
}

// apiLogout handles POST /api/auth/logout — deletes the Bearer token.
func (s *Server) apiLogout(c *fiber.Ctx) error {
	header := c.Get("Authorization")
	const prefix = "Bearer "
	if len(header) > len(prefix) {
		_ = s.db.DeleteAPIToken(header[len(prefix):])
	}
	return c.JSON(fiber.Map{"ok": true})
}

// ----- notes -----

// apiListNotes handles GET /api/notes.
// Query params: page, per_page (default 20), q, tag, category, sort.
func (s *Server) apiListNotes(c *fiber.Ctx) error {
	u := currentUser(c)
	q := strings.TrimSpace(c.Query("q"))
	tag := strings.TrimSpace(c.Query("tag"))
	category := strings.TrimSpace(c.Query("category"))
	sortKey := validSort(c.Query("sort"))

	var notes []*db.Note
	if q != "" {
		notes, _ = s.db.SearchNotes(u.ID, q)
	} else {
		notes, _ = s.db.ListNotes(u.ID)
	}

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
	sortNotes(notes, sortKey)

	per := 20
	if v, err := strconv.Atoi(c.Query("per_page")); err == nil && v > 0 && v <= 100 {
		per = v
	}
	total := len(notes)
	page := 1
	if v, err := strconv.Atoi(c.Query("page")); err == nil && v > 0 {
		page = v
	}
	start := (page - 1) * per
	end := start + per
	if start > total {
		start = total
	}
	if end > total {
		end = total
	}

	items := make([]apiNoteItem, 0, end-start)
	for _, n := range notes[start:end] {
		items = append(items, noteToItem(n))
	}
	return c.JSON(fiber.Map{
		"notes":    items,
		"total":    total,
		"page":     page,
		"per_page": per,
	})
}

// apiGetNote handles GET /api/notes/:id.
func (s *Server) apiGetNote(c *fiber.Ctx) error {
	u := currentUser(c)
	id, err := strconv.ParseInt(c.Params("id"), 10, 64)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid id"})
	}
	n, err := s.db.GetNote(id)
	if err == db.ErrNotFound {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "not found"})
	}
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	if n.OwnerID != u.ID {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "not found"})
	}
	return c.JSON(noteToDetail(n))
}

// apiCreateNote handles POST /api/notes.
// Body: {"title":"","body_md":"","tags":["a","b"],"category":"","source_type":"manual","source_ref":""}
func (s *Server) apiCreateNote(c *fiber.Ctx) error {
	u := currentUser(c)
	var body struct {
		Title      string   `json:"title"`
		BodyMd     string   `json:"body_md"`
		Tags       []string `json:"tags"`
		Category   string   `json:"category"`
		SourceType string   `json:"source_type"`
		SourceRef  string   `json:"source_ref"`
	}
	if err := c.BodyParser(&body); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid JSON"})
	}
	if s.overCapacity(u, int64(len(body.BodyMd))) {
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"error": "storage limit reached"})
	}
	sourceType := body.SourceType
	if sourceType == "" {
		sourceType = "manual"
	}
	note := &db.Note{
		OwnerID:    u.ID,
		Title:      strings.TrimSpace(body.Title),
		BodyMd:     body.BodyMd,
		BodyText:   body.BodyMd,
		SourceType: sourceType,
		SourceRef:  body.SourceRef,
		Status:     "processing",
	}
	id, err := s.db.CreateNote(note)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	if len(body.Tags) > 0 {
		_ = s.db.SetNoteTags(u.ID, id, "manual", body.Tags)
	}
	if cat := strings.TrimSpace(body.Category); cat != "" {
		if cid, err := s.db.UpsertCategory(u.ID, cat); err == nil && cid > 0 {
			_ = s.db.SetNoteCategory(id, &cid)
		}
	}

	useAI := canUseAI(c)
	if useAI {
		parts := []string{"summary"}
		if note.Title == "" {
			parts = append([]string{"title"}, parts...)
		}
		if len(body.Tags) == 0 {
			parts = append(parts, "tags")
		}
		if body.Category == "" {
			parts = append(parts, "category")
		}
		if err := s.db.EnqueueJob(id, "process", strings.Join(parts, ",")); err == nil {
			s.worker.Kick()
		}
	} else {
		s.db.SQL.Exec(`UPDATE notes SET status='ready' WHERE id=?`, id)
	}

	n, _ := s.db.GetNote(id)
	if n == nil {
		return c.Status(fiber.StatusCreated).JSON(fiber.Map{"id": id})
	}
	return c.Status(fiber.StatusCreated).JSON(noteToDetail(n))
}

// apiUpdateNote handles PUT /api/notes/:id.
// Only provided non-empty fields are updated.
func (s *Server) apiUpdateNote(c *fiber.Ctx) error {
	u := currentUser(c)
	id, err := strconv.ParseInt(c.Params("id"), 10, 64)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid id"})
	}
	note, err := s.db.GetNote(id)
	if err == db.ErrNotFound || note.OwnerID != u.ID {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "not found"})
	}
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	var body struct {
		Title    *string  `json:"title"`
		BodyMd   *string  `json:"body_md"`
		Tags     []string `json:"tags"`
		Category *string  `json:"category"`
	}
	if err := c.BodyParser(&body); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid JSON"})
	}

	title := note.Title
	bodyMd := note.BodyMd
	if body.Title != nil {
		title = strings.TrimSpace(*body.Title)
	}
	if body.BodyMd != nil {
		bodyMd = *body.BodyMd
		if delta := int64(len(bodyMd)) - int64(len(note.BodyMd)); s.overCapacity(u, delta) {
			return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"error": "storage limit reached"})
		}
	}
	if err := s.db.UpdateNoteContent(id, title, bodyMd, bodyMd); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	if body.Tags != nil {
		_ = s.db.SetNoteTags(u.ID, id, "ai", nil)
		_ = s.db.SetNoteTags(u.ID, id, "manual", body.Tags)
	}
	if body.Category != nil {
		cat := strings.TrimSpace(*body.Category)
		if cat == "" {
			_ = s.db.SetNoteCategory(id, nil)
		} else {
			if cid, err := s.db.UpsertCategory(u.ID, cat); err == nil && cid > 0 {
				_ = s.db.SetNoteCategory(id, &cid)
			}
		}
	}

	n, _ := s.db.GetNote(id)
	if n == nil {
		return c.JSON(fiber.Map{"id": id})
	}
	return c.JSON(noteToDetail(n))
}

// apiDeleteNote handles DELETE /api/notes/:id — soft delete.
func (s *Server) apiDeleteNote(c *fiber.Ctx) error {
	u := currentUser(c)
	id, err := strconv.ParseInt(c.Params("id"), 10, 64)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid id"})
	}
	n, err := s.db.GetNote(id)
	if err == db.ErrNotFound || n.OwnerID != u.ID {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "not found"})
	}
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	if err := s.db.SoftDeleteNote(id); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(fiber.Map{"ok": true})
}

// ----- taxonomy -----

// apiListTags handles GET /api/tags.
func (s *Server) apiListTags(c *fiber.Ctx) error {
	u := currentUser(c)
	tags, err := s.db.TagsWithCounts(u.ID)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	type tagJSON struct {
		Name  string `json:"name"`
		Count int    `json:"count"`
	}
	out := make([]tagJSON, 0, len(tags))
	for _, t := range tags {
		out = append(out, tagJSON{Name: t.Name, Count: t.Count})
	}
	return c.JSON(out)
}

// apiListCategories handles GET /api/categories.
func (s *Server) apiListCategories(c *fiber.Ctx) error {
	u := currentUser(c)
	cats, err := s.db.CategoriesWithCounts(u.ID)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	type catJSON struct {
		Name  string `json:"name"`
		Count int    `json:"count"`
	}
	out := make([]catJSON, 0, len(cats))
	for _, ct := range cats {
		out = append(out, catJSON{Name: ct.Name, Count: ct.Count})
	}
	return c.JSON(out)
}
