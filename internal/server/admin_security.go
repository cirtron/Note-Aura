package server

import (
	"database/sql"
	"strconv"
	"strings"

	"github.com/gofiber/fiber/v2"

	"note-aura/internal/i18n"
)

// ---- IP blocking ----

func (s *Server) getAdminIPBlock(c *fiber.Ctx) error {
	ips, _ := s.db.ListBlockedIPs()
	m := baseMap(c, "Admin · Blocked IPs")
	m["Nav"] = "admin"
	m["BlockedIPs"] = ips
	return c.Render("admin_ip_block", m, "layout")
}

func (s *Server) postAdminBlockIP(c *fiber.Ctx) error {
	ip := strings.TrimSpace(c.FormValue("ip"))
	reason := strings.TrimSpace(c.FormValue("reason"))
	if ip != "" {
		_ = s.db.BlockIP(ip, reason)
	}
	return c.Redirect("/admin/ip-block", fiber.StatusFound)
}

func (s *Server) postAdminUnblockIP(c *fiber.Ctx) error {
	id, _ := strconv.ParseInt(c.FormValue("id"), 10, 64)
	if id > 0 {
		_ = s.db.UnblockIP(id)
	}
	return c.Redirect("/admin/ip-block", fiber.StatusFound)
}

// ---- Force logout + clear lockout (per-user actions on users page) ----

func (s *Server) postAdminForceLogout(c *fiber.Ctx) error {
	uid, _ := strconv.ParseInt(c.FormValue("user_id"), 10, 64)
	if uid > 0 {
		_ = s.db.ForceLogout(uid)
	}
	return c.Redirect("/admin/users", fiber.StatusFound)
}

func (s *Server) postAdminClearLock(c *fiber.Ctx) error {
	uid, _ := strconv.ParseInt(c.FormValue("user_id"), 10, 64)
	if uid > 0 {
		_ = s.db.AdminClearLock(uid)
	}
	return c.Redirect("/admin/users", fiber.StatusFound)
}

// ---- Login lockout settings ----

func (s *Server) getAdminLockout(c *fiber.Ctx) error {
	app, _ := s.db.GetAppSettings()
	m := baseMap(c, "Admin · Login Lockout")
	m["Nav"] = "admin"
	m["MaxAttempts"] = app["login_max_attempts"]
	m["LockoutMinutes"] = app["login_lockout_minutes"]
	return c.Render("admin_lockout", m, "layout")
}

func (s *Server) postAdminLockout(c *fiber.Ctx) error {
	_ = s.db.SetAppSetting("login_max_attempts", strings.TrimSpace(c.FormValue("max_attempts")))
	_ = s.db.SetAppSetting("login_lockout_minutes", strings.TrimSpace(c.FormValue("lockout_minutes")))
	return c.Redirect("/admin/lockout", fiber.StatusFound)
}

// ---- Site announcement ----

func (s *Server) getAdminAnnouncement(c *fiber.Ctx) error {
	app, _ := s.db.GetAppSettings()
	m := baseMap(c, "Admin · Announcement")
	m["Nav"] = "admin"
	m["AnnounceText"] = app["announcement_text"]
	m["AnnounceEnabled"] = app["announcement_enabled"] == "1"
	return c.Render("admin_announcement", m, "layout")
}

func (s *Server) postAdminAnnouncement(c *fiber.Ctx) error {
	text := strings.TrimSpace(c.FormValue("announcement_text"))
	enabled := "0"
	if c.FormValue("announcement_enabled") == "on" {
		enabled = "1"
	}
	_ = s.db.SetAppSetting("announcement_text", text)
	_ = s.db.SetAppSetting("announcement_enabled", enabled)
	return c.Redirect("/admin/announcement", fiber.StatusFound)
}

// ---- Admin send email ----

func (s *Server) getAdminEmail(c *fiber.Ctx) error {
	lang := currentLang(c)
	if !s.mailer.Enabled() {
		m := baseMap(c, "Admin · Send Email")
		m["Nav"] = "admin"
		m["Error"] = i18n.T(lang, "admin.email.no_mail")
		return c.Render("admin_email", m, "layout")
	}
	roles, _ := s.db.ListRoles()
	groups, _ := s.db.AllGroups()
	m := baseMap(c, "Admin · Send Email")
	m["Nav"] = "admin"
	m["Roles"] = roles
	m["Groups"] = groups
	return c.Render("admin_email", m, "layout")
}

