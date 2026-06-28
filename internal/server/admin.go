package server

import (
	"crypto/tls"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/gofiber/fiber/v2"

	"note-aura/internal/ai"
	"note-aura/internal/auth"
	"note-aura/internal/db"
)

// getUsers renders the admin Users tab: account management + all invitations.
func (s *Server) getUsers(c *fiber.Ctx) error {
	m := baseMap(c, "Users")
	m["Nav"] = "users"
	m["Users"], _ = s.db.ListUsers()
	m["Roles"], _ = s.db.ListRoles()
	m["Invitations"], _ = s.db.ListAllInvitations()
	if c.Query("ucreated") == "1" {
		m["UserCreated"] = true
	}
	switch c.Query("uerr") {
	case "invalid":
		m["UserError"] = "Please enter a valid email address."
	case "password":
		m["UserError"] = "Password must be at least 6 characters."
	case "exists":
		m["UserError"] = "An account with that email already exists."
	case "create":
		m["UserError"] = "Could not create the account."
	}
	return c.Render("users", m, "layout")
}

// createUserAdmin creates a new account directly (admin only). The account is
// pre-verified — no email verification or invitation is required.
func (s *Server) createUserAdmin(c *fiber.Ctx) error {
	email := strings.ToLower(strings.TrimSpace(c.FormValue("email")))
	password := c.FormValue("password")
	if auth.ValidateEmail(email) != nil {
		return c.Redirect("/admin/users?uerr=invalid", fiber.StatusFound)
	}
	if len(password) < 6 {
		return c.Redirect("/admin/users?uerr=password", fiber.StatusFound)
	}
	if _, err := s.db.GetUserByEmail(email); err == nil {
		return c.Redirect("/admin/users?uerr=exists", fiber.StatusFound)
	}
	hash, err := auth.HashPassword(password)
	if err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, err.Error())
	}
	id, err := s.db.CreateUser(email, hash, c.FormValue("is_admin") == "on", true, "") // emailVerified=true
	if err != nil {
		return c.Redirect("/admin/users?uerr=create", fiber.StatusFound)
	}
	if role := strings.TrimSpace(c.FormValue("role_slug")); role != "" {
		_ = s.db.SetUserRole(id, role)
	}
	return c.Redirect("/admin/users?ucreated=1", fiber.StatusFound)
}

// getLogs renders the admin system-log viewer (paginated, filterable).
func (s *Server) getLogs(c *fiber.Ctx) error {
	level := strings.TrimSpace(c.Query("level"))
	category := strings.TrimSpace(c.Query("category"))
	const per = 50
	total, _ := s.db.CountLogs(level, category)
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
	logs, _ := s.db.ListLogs(level, category, per, (page-1)*per)
	cats, _ := s.db.LogCategories()

	m := baseMap(c, "System logs")
	m["Nav"] = "logs"
	m["Logs"] = logs
	m["Level"] = level
	m["Category"] = category
	m["LogCategories"] = cats
	m["Total"] = total
	m["Page"] = page
	m["TotalPages"] = totalPages
	link := func(p int) string {
		v := url.Values{}
		if level != "" {
			v.Set("level", level)
		}
		if category != "" {
			v.Set("category", category)
		}
		if p > 1 {
			v.Set("page", strconv.Itoa(p))
		}
		if enc := v.Encode(); enc != "" {
			return "/admin/logs?" + enc
		}
		return "/admin/logs"
	}
	if page > 1 {
		m["PrevURL"] = link(page - 1)
	}
	if page < totalPages {
		m["NextURL"] = link(page + 1)
	}
	return c.Render("logs", m, "layout")
}

// clearLogs deletes all system log entries.
func (s *Server) clearLogs(c *fiber.Ctx) error {
	_ = s.db.ClearLogs()
	return c.Redirect("/admin/logs", fiber.StatusFound)
}

// roleView is a Role plus its parsed upload-type state, for the admin form.
type roleView struct {
	*db.Role
	UploadCats   map[string]bool
	UploadCustom string
	UploadAny    bool
}

