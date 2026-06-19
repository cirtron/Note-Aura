package server

import (
	"fmt"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"

	"note-aura/internal/auth"
	"note-aura/internal/i18n"
	"note-aura/internal/syslog"
)

// registrationOpen reports whether self-registration is enabled (admin setting).
func (s *Server) registrationOpen() bool {
	app, _ := s.db.GetAppSettings()
	return app["registration_enabled"] != "off"
}

// canRegister allows a new sign-up when registration is open, the user holds a
// valid invite, or no admin exists yet (first-run bootstrap).
func (s *Server) canRegister(inviteToken string) bool {
	if admins, _ := s.db.CountAdmins(); admins == 0 {
		return true
	}
	if s.registrationOpen() {
		return true
	}
	if inviteToken != "" {
		if inv, err := s.db.GetInvitationByToken(inviteToken); err == nil && !inv.Accepted {
			return true
		}
	}
	return false
}

func (s *Server) getLogin(c *fiber.Ctx) error {
	if currentUser(c) != nil {
		return c.Redirect("/notes", fiber.StatusFound)
	}
	m := fiber.Map{"Title": "Sign in"}
	if !s.registrationOpen() {
		m["RegClosed"] = true
	}
	return c.Render("login", withLang(c, m), "layout")
}

func (s *Server) postLogin(c *fiber.Ctx) error {
	email := strings.ToLower(strings.TrimSpace(c.FormValue("email")))
	password := c.FormValue("password")

	user, err := s.db.GetUserByEmail(email)
	if err != nil || !auth.VerifyPassword(user.PasswordHash, password) {
		c.Status(fiber.StatusUnauthorized)
		return c.Render("login", withLang(c, fiber.Map{"Title": "Sign in", "Error": "Invalid email or password"}), "layout")
	}
	if user.Suspended {
		c.Status(fiber.StatusForbidden)
		return c.Render("login", withLang(c, fiber.Map{"Title": "Sign in",
			"Error": "Your account has been suspended. Contact an administrator."}), "layout")
	}
	if !user.EmailVerified {
		c.Status(fiber.StatusForbidden)
		return c.Render("login", withLang(c, fiber.Map{"Title": "Sign in",
			"Error":      "Please verify your email before signing in — check your inbox.",
			"Unverified": true, "Email": email}), "layout")
	}
	return s.startSession(c, user.ID)
}

func (s *Server) getRegister(c *fiber.Ctx) error {
	if currentUser(c) != nil {
		return c.Redirect("/notes", fiber.StatusFound)
	}
	token := c.Query("invite")
	if !s.canRegister(token) {
		return c.Render("login", withLang(c, fiber.Map{"Title": "Sign in", "RegClosed": true,
			"Error": "New sign-ups are currently closed. Ask an admin for an invitation."}), "layout")
	}
	m := fiber.Map{"Title": "Create account"}
	if token != "" {
		if inv, err := s.db.GetInvitationByToken(token); err == nil && !inv.Accepted {
			m["Email"] = inv.Email
			m["InviteToken"] = token
		}
	}
	return c.Render("register", withLang(c, m), "layout")
}