func (s *Server) postAdminEmail(c *fiber.Ctx) error {
	lang := currentLang(c)
	if !s.mailer.Enabled() {
		m := baseMap(c, "Admin · Send Email")
		m["Nav"] = "admin"
		m["Error"] = i18n.T(lang, "admin.email.no_mail")
		return c.Render("admin_email", m, "layout")
	}

	subject := strings.TrimSpace(c.FormValue("subject"))
	body := strings.TrimSpace(c.FormValue("body"))
	toUser := strings.TrimSpace(c.FormValue("to_user"))
	toRole := strings.TrimSpace(c.FormValue("to_role"))
	toGroup := strings.TrimSpace(c.FormValue("to_group"))

	var recipients []string
	switch {
	case toUser != "":
		recipients = []string{toUser}
	case toRole != "":
		users, _ := s.db.ListUsersByRole(toRole)
		for _, u := range users {
			recipients = append(recipients, u.Email)
		}
	case toGroup != "":
		gid, _ := strconv.ParseInt(toGroup, 10, 64)
		members, _ := s.db.GroupMembers(gid)
		for _, m := range members {
			recipients = append(recipients, m.Email)
		}
	}

	if len(recipients) == 0 || subject == "" || body == "" {
		roles, _ := s.db.ListRoles()
		groups, _ := s.db.AllGroups()
		m := baseMap(c, "Admin · Send Email")
		m["Nav"] = "admin"
		m["Error"] = "Please fill in all fields and choose a recipient."
		m["Roles"] = roles
		m["Groups"] = groups
		return c.Render("admin_email", m, "layout")
	}

	sent := 0
	for _, addr := range recipients {
		if err := s.mailer.Send(addr, subject, body); err == nil {
			sent++
		}
	}

	roles, _ := s.db.ListRoles()
	groups, _ := s.db.AllGroups()
	m := baseMap(c, "Admin · Send Email")
	m["Nav"] = "admin"
	m["Roles"] = roles
	m["Groups"] = groups
	m["Flash"] = i18n.T(lang, "admin.email.sent") + " (" + strconv.Itoa(sent) + ")"
	return c.Render("admin_email", m, "layout")
}

// ---- Banned usernames ----

func (s *Server) getAdminBannedUsernames(c *fiber.Ctx) error {
	list, _ := s.db.ListBannedUsernames()
	m := baseMap(c, "Admin · Banned Usernames")
	m["Nav"] = "admin"
	m["BannedUsernames"] = list
	return c.Render("admin_banned_usernames", m, "layout")
}

func (s *Server) postAdminAddBannedUsername(c *fiber.Ctx) error {
	username := strings.TrimSpace(c.FormValue("username"))
	note := strings.TrimSpace(c.FormValue("note"))
	if username != "" {
		_ = s.db.AddBannedUsername(username, note)
	}
	return c.Redirect("/admin/banned-usernames", fiber.StatusFound)
}

func (s *Server) postAdminRemoveBannedUsername(c *fiber.Ctx) error {
	id, _ := strconv.ParseInt(c.FormValue("id"), 10, 64)
	if id > 0 {
		_ = s.db.RemoveBannedUsername(id)
	}
	return c.Redirect("/admin/banned-usernames", fiber.StatusFound)
}

// ---- Banned email patterns ----

func (s *Server) getAdminBannedEmails(c *fiber.Ctx) error {
	list, _ := s.db.ListBannedEmailPatterns()
	m := baseMap(c, "Admin · Banned Emails")
	m["Nav"] = "admin"
	m["BannedEmails"] = list
	return c.Render("admin_banned_emails", m, "layout")
}

func (s *Server) postAdminAddBannedEmail(c *fiber.Ctx) error {
	pattern := strings.TrimSpace(c.FormValue("pattern"))
	note := strings.TrimSpace(c.FormValue("note"))
	if pattern != "" {
		_ = s.db.AddBannedEmailPattern(pattern, note)
	}
	return c.Redirect("/admin/banned-emails", fiber.StatusFound)
}

func (s *Server) postAdminRemoveBannedEmail(c *fiber.Ctx) error {
	id, _ := strconv.ParseInt(c.FormValue("id"), 10, 64)
	if id > 0 {
		_ = s.db.RemoveBannedEmailPattern(id)
	}
	return c.Redirect("/admin/banned-emails", fiber.StatusFound)
}

// announcementMiddleware injects the site announcement into every page.
func (s *Server) announcementMiddleware(c *fiber.Ctx) error {
	app, _ := s.db.GetAppSettings()
	if app["announcement_enabled"] == "1" && app["announcement_text"] != "" {
		c.Locals("announcement", app["announcement_text"])
	}
	return c.Next()
}

// getAnnouncementText is a template function that reads the announcement text
// from Locals (set by announcementMiddleware).
func getAnnouncementText(c *fiber.Ctx) string {
	if v, ok := c.Locals("announcement").(string); ok {
		return v
	}
	return ""
}

// suspendedUntilLabel returns a human-readable suspension expiry label or "".
func suspendedUntilLabel(t sql.NullTime) string {
	if !t.Valid {
		return ""
	}
	return t.Time.Format("2006-01-02 15:04")
}
