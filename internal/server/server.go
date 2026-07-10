// Package server wires the Fiber HTTP server, templates, and routes.
package server

import (
	"fmt"
	"html/template"
	"io/fs"
	"net/http"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/template/html/v2"

	"note-aura/internal/ai"
	"note-aura/internal/config"
	"note-aura/internal/db"
	"note-aura/internal/i18n"
	"note-aura/internal/ingest"
	"note-aura/internal/mailer"
	"note-aura/internal/markdown"
	"note-aura/internal/syslog"
	"note-aura/internal/worker"
)

// app_settings keys for organization branding.
const (
	brandLogoKey = "brand_logo_path"
	brandTextKey = "brand_text"
)

// Brand is the organization branding shown in the header and on the login page.
type Brand struct {
	LogoPath string
	Text     string
}

// Server holds shared dependencies for handlers.
type Server struct {
	App        *fiber.App
	cfg        *config.Config
	db         *db.DB
	fallback   ai.GlobalConfig // config.yaml AI defaults; admin app_settings overlay it
	worker     *worker.Worker
	mailer     *mailer.Mailer
	uploadsDir string
	startTime  time.Time
}

// providerFor builds the AI provider for a user from the live admin config and
// their optional cloud override.
func (s *Server) providerFor(userID int64) ai.Provider {
	app, _ := s.db.GetAppSettings()
	global := ai.LoadGlobal(app, s.fallback)
	settings, _ := s.db.GetUserSettings(userID)
	// Users on their own external AI server may customize their own prompts; the
	// overlay only applies on the cloud branch, so this is a no-op for Ollama.
	return ai.BuildProvider(global, settings, true)
}

