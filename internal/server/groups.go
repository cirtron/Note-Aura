package server

import (
	"strconv"
	"strings"

	"github.com/gofiber/fiber/v2"

	"note-aura/internal/db"
)

// canCreateGroup reports whether the user may create another group under their
// role's max-groups limit (admins unlimited).
func (s *Server) canCreateGroup(u *db.User) bool {
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
	if r.MaxGroups < 0 {
		return true // unlimited
	}
	if r.MaxGroups == 0 {
		return false // none allowed
	}
	n, _ := s.db.CountGroupsOwned(u.ID)
	return int64(n) < r.MaxGroups
}

func (s *Server) listGroups(c *fiber.Ctx) error {
	u := currentUser(c)
	owned, _ := s.db.ListOwnedGroups(u.ID)
	member, _ := s.db.ListMemberGroups(u.ID)
	invites, _ := s.db.GroupInvitesForUser(u.ID)
	m := baseMap(c, "Groups")
	m["Nav"] = "groups"
	m["Owned"] = owned
	m["Member"] = member
	m["Invites"] = invites
	m["CanCreate"] = s.canCreateGroup(u)
	m["LimitError"] = c.Query("error") == "limit"
	return c.Render("groups", m, "layout")
}

func (s *Server) createGroup(c *fiber.Ctx) error {
	u := currentUser(c)
	if !s.canCreateGroup(u) {
		return c.Redirect("/groups?error=limit", fiber.StatusFound)
	}
	name := strings.TrimSpace(c.FormValue("name"))
	if name == "" {
		return c.Redirect("/groups", fiber.StatusFound)
	}
	id, err := s.db.CreateGroup(u.ID, name)
	if err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, err.Error())
	}
	return c.Redirect("/groups/"+strconv.FormatInt(id, 10), fiber.StatusFound)
}

func (s *Server) viewGroup(c *fiber.Ctx) error {
	u := currentUser(c)
	id, _ := strconv.ParseInt(c.Params("id"), 10, 64)
	g, err := s.db.GetGroup(id)
	if err != nil {
		return fiber.NewError(fiber.StatusNotFound, "group not found")
	}
	isOwner := g.OwnerID == u.ID
	member, _ := s.db.IsGroupMember(id, u.ID)
	if !isOwner && !member {
		return fiber.NewError(fiber.StatusForbidden, "you are not in this group")
	}
	if o, e := s.db.GetUser(g.OwnerID); e == nil {
		g.OwnerEmail = o.Email
	}
	isGroupAdmin := s.db.IsGroupAdmin(id, u.ID)
	members, _ := s.db.GroupMembers(id)
	notes, _ := s.db.NotesSharedToGroup(id)

	m := baseMap(c, g.Name)
	m["Nav"] = "groups"
	m["Group"] = g
	m["IsOwner"] = isOwner
	m["IsGroupAdmin"] = isGroupAdmin
	m["IsMember"] = member
	m["Members"] = members
	m["Notes"] = notes
	if isGroupAdmin {
		m["Pending"], _ = s.db.PendingInvitesForGroup(id)
	}
	return c.Render("group", m, "layout")
}

func (s *Server) deleteGroup(c *fiber.Ctx) error {
	u := currentUser(c)
	id, _ := strconv.ParseInt(c.Params("id"), 10, 64)
	g, err := s.db.GetGroup(id)
	if err != nil || g.OwnerID != u.ID {
		return fiber.NewError(fiber.StatusForbidden, "only the group owner can delete it")
	}
	_ = s.db.DeleteGroup(id)
	return c.Redirect("/groups", fiber.StatusFound)
}