func (s *Server) postRegister(c *fiber.Ctx) error {
	email := strings.ToLower(strings.TrimSpace(c.FormValue("email")))
	password := c.FormValue("password")
	inviteToken := strings.TrimSpace(c.FormValue("invite_token"))

	if !s.canRegister(inviteToken) {
		return c.Render("login", withLang(c, fiber.Map{"Title": "Sign in", "RegClosed": true,
			"Error": "New sign-ups are currently closed."}), "layout")
	}

	render := func(msg string) error {
		c.Status(fiber.StatusBadRequest)
		return c.Render("register", withLang(c, fiber.Map{"Title": "Create account", "Error": msg,
			"Email": email, "InviteToken": inviteToken}), "layout")
	}
	if err := auth.ValidateEmail(email); err != nil {
		return render("Please enter a valid email address")
	}
	if _, err := s.db.GetUserByEmail(email); err == nil {
		return render("An account with that email already exists")
	}
	hash, err := auth.HashPassword(password)
	if err != nil {
		return render(err.Error())
	}
	// Bootstrap: the first account (when no admin exists yet) becomes the admin.
	admins, _ := s.db.CountAdmins()
	isAdmin := admins == 0

	// A valid invite for this email proves ownership → register pre-verified.
	invited := false
	if inviteToken != "" {
		if inv, err := s.db.GetInvitationByToken(inviteToken); err == nil && !inv.Accepted &&
			strings.EqualFold(strings.TrimSpace(inv.Email), email) {
			invited = true
		}
	}
	// Require email verification when SMTP is configured and the user isn't the
	// bootstrap admin or coming through an invite.
	verified := isAdmin || invited || !s.mailer.Enabled()
	verifyToken := ""
	if !verified {
		verifyToken, _ = auth.NewToken()
	}

	id, err := s.db.CreateUser(email, hash, isAdmin, verified, verifyToken)
	if err != nil {
		return render("Could not create account")
	}
	if invited {
		_ = s.db.MarkInvitationAccepted(inviteToken)
	}
	if !verified {
		s.sendVerifyEmail(email, verifyToken, currentLang(c))
		return c.Render("login", withLang(c, fiber.Map{"Title": "Sign in",
			"Notice": "Account created. We've emailed you a verification link — verify, then sign in.",
			"Email":  email, "Unverified": true}), "layout")
	}
	return s.startSession(c, id)
}

func (s *Server) getVerify(c *fiber.Ctx) error {
	user, err := s.db.GetUserByVerifyToken(c.Query("token"))
	if err != nil {
		return c.Render("login", withLang(c, fiber.Map{"Title": "Sign in",
			"Error": "Invalid or expired verification link."}), "layout")
	}
	_ = s.db.SetEmailVerified(user.ID)
	return s.startSession(c, user.ID) // verified → sign in
}

func (s *Server) postResendVerify(c *fiber.Ctx) error {
	email := strings.ToLower(strings.TrimSpace(c.FormValue("email")))
	if user, err := s.db.GetUserByEmail(email); err == nil && !user.EmailVerified {
		token, _ := auth.NewToken()
		_ = s.db.SetVerifyToken(user.ID, token)
		s.sendVerifyEmail(email, token, currentLang(c))
	}
	return c.Render("login", withLang(c, fiber.Map{"Title": "Sign in",
		"Notice": "If that account still needs verification, we've sent a new link."}), "layout")
}

// getForgot shows the "request a password reset" form.
func (s *Server) getForgot(c *fiber.Ctx) error {
	if currentUser(c) != nil {
		return c.Redirect("/notes", fiber.StatusFound)
	}
	return c.Render("forgot", withLang(c, fiber.Map{"Title": "Reset password"}), "layout")
}

// postForgot emails a reset link when the address matches an account. The
// response is the same whether or not it does (no account enumeration).
func (s *Server) postForgot(c *fiber.Ctx) error {
	email := strings.ToLower(strings.TrimSpace(c.FormValue("email")))
	if user, err := s.db.GetUserByEmail(email); err == nil && !user.Suspended {
		if token, terr := auth.NewToken(); terr == nil {
			_ = s.db.SetResetToken(user.ID, token, time.Now().Add(time.Hour).Unix())
			s.sendResetEmail(email, token, currentLang(c))
		}
	}
	return c.Render("forgot", withLang(c, fiber.Map{"Title": "Reset password",
		"Notice": "If an account with that email exists, we've sent a password-reset link. It's valid for 1 hour."}), "layout")
}

// getReset shows the "set a new password" form for a valid, unexpired token.
func (s *Server) getReset(c *fiber.Ctx) error {
	token := c.Query("token")
	if _, err := s.db.GetUserByResetToken(token); err != nil {
		return c.Render("login", withLang(c, fiber.Map{"Title": "Sign in",
			"Error": "That password-reset link is invalid or has expired."}), "layout")
	}
	return c.Render("reset", withLang(c, fiber.Map{"Title": "Set a new password", "Token": token}), "layout")
}