// getAdmin renders the global AI configuration: per-capability models and the
// editable prompts. Stored values are shown; blanks fall back to the config.yaml
// defaults displayed alongside.
func (s *Server) getAdmin(c *fiber.Ctx) error {
	app, _ := s.db.GetAppSettings()
	def := ai.DefaultPrompts()
	m := baseMap(c, "Admin · AI settings")
	m["Nav"] = "admin"

	m["OllamaURL"] = app[ai.KeyOllamaURL]
	m["ModelTitle"] = app[ai.KeyModelTitle]
	m["ModelSummary"] = app[ai.KeyModelSummary]
	m["ModelTags"] = app[ai.KeyModelTags]
	m["ModelOCR"] = app[ai.KeyModelOCR]
	m["ModelImage"] = app[ai.KeyModelImage]
	m["ModelEmbed"] = app[ai.KeyModelEmbed]
	m["ModelChat"] = app[ai.KeyModelChat]

	m["FacebookCookies"] = app["facebook.cookies"]

	m["PromptTitle"] = firstNonEmptyStr(app[ai.KeyPromptTitle], def.Title)
	m["PromptSummary"] = firstNonEmptyStr(app[ai.KeyPromptSummary], def.Summary)
	m["PromptTags"] = firstNonEmptyStr(app[ai.KeyPromptTags], def.Tags)
	m["PromptCategory"] = firstNonEmptyStr(app[ai.KeyPromptCategory], def.Category)
	m["PromptOCR"] = firstNonEmptyStr(app[ai.KeyPromptOCR], def.OCR)
	m["PromptImage"] = firstNonEmptyStr(app[ai.KeyPromptImage], def.Image)

	// Source-specific prompts (blank = falls back to the general prompt above).
	m["PromptWebTitle"] = app[ai.KeyPromptWebTitle]
	m["PromptWebSummary"] = app[ai.KeyPromptWebSummary]
	m["PromptWebTags"] = app[ai.KeyPromptWebTags]
	m["PromptYTTitle"] = app[ai.KeyPromptYTTitle]
	m["PromptYTSummary"] = app[ai.KeyPromptYTSummary]
	m["PromptYTTags"] = app[ai.KeyPromptYTTags]

	// Show the config.yaml fallbacks so the admin knows the effective default.
	m["DefChat"] = s.fallback.Models.Chat
	m["DefVision"] = s.fallback.Models.OCR
	m["DefEmbed"] = s.fallback.Models.Embed
	m["DefOllama"] = s.fallback.OllamaURL

	// Branding.
	m["BrandLogo"] = app[brandLogoKey]
	m["BrandText"] = app[brandTextKey]

	// Loaded holiday data (for the management list).
	m["HolidayCountries"], _ = s.db.DistinctHolidayCountries()

	// Registration toggle.
	m["RegistrationOpen"] = app["registration_enabled"] != "off"

	// HTTPS / TLS (admin setting overrides config.yaml; applies on restart).
	cert, key, sslOn := s.effectiveTLS()
	m["SSLEnabled"] = sslOn
	m["SSLCert"] = cert
	m["SSLKey"] = key
	switch c.Query("sslerror") {
	case "load":
		m["SSLError"] = "Could not load that certificate/key pair — check the paths and that the key is an unencrypted PEM."
	case "paths":
		m["SSLError"] = "Enter both a certificate file and a key file to enable HTTPS."
	}
	if c.Query("sslsaved") == "1" {
		m["SSLSaved"] = true
	}

	// Roles management (view-model carries parsed upload-type state).
	roles, _ := s.db.ListRoles()
	rvs := make([]roleView, 0, len(roles))
	for _, r := range roles {
		cats, custom, any := parseUploadSpec(r.UploadTypes)
		rvs = append(rvs, roleView{Role: r, UploadCats: cats, UploadCustom: strings.Join(custom, ", "), UploadAny: any})
	}
	m["Roles"] = rvs
	return c.Render("admin", m, "layout")
}

// logoExts are the image extensions accepted for an uploaded logo.
var logoExts = map[string]bool{
	".png": true, ".jpg": true, ".jpeg": true, ".gif": true, ".svg": true, ".webp": true,
}

