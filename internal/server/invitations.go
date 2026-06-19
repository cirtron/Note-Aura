package server

import (
	"strconv"
	"strings"

	"github.com/gofiber/fiber/v2"

	"note-aura/internal/auth"
	"note-aura/internal/db"
	"note-aura/internal/i18n"
)

// inviteAllowance returns the user's invite limit (<0 unlimited, 0 none) and how
// many invites they have left.
func (s *Server) inviteAllowance(u *db.User) (limit, remaining int64) {
	if u == nil {
		return 0, 0
	}
	limit = s.db.InviteLimit(u.ID)
	if limit <= 0 {
		return limit, 0
	}
	used, _ := s.db.CountInvitationsBy(u.ID)
	remaining = limit - int64(used)
	if remaining < 0 {
		remaining = 0
	}
	return limit, remaining
}

func (s *Server) inviteUser(c *fiber.Ctx) error {
	u := currentUser(c)
	limit, remaining := s.inviteAllowance(u)
	if limit == 0 {
		return c.Redirect("/settings?ierror=none", fiber.StatusFound)
	}
	if limit > 0 && remaining <= 0 {
		return c.Redirect("/settings?ierror=limit", fiber.StatusFound)
	}
	email := strings.ToLower(strings.TrimSpace(c.FormValue("email")))
	if auth.ValidateEmail(email) != nil {
		return c.Redirect("/settings?ierror=invalid", fiber.StatusFound)
	}
	if _, err := s.db.GetUserByEmail(email); err == nil {
		return c.Redirect("/settings?ierror=exists", fiber.StatusFound)
	}
	lang := strings.TrimSpace(c.FormValue("lang"))
	if !i18n.Supported(lang) {
		lang = currentLang(c)
	}
	token, err := auth.NewToken()
	if err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, "token error")
	}
	if err := s.db.CreateInvitation(u.ID, email, token, lang); err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, err.Error())
	}
	s.sendInviteEmail(email, u.Email, token, lang)
	return c.Redirect("/settings?isent=1", fiber.StatusFound)
}

// deleteInvitation lets a user remove one of their own invitations.
func (s *Server) deleteInvitation(c *fiber.Ctx) error {
	u := currentUser(c)
	id, _ := strconv.ParseInt(c.FormValue("id"), 10, 64)
	if inv, err := s.db.GetInvitation(id); err == nil && inv.InviterID == u.ID {
		_ = s.db.DeleteInvitation(id)
	}
	return c.Redirect("/settings", fiber.StatusFound)
}

// resendInvitation re-sends the invite email for a user's own pending invitation.
func (s *Server) resendInvitation(c *fiber.Ctx) error {
	u := currentUser(c)
	id, _ := strconv.ParseInt(c.FormValue("id"), 10, 64)
	if inv, err := s.db.GetInvitation(id); err == nil && inv.InviterID == u.ID && !inv.Accepted {
		s.sendInviteEmail(inv.Email, u.Email, inv.Token, inv.Lang)
	}
	return c.Redirect("/settings?iresent=1", fiber.StatusFound)
}

// adminResendInvitation lets an admin re-send any pending invitation (Users tab).
func (s *Server) adminResendInvitation(c *fiber.Ctx) error {
	id, _ := strconv.ParseInt(c.FormValue("id"), 10, 64)
	if inv, err := s.db.GetInvitation(id); err == nil && !inv.Accepted {
		inviter := ""
		if iu, e := s.db.GetUser(inv.InviterID); e == nil {
			inviter = iu.Email
		}
		s.sendInviteEmail(inv.Email, inviter, inv.Token, inv.Lang)
	}
	return c.Redirect("/admin/users", fiber.StatusFound)
}

// adminDeleteInvitation lets an admin remove any invitation (from the Users tab).
func (s *Server) adminDeleteInvitation(c *fiber.Ctx) error {
	id, _ := strconv.ParseInt(c.FormValue("id"), 10, 64)
	_ = s.db.DeleteInvitation(id)
	return c.Redirect("/admin/users", fiber.StatusFound)
}