// postReset sets a new password from a valid reset token.
func (s *Server) postReset(c *fiber.Ctx) error {
	token := strings.TrimSpace(c.FormValue("token"))
	password := c.FormValue("password")
	user, err := s.db.GetUserByResetToken(token)
	if err != nil {
		return c.Render("login", withLang(c, fiber.Map{"Title": "Sign in",
			"Error": "That password-reset link is invalid or has expired."}), "layout")
	}
	if len(password) < 6 {
		return c.Render("reset", withLang(c, fiber.Map{"Title": "Set a new password", "Token": token,
			"Error": "Password must be at least 6 characters."}), "layout")
	}
	hash, err := auth.HashPassword(password)
	if err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, err.Error())
	}
	if err := s.db.SetPassword(user.ID, hash); err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, err.Error())
	}
	// Completing a reset proves control of the email address.
	_ = s.db.SetEmailVerified(user.ID)
	return c.Render("login", withLang(c, fiber.Map{"Title": "Sign in",
		"Notice": "Your password has been reset — sign in with your new password.", "Email": user.Email}), "layout")
}

// sendResetEmail emails a password-reset link in lang (no-op without SMTP).
func (s *Server) sendResetEmail(to, token, lang string) {
	if !s.mailer.Enabled() {
		return
	}
	link := s.LinkBase() + "/reset?token=" + token
	subject := i18n.T(lang, "email.reset.subject")
	body := fmt.Sprintf(i18n.T(lang, "email.reset.body"), link)
	go func() { _ = s.mailer.Send(to, subject, body) }()
}

// sendVerifyEmail emails a verification link in lang (no-op without SMTP).
func (s *Server) sendVerifyEmail(to, token, lang string) {
	if !s.mailer.Enabled() {
		return
	}
	link := s.LinkBase() + "/verify?token=" + token
	subject := i18n.T(lang, "email.verify.subject")
	body := fmt.Sprintf(i18n.T(lang, "email.verify.body"), link)
	go func() { _ = s.mailer.Send(to, subject, body) }()
}

// sendInviteEmail emails a new-user invitation link in lang (no-op without SMTP).
func (s *Server) sendInviteEmail(to, inviter, token, lang string) {
	if !s.mailer.Enabled() {
		return
	}
	link := s.LinkBase() + "/register?invite=" + token
	subject := fmt.Sprintf(i18n.T(lang, "email.invite.subject"), inviter)
	body := fmt.Sprintf(i18n.T(lang, "email.invite.body"), inviter, link)
	go func() { _ = s.mailer.Send(to, subject, body) }()
}

func (s *Server) startSession(c *fiber.Ctx, userID int64) error {
	sid, err := auth.NewToken()
	if err != nil {
		syslog.Errorf("auth", "generate session token: %v", err)
		return fiber.NewError(fiber.StatusInternalServerError, "session error")
	}
	exp := time.Now().Add(time.Duration(s.cfg.Session.TTLHours) * time.Hour)
	if err := s.db.CreateSession(sid, userID, exp); err != nil {
		syslog.Errorf("auth", "create session for user %d: %v", userID, err)
		return fiber.NewError(fiber.StatusInternalServerError, "session error")
	}
	c.Cookie(&fiber.Cookie{
		Name:     s.cfg.Session.CookieName,
		Value:    sid,
		Expires:  exp,
		HTTPOnly: true,
		SameSite: "Lax",
		Secure:   s.cfg.Session.Secure,
		Path:     "/",
	})
	return c.Redirect("/notes", fiber.StatusFound)
}

func (s *Server) postLogout(c *fiber.Ctx) error {
	if sid := c.Cookies(s.cfg.Session.CookieName); sid != "" {
		_ = s.db.DeleteSession(sid)
	}
	c.Cookie(&fiber.Cookie{
		Name:     s.cfg.Session.CookieName,
		Value:    "",
		Expires:  time.Now().Add(-time.Hour),
		HTTPOnly: true,
		SameSite: "Lax",
		Secure:   s.cfg.Session.Secure,
		Path:     "/",
	})
	return c.Redirect("/login", fiber.StatusFound)
}