// inviteGroupMember sends a pending invite (the user must accept to join).
func (s *Server) inviteGroupMember(c *fiber.Ctx) error {
	u := currentUser(c)
	id, _ := strconv.ParseInt(c.Params("id"), 10, 64)
	if _, err := s.db.GetGroup(id); err != nil || !s.db.IsGroupAdmin(id, u.ID) {
		return fiber.NewError(fiber.StatusForbidden, "only a group admin can invite members")
	}
	email := strings.ToLower(strings.TrimSpace(c.FormValue("email")))
	canWrite := c.FormValue("can_write") == "on"
	if email == "" || email == u.Email {
		return c.Redirect("/groups/"+strconv.FormatInt(id, 10), fiber.StatusFound)
	}
	target, terr := s.db.GetUserByEmail(email)
	if terr != nil {
		return s.viewGroupWithError(c, id, "No user with that email")
	}
	if s.db.BlockedEitherWay(u.ID, target.ID) {
		return s.viewGroupWithError(c, id, "You can't invite this user")
	}
	_ = s.db.InviteToGroup(id, target.ID, canWrite)
	return c.Redirect("/groups/"+strconv.FormatInt(id, 10), fiber.StatusFound)
}

// cancelInvite lets the owner withdraw a pending invite.
func (s *Server) cancelInvite(c *fiber.Ctx) error {
	u := currentUser(c)
	id, _ := strconv.ParseInt(c.Params("id"), 10, 64)
	if _, err := s.db.GetGroup(id); err != nil || !s.db.IsGroupAdmin(id, u.ID) {
		return fiber.NewError(fiber.StatusForbidden, "only a group admin can manage invites")
	}
	target, _ := strconv.ParseInt(c.FormValue("user_id"), 10, 64)
	_ = s.db.DeleteInvite(id, target)
	return c.Redirect("/groups/"+strconv.FormatInt(id, 10), fiber.StatusFound)
}

// acceptInvite: the invited user joins the group.
func (s *Server) acceptInvite(c *fiber.Ctx) error {
	u := currentUser(c)
	id, _ := strconv.ParseInt(c.Params("id"), 10, 64)
	if err := s.db.AcceptInvite(id, u.ID); err != nil {
		return c.Redirect("/groups", fiber.StatusFound)
	}
	return c.Redirect("/groups/"+strconv.FormatInt(id, 10), fiber.StatusFound)
}

// rejectInvite: the invited user declines.
func (s *Server) rejectInvite(c *fiber.Ctx) error {
	u := currentUser(c)
	id, _ := strconv.ParseInt(c.Params("id"), 10, 64)
	_ = s.db.DeleteInvite(id, u.ID)
	return c.Redirect("/groups", fiber.StatusFound)
}

// leaveGroup: a member leaves; the owner cannot (they delete the group instead).
func (s *Server) leaveGroup(c *fiber.Ctx) error {
	u := currentUser(c)
	id, _ := strconv.ParseInt(c.Params("id"), 10, 64)
	g, err := s.db.GetGroup(id)
	if err != nil {
		return fiber.NewError(fiber.StatusNotFound, "group not found")
	}
	if g.OwnerID == u.ID {
		return fiber.NewError(fiber.StatusForbidden, "the group owner can't leave — delete the group instead")
	}
	_ = s.db.LeaveGroup(id, u.ID)
	return c.Redirect("/groups", fiber.StatusFound)
}

func (s *Server) removeGroupMember(c *fiber.Ctx) error {
	u := currentUser(c)
	id, _ := strconv.ParseInt(c.Params("id"), 10, 64)
	if _, err := s.db.GetGroup(id); err != nil || !s.db.IsGroupAdmin(id, u.ID) {
		return fiber.NewError(fiber.StatusForbidden, "only a group admin can manage members")
	}
	target, _ := strconv.ParseInt(c.FormValue("user_id"), 10, 64)
	_ = s.db.RemoveGroupMember(id, target)
	return c.Redirect("/groups/"+strconv.FormatInt(id, 10), fiber.StatusFound)
}