// postAdminBranding saves the organization logo and/or wording. The logo (if
// uploaded) takes precedence; the wording is used when there is no logo; with
// neither, the product name "Note-Aura" is shown.
func (s *Server) postAdminBranding(c *fiber.Ctx) error {
	_ = s.db.SetAppSetting(brandTextKey, strings.TrimSpace(c.FormValue("brand_text")))

	if c.FormValue("remove_logo") == "on" {
		_ = s.db.SetAppSetting(brandLogoKey, "")
	}

	if fh, err := c.FormFile("logo"); err == nil && fh != nil && fh.Size > 0 {
		ext := strings.ToLower(filepath.Ext(fh.Filename))
		if !logoExts[ext] {
			return fiber.NewError(fiber.StatusBadRequest, "logo must be an image (png, jpg, gif, svg, webp)")
		}
		dir := filepath.Join(s.uploadsDir, "brand")
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return fiber.NewError(fiber.StatusInternalServerError, err.Error())
		}
		dest := filepath.Join(dir, "logo"+ext)
		// Remove any previous logo of a different extension so the path is unique.
		for e := range logoExts {
			if e != ext {
				os.Remove(filepath.Join(dir, "logo"+e))
			}
		}
		if err := c.SaveFile(fh, dest); err != nil {
			return fiber.NewError(fiber.StatusInternalServerError, err.Error())
		}
		_ = s.db.SetAppSetting(brandLogoKey, "/uploads/brand/logo"+ext)
	}

	return c.Redirect("/admin", fiber.StatusFound)
}

// postAdmin saves the global AI configuration. Empty fields clear the override
// (reverting to the config.yaml default).
func (s *Server) postAdmin(c *fiber.Ctx) error {
	set := func(k, v string) { _ = s.db.SetAppSetting(k, v) }

	set(ai.KeyOllamaURL, c.FormValue("ollama_url"))
	set(ai.KeyModelTitle, c.FormValue("model_title"))
	set(ai.KeyModelSummary, c.FormValue("model_summary"))
	set(ai.KeyModelTags, c.FormValue("model_tags"))
	set(ai.KeyModelOCR, c.FormValue("model_ocr"))
	set(ai.KeyModelImage, c.FormValue("model_image"))
	set(ai.KeyModelEmbed, c.FormValue("model_embed"))
	set(ai.KeyModelChat, c.FormValue("model_chat"))

	set(ai.KeyPromptTitle, c.FormValue("prompt_title"))
	set(ai.KeyPromptSummary, c.FormValue("prompt_summary"))
	set(ai.KeyPromptTags, c.FormValue("prompt_tags"))
	set(ai.KeyPromptCategory, c.FormValue("prompt_category"))
	set(ai.KeyPromptOCR, c.FormValue("prompt_ocr"))
	set(ai.KeyPromptImage, c.FormValue("prompt_image"))

	set(ai.KeyPromptWebTitle, c.FormValue("prompt_web_title"))
	set(ai.KeyPromptWebSummary, c.FormValue("prompt_web_summary"))
	set(ai.KeyPromptWebTags, c.FormValue("prompt_web_tags"))
	set(ai.KeyPromptYTTitle, c.FormValue("prompt_yt_title"))
	set(ai.KeyPromptYTSummary, c.FormValue("prompt_yt_summary"))
	set(ai.KeyPromptYTTags, c.FormValue("prompt_yt_tags"))

	set("facebook.cookies", strings.TrimSpace(c.FormValue("facebook_cookies")))

	return c.Redirect("/admin", fiber.StatusFound)
}

func firstNonEmptyStr(a, b string) string {
	if a != "" {
		return a
	}
	return b
}

// appIntSetting reads an integer from app_settings, falling back to defaultVal.
func appIntSetting(app map[string]string, key string, defaultVal int) int {
	if v, err := strconv.Atoi(app[key]); err == nil && v > 0 {
		return v
	}
	return defaultVal
}

// setSSL stores the admin HTTPS preference. Enabling validates the cert/key pair
// up front so a bad path can't break the next startup. Takes effect on restart.
func (s *Server) setSSL(c *fiber.Ctx) error {
	cert := strings.TrimSpace(c.FormValue("ssl_cert_file"))
	key := strings.TrimSpace(c.FormValue("ssl_key_file"))
	_ = s.db.SetAppSetting("ssl_cert_file", cert)
	_ = s.db.SetAppSetting("ssl_key_file", key)

	if c.FormValue("ssl_enabled") == "on" {
		if cert == "" || key == "" {
			return c.Redirect("/admin?sslerror=paths", fiber.StatusFound)
		}
		if _, err := tls.LoadX509KeyPair(cert, key); err != nil {
			return c.Redirect("/admin?sslerror=load", fiber.StatusFound)
		}
		_ = s.db.SetAppSetting("ssl_enabled", "on")
	} else {
		_ = s.db.SetAppSetting("ssl_enabled", "off")
	}
	return c.Redirect("/admin?sslsaved=1", fiber.StatusFound)
}

