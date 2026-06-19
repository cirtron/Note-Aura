package server

import (
	"sort"
	"strings"

	"github.com/gofiber/fiber/v2"

	"note-aura/internal/db"
)

// uploadCategories is the ordered list of high-level upload categories an admin
// can toggle per role. Each maps to a set of file extensions.
var uploadCategories = []string{"image", "video", "audio", "document"}

// categoryExts maps each category to its allowed extensions (no leading dot).
var categoryExts = map[string][]string{
	"image":    {"png", "jpg", "jpeg", "gif", "webp", "bmp"},
	"video":    {"mp4", "webm", "mov", "mkv", "avi", "m4v"},
	"audio":    {"mp3", "wav", "ogg", "m4a", "flac", "aac"},
	"document": {"pdf", "txt", "md", "markdown", "csv", "doc", "docx", "ppt", "pptx", "xls", "xlsx", "rtf", "odt", "json", "log"},
}

// textExts are extensions whose content is read directly into the note body.
var textExts = map[string]bool{
	"txt": true, "md": true, "markdown": true, "csv": true, "log": true, "json": true, "text": true,
}

// normalizeExt lowercases an extension and strips a leading dot/space.
func normalizeExt(e string) string {
	return strings.ToLower(strings.TrimPrefix(strings.TrimSpace(e), "."))
}

// isCategory reports whether tok is a known upload category.
func isCategory(tok string) bool {
	_, ok := categoryExts[tok]
	return ok
}

// parseUploadSpec splits a role's upload_types value into its checked categories,
// custom (non-category) extensions, and an "any" flag ("*").
func parseUploadSpec(spec string) (cats map[string]bool, custom []string, any bool) {
	cats = map[string]bool{}
	for _, tok := range strings.Split(spec, ",") {
		t := normalizeExt(tok)
		switch {
		case t == "":
			continue
		case t == "*":
			any = true
		case isCategory(t):
			cats[t] = true
		default:
			custom = append(custom, t)
		}
	}
	return
}

// resolveAllowedExts expands a role's upload_types spec into the concrete set of
// allowed extensions. The bool is true when ANY extension is allowed ("*").
func resolveAllowedExts(spec string) (exts map[string]bool, any bool) {
	cats, custom, any := parseUploadSpec(spec)
	if any {
		return nil, true
	}
	exts = map[string]bool{}
	for c := range cats {
		for _, e := range categoryExts[c] {
			exts[e] = true
		}
	}
	for _, e := range custom {
		exts[e] = true
	}
	return exts, false
}

// extCategory returns the high-level category for an extension, or "other".
func extCategory(ext string) string {
	ext = normalizeExt(ext)
	for cat, list := range categoryExts {
		for _, e := range list {
			if e == ext {
				return cat
			}
		}
	}
	return "other"
}

// userCanUploadExt reports whether the user's role permits uploading a file with
// the given extension. Admins may upload anything.
func (s *Server) userCanUploadExt(u *db.User, ext string) bool {
	if u == nil {
		return false
	}
	if u.IsAdmin {
		return true
	}
	r := s.userRole(u)
	if r == nil {
		return false
	}
	exts, any := resolveAllowedExts(r.UploadTypes)
	if any {
		return true
	}
	return exts[normalizeExt(ext)]
}

// userUploadInfo returns an HTML "accept" attribute, a human-readable hint, and
// whether the user may upload files at all (per their role; admins: anything).
func (s *Server) userUploadInfo(u *db.User) (accept, hint string, can bool) {
	if u == nil {
		return "", "", false
	}
	if u.IsAdmin {
		return "", "any file type", true
	}
	r := s.userRole(u)
	if r == nil {
		return "", "", false
	}
	exts, any := resolveAllowedExts(r.UploadTypes)
	if any {
		return "", "any file type", true
	}
	if len(exts) == 0 {
		return "", "", false
	}
	list := make([]string, 0, len(exts))
	for e := range exts {
		list = append(list, e)
	}
	sort.Strings(list)
	return "." + strings.Join(list, ",."), strings.Join(list, ", "), true
}

// uploadSpecFromForm builds a role's upload_types value from the admin form:
// checked category checkboxes plus a comma list of extra extensions.
func uploadSpecFromForm(c *fiber.Ctx) string {
	var toks []string
	for _, cat := range uploadCategories {
		if c.FormValue("upload_"+cat) == "on" {
			toks = append(toks, cat)
		}
	}
	for _, e := range strings.Split(c.FormValue("upload_exts"), ",") {
		if t := normalizeExt(e); t != "" && !isCategory(t) {
			toks = append(toks, t)
		}
	}
	return strings.Join(toks, ",")
}
