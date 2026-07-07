package server

import (
	"os"
	"strings"

	"github.com/gofiber/fiber/v2"
)

func (s *Server) getGuide(c *fiber.Ctx) error {
	m := baseMap(c, "User Guide")
	content := ""
	if app, err := s.db.GetAppSettings(); err == nil {
		content = app["user_guide"]
	}
	if content == "" {
		lang, _ := c.Locals("lang").(string)
		content = loadBuiltinGuide(lang)
	}
	m["GuideContent"] = content
	return c.Render("guide", m, "layout")
}

func (s *Server) getAdminGuide(c *fiber.Ctx) error {
	m := baseMap(c, "Edit User Guide")
	if app, err := s.db.GetAppSettings(); err == nil {
		m["GuideContent"] = app["user_guide"]
	}
	m["Saved"] = c.Query("saved") == "1"
	return c.Render("admin_guide", m, "layout")
}

func (s *Server) postAdminGuide(c *fiber.Ctx) error {
	_ = s.db.SetAppSetting("user_guide", strings.TrimSpace(c.FormValue("guide_content")))
	return c.Redirect("/admin/guide?saved=1", fiber.StatusFound)
}

// loadBuiltinGuide reads USER_GUIDE.{lang}.md from the working directory,
// falling back to USER_GUIDE.md when no language variant exists.
func loadBuiltinGuide(lang string) string {
	for _, name := range []string{"USER_GUIDE." + lang + ".md", "USER_GUIDE.md"} {
		if data, err := os.ReadFile(name); err == nil {
			return string(data)
		}
	}
	return ""
}