// New builds the Fiber app with templates and routes registered.
func New(cfg *config.Config, database *db.DB, fallback ai.GlobalConfig, wk *worker.Worker, mail *mailer.Mailer, templates fs.FS) *Server {
	engine := html.NewFileSystem(http.FS(templates), ".html")

	engine.AddFunc("mdHTML", func(s string) template.HTML { return markdown.Render(s) })
	engine.AddFunc("date", func(t time.Time) string { return t.Format("2006-01-02 15:04") })
	engine.AddFunc("dateTZ", func(t time.Time, tz string) string {
		loc := time.UTC
		if tz != "" {
			if l, err := time.LoadLocation(tz); err == nil {
				loc = l
			}
		}
		return t.In(loc).Format("2006-01-02 15:04")
	})
	engine.AddFunc("uploadCats", func() []string { return uploadCategories })
	engine.AddFunc("mul", func(a, b int) int { return a * b })
	engine.AddFunc("mailEnabled", func() bool { return mail.Enabled() })
	engine.AddFunc("aidur", func(ms int64) string {
		switch {
		case ms <= 0:
			return ""
		case ms < 1000:
			return fmt.Sprintf("%dms", ms)
		default:
			return fmt.Sprintf("%.1fs", float64(ms)/1000)
		}
	})
	engine.AddFunc("countryName", countryName)
	engine.AddFunc("isFacebook", ingest.IsFacebook)
	engine.AddFunc("t", i18n.T)
	engine.AddFunc("langs", func() []i18n.Language { return i18n.Languages })
	engine.AddFunc("suspendedUntilLabel", suspendedUntilLabel)
	engine.AddFunc("contains", strings.Contains)
	// brand exposes the admin-configured logo/wording to every template (header,
	// login). Falls back to the product name when unset.
	engine.AddFunc("brand", func() Brand {
		app, _ := database.GetAppSettings()
		return Brand{LogoPath: app[brandLogoKey], Text: app[brandTextKey]}
	})

	app := fiber.New(fiber.Config{
		Views:     engine,
		BodyLimit: 25 * 1024 * 1024, // 25 MB for image uploads
		ErrorHandler: func(c *fiber.Ctx, err error) error {
			code := fiber.StatusInternalServerError
			if e, ok := err.(*fiber.Error); ok {
				code = e.Code
			}
			if code >= 500 { // record server-side errors to the admin log
				syslog.Errorf("server", "%s %s -> %d: %v", c.Method(), c.OriginalURL(), code, err)
			}
			return c.Status(code).SendString(err.Error())
		},
	})
	s := &Server{App: app, cfg: cfg, db: database, fallback: fallback, worker: wk, mailer: mail, uploadsDir: cfg.UploadsDir, startTime: time.Now()}

	app.Use(s.loadSession)
	app.Use(s.detectLang)
	app.Use(s.announcementMiddleware)

	// Language switcher (available to everyone).
	app.Get("/lang/:code", s.setLang)

	// Auth.
	app.Get("/login", s.getLogin)
	app.Post("/login", s.postLogin)
	app.Get("/register", s.getRegister)
	app.Post("/register", s.postRegister)
	app.Get("/verify", s.getVerify)
	app.Post("/verify/resend", s.postResendVerify)
	app.Get("/forgot", s.getForgot)
	app.Post("/forgot", s.postForgot)
	app.Get("/reset", s.getReset)
	app.Post("/reset", s.postReset)
	app.Post("/logout", s.postLogout)

	// Invite a new user to the platform.
	app.Post("/invite", s.requireAuth, s.inviteUser)

	app.Get("/", func(c *fiber.Ctx) error {
		if currentUser(c) != nil {
			return c.Redirect("/notes", fiber.StatusFound)
		}
		return c.Redirect("/login", fiber.StatusFound)
	})

	// Notes (auth required).
	app.Get("/notes", s.requireAuth, s.listNotes)
	app.Get("/notes/new", s.requireAuth, s.newNoteForm)
	app.Post("/notes", s.requireAuth, s.createNote)
	app.Post("/notes/bulk-delete", s.requireAuth, s.bulkDeleteNotes)
	app.Get("/notes/export", s.requireAuth, s.exportNotes)
	app.Get("/notes/export/md", s.requireAuth, s.exportMarkdownZip)
	app.Post("/notes/import", s.requireAuth, s.importNotes)
	app.Post("/capture", s.requireAuth, s.capture)
	app.Get("/notes/:id", s.requireAuth, s.viewNote)
	app.Get("/notes/:id/status", s.requireAuth, s.noteStatus)
	app.Get("/notes/:id/edit", s.requireAuth, s.editNoteForm)
	app.Post("/notes/:id", s.requireAuth, s.updateNote)
	app.Post("/notes/:id/delete", s.requireAuth, s.deleteNote)
	app.Post("/notes/:id/retry", s.requireAuth, s.retryNote)
	app.Post("/notes/:id/stop", s.requireAuth, s.stopNote)
	app.Post("/notes/:id/share", s.requireAuth, s.shareNote)
	app.Post("/notes/:id/unshare", s.requireAuth, s.unshareNote)
	app.Post("/notes/:id/share-group", s.requireAuth, s.shareNoteToGroup)
	app.Post("/notes/:id/unshare-group", s.requireAuth, s.unshareNoteFromGroup)

	// Groups.
	app.Get("/groups", s.requireAuth, s.listGroups)
	app.Post("/groups", s.requireAuth, s.createGroup)
	app.Get("/groups/:id", s.requireAuth, s.viewGroup)
	app.Post("/groups/:id/delete", s.requireAuth, s.deleteGroup)
	app.Post("/groups/:id/invite", s.requireAuth, s.inviteGroupMember)
	app.Post("/groups/:id/invite/cancel", s.requireAuth, s.cancelInvite)
	app.Post("/groups/:id/accept", s.requireAuth, s.acceptInvite)
	app.Post("/groups/:id/reject", s.requireAuth, s.rejectInvite)
	app.Post("/groups/:id/leave", s.requireAuth, s.leaveGroup)
	app.Post("/groups/:id/members/remove", s.requireAuth, s.removeGroupMember)
	app.Post("/groups/:id/members/admin", s.requireAuth, s.setGroupMemberAdmin)
	app.Post("/groups/:id/members/write", s.requireAuth, s.setGroupMemberWrite)

	// Blocking.
	app.Post("/settings/block", s.requireAuth, s.blockUser)
	app.Post("/settings/unblock", s.requireAuth, s.unblockUser)

	// Inline image upload from the Markdown editor.
	app.Post("/upload/image", s.requireAuth, s.uploadImage)

	// Shared with me.
	app.Get("/shared", s.requireAuth, s.listShared)

	// Calendar.
	app.Get("/calendar", s.requireAuth, s.getCalendar)

	// Ask your notes (RAG).
	app.Get("/ask", s.requireAuth, s.askForm)
	app.Post("/ask", s.requireAuth, s.ask)

	// Settings.
	app.Get("/settings", s.requireAuth, s.getSettings)
	app.Post("/settings", s.requireAuth, s.postSettings)
	app.Post("/settings/email-token", s.requireAuth, s.regenerateEmailToken)

	// User guide.
	app.Get("/guide", s.requireAuth, s.getGuide)
	app.Get("/admin/guide", s.requireAuth, s.requireAdmin, s.getAdminGuide)
	app.Post("/admin/guide", s.requireAuth, s.requireAdmin, s.postAdminGuide)

	// Admin dashboard.
	app.Get("/dashboard", s.requireAuth, s.requireAdmin, s.getDashboard)

	// Admin: global AI models + prompts.
	app.Get("/admin", s.requireAuth, s.requireAdmin, s.getAdmin)
	app.Post("/admin", s.requireAuth, s.requireAdmin, s.postAdmin)
	app.Post("/admin/branding", s.requireAuth, s.requireAdmin, s.postAdminBranding)
	app.Post("/admin/registration", s.requireAuth, s.requireAdmin, s.setRegistration)
	app.Get("/admin/logs", s.requireAuth, s.requireAdmin, s.getLogs)
	app.Post("/admin/logs/clear", s.requireAuth, s.requireAdmin, s.clearLogs)
	app.Post("/admin/ssl", s.requireAuth, s.requireAdmin, s.setSSL)
	app.Post("/admin/holidays/import", s.requireAuth, s.requireAdmin, s.importHolidays)
	app.Post("/admin/holidays/upload", s.requireAuth, s.requireAdmin, s.uploadHolidays)
	app.Post("/admin/holidays/delete", s.requireAuth, s.requireAdmin, s.deleteHolidays)
	app.Post("/admin/roles", s.requireAuth, s.requireAdmin, s.saveRole)
	app.Post("/admin/roles/delete", s.requireAuth, s.requireAdmin, s.deleteRole)
	app.Get("/admin/users", s.requireAuth, s.requireAdmin, s.getUsers)
	app.Post("/admin/users", s.requireAuth, s.requireAdmin, s.saveUser)
	app.Post("/admin/users/create", s.requireAuth, s.requireAdmin, s.createUserAdmin)
	app.Post("/admin/users/suspend", s.requireAuth, s.requireAdmin, s.suspendUser)
	app.Post("/admin/users/delete", s.requireAuth, s.requireAdmin, s.deleteUser)
	app.Post("/admin/users/force-logout", s.requireAuth, s.requireAdmin, s.postAdminForceLogout)
	app.Post("/admin/users/clear-lock", s.requireAuth, s.requireAdmin, s.postAdminClearLock)
	app.Post("/admin/invitations/delete", s.requireAuth, s.requireAdmin, s.adminDeleteInvitation)
	app.Post("/admin/invitations/resend", s.requireAuth, s.requireAdmin, s.adminResendInvitation)
	app.Get("/admin/ip-block", s.requireAuth, s.requireAdmin, s.getAdminIPBlock)
	app.Post("/admin/ip-block/add", s.requireAuth, s.requireAdmin, s.postAdminBlockIP)
	app.Post("/admin/ip-block/remove", s.requireAuth, s.requireAdmin, s.postAdminUnblockIP)
	app.Get("/admin/lockout", s.requireAuth, s.requireAdmin, s.getAdminLockout)
	app.Post("/admin/lockout", s.requireAuth, s.requireAdmin, s.postAdminLockout)
	app.Get("/admin/announcement", s.requireAuth, s.requireAdmin, s.getAdminAnnouncement)
	app.Post("/admin/announcement", s.requireAuth, s.requireAdmin, s.postAdminAnnouncement)
	app.Get("/admin/email", s.requireAuth, s.requireAdmin, s.getAdminEmail)
	app.Post("/admin/email", s.requireAuth, s.requireAdmin, s.postAdminEmail)
	app.Post("/admin/invite", s.requireAuth, s.requireAdmin, s.postAdminInvite)
	app.Get("/admin/banned-usernames", s.requireAuth, s.requireAdmin, s.getAdminBannedUsernames)
	app.Post("/admin/banned-usernames/add", s.requireAuth, s.requireAdmin, s.postAdminAddBannedUsername)
	app.Post("/admin/banned-usernames/remove", s.requireAuth, s.requireAdmin, s.postAdminRemoveBannedUsername)
	app.Get("/admin/banned-emails", s.requireAuth, s.requireAdmin, s.getAdminBannedEmails)
	app.Post("/admin/banned-emails/add", s.requireAuth, s.requireAdmin, s.postAdminAddBannedEmail)
	app.Post("/admin/banned-emails/remove", s.requireAuth, s.requireAdmin, s.postAdminRemoveBannedEmail)
	app.Post("/invite/delete", s.requireAuth, s.deleteInvitation)
	app.Post("/invite/resend", s.requireAuth, s.resendInvitation)

	// User: choose which countries' holidays show on the calendar.
	app.Post("/settings/holidays", s.requireAuth, s.setHolidayCountries)

	// Uploaded files.
	app.Static("/uploads", cfg.UploadsDir)

	return s
}

