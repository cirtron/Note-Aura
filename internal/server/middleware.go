package server

import (
	"net/url"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"

	"note-aura/internal/db"
	"note-aura/internal/i18n"
)

const userLocalKey = "user"

// loadSession resolves the session cookie to a *db.User in c.Locals. Anonymous
// requests proceed with no user set. Blocked IPs are rejected immediately.
func (s *Server) loadSession(c *fiber.Ctx) error {
	// Expire any timed suspensions once per request (cheap no-op when none match).
	s.db.AutoExpireSuspensions()

	// Block banned IP addresses before touching any session.
	if s.db.IsIPBlocked(c.IP()) {
		return c.Status(fiber.StatusForbidden).SendString("Your IP address has been blocked.")
	}

	sid := c.Cookies(s.cfg.Session.CookieName)
	if sid == "" {
		return c.Next()
	}
	sess, err := s.db.GetSession(sid)
	if err != nil {
		return c.Next()
	}
	if user, err := s.db.GetUser(sess.UserID); err == nil && !user.Suspended {
		c.Locals(userLocalKey, user)
		c.Locals("canAI", s.canUseAIUser(user))
		// Record the visit (time + IP), throttled to avoid a write on every
		// request, but also refresh when the source IP changes.
		ip := c.IP()
		if !user.LastSeenAt.Valid || time.Since(user.LastSeenAt.Time) > 5*time.Minute || ip != user.LastSeenIP {
			_ = s.db.TouchUserSeen(user.ID, ip)
		}
	}
	return c.Next()
}

// currentUser returns the logged-in user or nil.
func currentUser(c *fiber.Ctx) *db.User {
	u, _ := c.Locals(userLocalKey).(*db.User)
	return u
}

// requireAuth redirects anonymous users to /login.
func (s *Server) requireAuth(c *fiber.Ctx) error {
	if currentUser(c) == nil {
		return c.Redirect("/login", fiber.StatusFound)
	}
	return c.Next()
}

// requireAdmin rejects non-admin users (runs after requireAuth).
func (s *Server) requireAdmin(c *fiber.Ctx) error {
	u := currentUser(c)
	if u == nil || !u.IsAdmin {
		return fiber.NewError(fiber.StatusForbidden, "admin only")
	}
	return c.Next()
}

// detectLang resolves the UI language and timezone into c.Locals for templates.
// Precedence: explicit lang cookie → logged-in user's saved preference → Accept-Language → default.
func (s *Server) detectLang(c *fiber.Ctx) error {
	lang := ""
	tz := ""
	if ck := c.Cookies("lang"); i18n.Supported(ck) {
		lang = ck
	}
	if u := currentUser(c); u != nil {
		if settings, err := s.db.GetUserSettings(u.ID); err == nil {
			if lang == "" && i18n.Supported(settings["lang"]) {
				lang = settings["lang"]
			}
			tz = settings["timezone"]
		}
	}
	if lang == "" {
		lang = i18n.Match(c.Get("Accept-Language"))
	}
	c.Locals("lang", lang)
	c.Locals("userTZ", tz)
	return c.Next()
}

// currentLang returns the resolved UI language for this request.
func currentLang(c *fiber.Ctx) string {
	if v, ok := c.Locals("lang").(string); ok && v != "" {
		return v
	}
	return i18n.Default
}

// setLang stores a language cookie and returns to the previous page. When the
// user is logged in, it also persists the preference to their settings.
func (s *Server) setLang(c *fiber.Ctx) error {
	if code := c.Params("code"); i18n.Supported(code) {
		c.Cookie(&fiber.Cookie{
			Name: "lang", Value: code, Path: "/",
			Expires: time.Now().AddDate(1, 0, 0), SameSite: "Lax",
		})
		if u := currentUser(c); u != nil {
			_ = s.db.SetUserSetting(u.ID, "lang", code)
		}
	}
	return c.Redirect(safeReferer(c.Get("Referer")), fiber.StatusFound)
}

// safeReferer returns the Referer as a local redirect target, or "/" when it is
// empty, unparseable, or points off-site (open-redirect guard).
func safeReferer(ref string) string {
	if ref == "" {
		return "/"
	}
	u, err := url.Parse(ref)
	if err != nil {
		return "/"
	}
	// Only allow a same-document path+query (no scheme, no host). url.Parse on an
	// absolute URL populates Host; on a protocol-relative "//evil.com" it also sets
	// Host. Reject both.
	if u.Scheme != "" || u.Host != "" {
		return "/"
	}
	p := u.EscapedPath()
	if p == "" || p[0] != '/' {
		return "/"
	}
	if u.RawQuery != "" {
		p += "?" + u.RawQuery
	}
	return p
}

// summaryLang reads and validates the per-note summary-language form value.
func summaryLang(c *fiber.Ctx) string {
	if v := strings.TrimSpace(c.FormValue("summary_lang")); i18n.Supported(v) {
		return v
	}
	return ""
}

// withLang adds the resolved language to a render map (for pages not using
// baseMap, e.g. login/register).
func withLang(c *fiber.Ctx, m fiber.Map) fiber.Map {
	m["Lang"] = currentLang(c)
	return m
}

// canUseAI reports the AI privilege resolved for the current request.
func canUseAI(c *fiber.Ctx) bool {
	if v, ok := c.Locals("canAI").(bool); ok {
		return v
	}
	return true
}

// baseMap seeds template data common to authenticated pages.
func baseMap(c *fiber.Ctx, title string) fiber.Map {
	m := fiber.Map{"Title": title, "Lang": currentLang(c)}
	if u := currentUser(c); u != nil {
		m["UserEmail"] = u.Email
		m["IsAdmin"] = u.IsAdmin
		m["CanAI"] = canUseAI(c)
	}
	if tz, ok := c.Locals("userTZ").(string); ok {
		m["UserTZ"] = tz
	}
	if ann, ok := c.Locals("announcement").(string); ok && ann != "" {
		m["Announcement"] = ann
	}
	return m
}