// setRegistration enables/disables self-registration.
func (s *Server) setRegistration(c *fiber.Ctx) error {
	if c.FormValue("allow") == "on" {
		_ = s.db.SetAppSetting("registration_enabled", "") // empty clears → default (open)
	} else {
		_ = s.db.SetAppSetting("registration_enabled", "off")
	}
	return c.Redirect("/admin", fiber.StatusFound)
}

// ----- roles -----

func (s *Server) saveRole(c *fiber.Ctx) error {
	slug := strings.ToLower(strings.TrimSpace(c.FormValue("slug")))
	if slug == "" {
		return c.Redirect("/admin", fiber.StatusFound)
	}
	label := strings.TrimSpace(c.FormValue("label"))
	if label == "" {
		label = slug
	}
	role := db.Role{
		Slug:             slug,
		Label:            label,
		CapacityMB:       parseInt64(c.FormValue("capacity_mb"), 0),
		MaxGroups:        parseInt64(c.FormValue("max_groups"), 0),
		CanUseAI:         c.FormValue("can_use_ai") == "on",
		OllamaDailyLimit: parseInt64(c.FormValue("ollama_daily_limit"), 0),
		InviteLimit:      parseInt64(c.FormValue("invite_limit"), 0),
		UploadTypes:      uploadSpecFromForm(c),
	}
	if err := s.db.UpsertRole(role); err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, err.Error())
	}
	return c.Redirect("/admin", fiber.StatusFound)
}

func (s *Server) deleteRole(c *fiber.Ctx) error {
	_ = s.db.DeleteRole(strings.ToLower(strings.TrimSpace(c.FormValue("slug"))))
	return c.Redirect("/admin", fiber.StatusFound)
}

// ----- users -----

func (s *Server) saveUser(c *fiber.Ctx) error {
	uid, _ := strconv.ParseInt(c.FormValue("user_id"), 10, 64)
	if uid == 0 {
		return c.Redirect("/admin/users", fiber.StatusFound)
	}
	if role := strings.TrimSpace(c.FormValue("role_slug")); role != "" {
		_ = s.db.SetUserRole(uid, role)
	}
	// Promote/demote to platform admin (never demote the last admin).
	makeAdmin := c.FormValue("is_admin") == "on"
	if target, err := s.db.GetUser(uid); err == nil && target.IsAdmin != makeAdmin {
		if !makeAdmin {
			if n, _ := s.db.CountAdmins(); n <= 1 {
				makeAdmin = true // refuse to remove the last admin
			}
		}
		if target.IsAdmin != makeAdmin {
			_ = s.db.SetUserAdmin(uid, makeAdmin)
		}
	}
	if capMB := strings.TrimSpace(c.FormValue("capacity_mb")); capMB == "" {
		_ = s.db.SetUserCapacityOverride(uid, nil) // clear -> use role default
	} else {
		v := parseInt64(capMB, 0)
		_ = s.db.SetUserCapacityOverride(uid, &v)
	}
	if od := strings.TrimSpace(c.FormValue("ollama_daily")); od == "" {
		_ = s.db.SetUserOllamaOverride(uid, nil)
	} else {
		v := parseInt64(od, 0)
		_ = s.db.SetUserOllamaOverride(uid, &v)
	}
	if iv := strings.TrimSpace(c.FormValue("invite_override")); iv == "" {
		_ = s.db.SetUserInviteOverride(uid, nil)
	} else {
		v := parseInt64(iv, 0)
		_ = s.db.SetUserInviteOverride(uid, &v)
	}
	return c.Redirect("/admin/users", fiber.StatusFound)
}

func parseInt64(s string, def int64) int64 {
	s = strings.TrimSpace(s)
	if s == "" {
		return def
	}
	v, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		return def
	}
	return v
}
