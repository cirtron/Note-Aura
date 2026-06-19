package server

import (
	"strconv"
	"strings"

	"github.com/gofiber/fiber/v2"
)

// blockUser blocks another user by email. Admins cannot be blocked.
func (s *Server) blockUser(c *fiber.Ctx) error {
	u := currentUser(c)
	email := strings.ToLower(strings.TrimSpace(c.FormValue("email")))
	if email == "" || email == u.Email {
		return c.Redirect("/settings", fiber.StatusFound)
	}
	target, err := s.db.GetUserByEmail(email)
	if err != nil {
		return c.Redirect("/settings?berror=notfound", fiber.StatusFound)
	}
	if target.IsAdmin {
		return c.Redirect("/settings?berror=admin", fiber.StatusFound)
	}
	_ = s.db.AddBlock(u.ID, target.ID)
	return c.Redirect("/settings", fiber.StatusFound)
}

// unblockUser removes a block.
func (s *Server) unblockUser(c *fiber.Ctx) error {
	u := currentUser(c)
	target, _ := strconv.ParseInt(c.FormValue("user_id"), 10, 64)
	_ = s.db.RemoveBlock(u.ID, target)
	return c.Redirect("/settings", fiber.StatusFound)
}
