package server

import (
	"strconv"
	"strings"

	"github.com/gofiber/fiber/v2"

	"note-aura/internal/db"
)

func (s *Server) shareNote(c *fiber.Ctx) error {
	u := currentUser(c)
	id, _ := strconv.ParseInt(c.Params("id"), 10, 64)
	note, err := s.db.GetNote(id)
	if err != nil || note.OwnerID != u.ID {
		return fiber.NewError(fiber.StatusForbidden, "only the owner can share a note")
	}
	email := strings.ToLower(strings.TrimSpace(c.FormValue("email")))
	canEdit := c.FormValue("can_edit") == "on" || c.FormValue("can_edit") == "true"
	if email == u.Email {
		return c.Redirect("/notes/"+c.Params("id"), fiber.StatusFound)
	}
	// Refuse to share if either party has blocked the other.
	if target, terr := s.db.GetUserByEmail(email); terr == nil && s.db.BlockedEitherWay(u.ID, target.ID) {
		return s.viewNoteWithShareError(c, id, "You can't share with this user")
	}
	if err := s.db.ShareNote(id, email, canEdit); err != nil {
		if err == db.ErrNotFound {
			return s.viewNoteWithShareError(c, id, "No user with that email")
		}
		return fiber.NewError(fiber.StatusInternalServerError, err.Error())
	}
	return c.Redirect("/notes/"+c.Params("id"), fiber.StatusFound)
}

func (s *Server) unshareNote(c *fiber.Ctx) error {
	u := currentUser(c)
	id, _ := strconv.ParseInt(c.Params("id"), 10, 64)
	note, err := s.db.GetNote(id)
	if err != nil || note.OwnerID != u.ID {
		return fiber.NewError(fiber.StatusForbidden, "only the owner can manage sharing")
	}
	targetID, _ := strconv.ParseInt(c.FormValue("user_id"), 10, 64)
	if err := s.db.UnshareNote(id, targetID); err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, err.Error())
	}
	return c.Redirect("/notes/"+c.Params("id"), fiber.StatusFound)
}

func (s *Server) listShared(c *fiber.Ctx) error {
	u := currentUser(c)
	notes, err := s.db.ListSharedWithUser(u.ID)
	if err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, err.Error())
	}
	m := baseMap(c, "Shared with me")
	m["Notes"] = notes
	m["Shared"] = true
	m["Nav"] = "shared"
	return c.Render("notes_list", m, "layout")
}

func (s *Server) viewNoteWithShareError(c *fiber.Ctx, id int64, msg string) error {
	u := currentUser(c)
	note, err := s.db.GetNote(id)
	if err != nil {
		return fiber.NewError(fiber.StatusNotFound, "note not found")
	}
	shares, _ := s.db.SharesForNote(id)
	m := baseMap(c, note.Title)
	m["Note"] = note
	m["CanEdit"] = note.OwnerID == u.ID
	m["IsOwner"] = note.OwnerID == u.ID
	m["Shares"] = shares
	m["ShareError"] = msg
	m["Nav"] = "notes"
	return c.Render("note_view", m, "layout")
}