// Listen starts the HTTP server.
func (s *Server) Listen() error {
	if cert, key, ok := s.effectiveTLS(); ok {
		return s.App.ListenTLS(s.cfg.ListenAddr, cert, key)
	}
	return s.App.Listen(s.cfg.ListenAddr)
}

// effectiveTLS resolves whether to serve HTTPS, and with which cert/key. The
// admin setting (app_settings ssl_enabled = "on"/"off") overrides config.yaml;
// when unset, config.yaml's tls: block applies.
func (s *Server) effectiveTLS() (cert, key string, ok bool) {
	app, _ := s.db.GetAppSettings()
	switch app["ssl_enabled"] {
	case "on":
		cert, key = app["ssl_cert_file"], app["ssl_key_file"]
		return cert, key, cert != "" && key != ""
	case "off":
		return "", "", false
	default:
		return s.cfg.TLS.CertFile, s.cfg.TLS.KeyFile, s.cfg.TLS.Enabled()
	}
}

// LinkBase returns the base URL for links in emails, with the scheme forced to
// match how the server is actually serving (https when TLS is on, else http) —
// regardless of the scheme written in base_url. Host/port come from base_url.
func (s *Server) LinkBase() string {
	b := strings.TrimRight(s.cfg.BaseURL, "/")
	if i := strings.Index(b, "://"); i >= 0 {
		b = b[i+3:] // drop any existing scheme
	}
	scheme := "http"
	if _, _, https := s.effectiveTLS(); https {
		scheme = "https"
	}
	return scheme + "://" + b
}