// setGroupMemberAdmin promotes or demotes a member to/from group admin. Only a
// group admin may do this.
func (s *Server) setGroupMemberAdmin(c *fiber.Ctx) error {
	u := currentUser(c)
	id, _ := strconv.ParseInt(c.Params("id"), 10, 64)
	if _, err := s.db.GetGroup(id); err != nil || !s.db.IsGroupAdmin(id, u.ID) {
		return fiber.NewError(fiber.StatusForbidden, "only a group admin can change group admins")
	}
	target, _ := strconv.ParseInt(c.FormValue("user_id"), 10, 64)
	makeAdmin := c.FormValue("make") == "on" || c.FormValue("make") == "true"
	_ = s.db.SetGroupMemberAdmin(id, target, makeAdmin)
	return c.Redirect("/groups/"+strconv.FormatInt(id, 10), fiber.StatusFound)
}

// setGroupMemberWrite toggles a member's read/write permission (group admin only).
func (s *Server) setGroupMemberWrite(c *fiber.Ctx) error {
	u := currentUser(c)
	id, _ := strconv.ParseInt(c.Params("id"), 10, 64)
	if _, err := s.db.GetGroup(id); err != nil || !s.db.IsGroupAdmin(id, u.ID) {
		return fiber.NewError(fiber.StatusForbidden, "only a group admin can change permissions")
	}
	target, _ := strconv.ParseInt(c.FormValue("user_id"), 10, 64)
	canWrite := c.FormValue("write") == "on" || c.FormValue("write") == "true"
	_ = s.db.SetGroupMemberWrite(id, target, canWrite)
	return c.Redirect("/groups/"+strconv.FormatInt(id, 10), fiber.StatusFound)
}

func (s *Server) viewGroupWithError(c *fiber.Ctx, id int64, msg string) error {
	u := currentUser(c)
	g, err := s.db.GetGroup(id)
	if err != nil {
		return fiber.NewError(fiber.StatusNotFound, "group not found")
	}
	if o, e := s.db.GetUser(g.OwnerID); e == nil {
		g.OwnerEmail = o.Email
	}
	members, _ := s.db.GroupMembers(id)
	notes, _ := s.db.NotesSharedToGroup(id)
	isMember, _ := s.db.IsGroupMember(id, u.ID)
	isGroupAdmin := s.db.IsGroupAdmin(id, u.ID)
	m := baseMap(c, g.Name)
	m["Nav"] = "groups"
	m["Group"] = g
	m["IsOwner"] = g.OwnerID == u.ID
	m["IsGroupAdmin"] = isGroupAdmin
	m["IsMember"] = isMember
	m["Members"] = members
	m["Notes"] = notes
	m["MemberError"] = msg
	if isGroupAdmin {
		m["Pending"], _ = s.db.PendingInvitesForGroup(id)
	}
	return c.Render("group", m, "layout")
}

// ----- sharing a note to a group (from the note view) -----

func (s *Server) shareNoteToGroup(c *fiber.Ctx) error {
	u := currentUser(c)
	id, _ := strconv.ParseInt(c.Params("id"), 10, 64)
	note, err := s.db.GetNote(id)
	if err != nil || note.OwnerID != u.ID {
		return fiber.NewError(fiber.StatusForbidden, "only the owner can share a note")
	}
	groupID, _ := strconv.ParseInt(c.FormValue("group_id"), 10, 64)
	if groupID > 0 && s.db.CanShareToGroup(u.ID, groupID) {
		_ = s.db.ShareNoteToGroup(id, groupID)
	}
	return c.Redirect("/notes/"+strconv.FormatInt(id, 10), fiber.StatusFound)
}

func (s *Server) unshareNoteFromGroup(c *fiber.Ctx) error {
	u := currentUser(c)
	id, _ := strconv.ParseInt(c.Params("id"), 10, 64)
	note, err := s.db.GetNote(id)
	if err != nil || note.OwnerID != u.ID {
		return fiber.NewError(fiber.StatusForbidden, "only the owner can manage sharing")
	}
	groupID, _ := strconv.ParseInt(c.FormValue("group_id"), 10, 64)
	_ = s.db.UnshareNoteFromGroup(id, groupID)
	return c.Redirect("/notes/"+strconv.FormatInt(id, 10), fiber.StatusFound)
}
