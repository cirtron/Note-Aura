package db

import (
	"database/sql"
	"strings"
)

// CreateGroup creates a group owned by ownerID.
func (d *DB) CreateGroup(ownerID int64, name string) (int64, error) {
	res, err := d.SQL.Exec(`INSERT INTO user_groups (owner_id, name) VALUES (?, ?)`, ownerID, strings.TrimSpace(name))
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

// CountGroupsOwned returns how many groups a user owns (for the create limit).
func (d *DB) CountGroupsOwned(ownerID int64) (int, error) {
	var n int
	err := d.SQL.QueryRow(`SELECT COUNT(*) FROM user_groups WHERE owner_id=?`, ownerID).Scan(&n)
	return n, err
}

// GetGroup returns a group or ErrNotFound.
func (d *DB) GetGroup(id int64) (*Group, error) {
	var g Group
	err := d.SQL.QueryRow(`SELECT id, owner_id, name, created_at FROM user_groups WHERE id=?`, id).
		Scan(&g.ID, &g.OwnerID, &g.Name, &g.CreatedAt)
	if err != nil {
		if isNoRows(err) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return &g, nil
}

// DeleteGroup removes a group (owner only is enforced by the caller).
func (d *DB) DeleteGroup(id int64) error {
	_, err := d.SQL.Exec(`DELETE FROM user_groups WHERE id=?`, id)
	return err
}

// AllGroups returns all groups in the system (for admin UI), with member counts.
func (d *DB) AllGroups() ([]*Group, error) {
	rows, err := d.SQL.Query(`
		SELECT g.id, g.owner_id, g.name, g.created_at,
		       (SELECT COUNT(*) FROM group_members m WHERE m.group_id=g.id)
		FROM user_groups g ORDER BY g.name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []*Group
	for rows.Next() {
		var g Group
		if err := rows.Scan(&g.ID, &g.OwnerID, &g.Name, &g.CreatedAt, &g.MemberCount); err != nil {
			return nil, err
		}
		out = append(out, &g)
	}
	return out, rows.Err()
}

// ListOwnedGroups lists groups a user owns, with member counts.
func (d *DB) ListOwnedGroups(ownerID int64) ([]*Group, error) {
	rows, err := d.SQL.Query(`
		SELECT g.id, g.owner_id, g.name, g.created_at,
		       (SELECT COUNT(*) FROM group_members m WHERE m.group_id=g.id)
		FROM user_groups g WHERE g.owner_id=? ORDER BY g.name`, ownerID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []*Group
	for rows.Next() {
		var g Group
		if err := rows.Scan(&g.ID, &g.OwnerID, &g.Name, &g.CreatedAt, &g.MemberCount); err != nil {
			return nil, err
		}
		g.IsOwner = true
		out = append(out, &g)
	}
	return out, rows.Err()
}

// ListMemberGroups lists groups a user belongs to (not as owner), with their
// write permission and the owner's email.
func (d *DB) ListMemberGroups(userID int64) ([]*Group, error) {
	rows, err := d.SQL.Query(`
		SELECT g.id, g.owner_id, g.name, g.created_at, m.can_write, u.email
		FROM group_members m
		JOIN user_groups g ON g.id = m.group_id
		JOIN users u ON u.id = g.owner_id
		WHERE m.user_id=? ORDER BY g.name`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []*Group
	for rows.Next() {
		var g Group
		if err := rows.Scan(&g.ID, &g.OwnerID, &g.Name, &g.CreatedAt, &g.CanWrite, &g.OwnerEmail); err != nil {
			return nil, err
		}
		out = append(out, &g)
	}
	return out, rows.Err()
}

// GroupMembers lists a group's members.
func (d *DB) GroupMembers(groupID int64) ([]GroupMember, error) {
	rows, err := d.SQL.Query(`
		SELECT m.user_id, u.email, m.can_write, m.is_admin
		FROM group_members m JOIN users u ON u.id = m.user_id
		WHERE m.group_id=? ORDER BY u.email`, groupID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []GroupMember
	for rows.Next() {
		var m GroupMember
		if err := rows.Scan(&m.UserID, &m.Email, &m.CanWrite, &m.IsAdmin); err != nil {
			return nil, err
		}
		out = append(out, m)
	}
	return out, rows.Err()
}

// IsGroupAdmin reports whether a user administers a group (the owner, or a member
// granted group-admin).
func (d *DB) IsGroupAdmin(groupID, userID int64) bool {
	var ownerID int64
	if err := d.SQL.QueryRow(`SELECT owner_id FROM user_groups WHERE id=?`, groupID).Scan(&ownerID); err == nil && ownerID == userID {
		return true
	}
	var adm bool
	d.SQL.QueryRow(`SELECT is_admin FROM group_members WHERE group_id=? AND user_id=?`, groupID, userID).Scan(&adm)
	return adm
}

// SetGroupMemberAdmin grants or revokes group-admin for a member.
func (d *DB) SetGroupMemberAdmin(groupID, userID int64, isAdmin bool) error {
	_, err := d.SQL.Exec(`UPDATE group_members SET is_admin=? WHERE group_id=? AND user_id=?`,
		isAdmin, groupID, userID)
	return err
}

// SetGroupMemberWrite sets a member's read/write permission.
func (d *DB) SetGroupMemberWrite(groupID, userID int64, canWrite bool) error {
	_, err := d.SQL.Exec(`UPDATE group_members SET can_write=? WHERE group_id=? AND user_id=?`,
		canWrite, groupID, userID)
	return err
}

// AddGroupMember adds (or updates) a member by email. Returns ErrNotFound when
// no such user exists.
func (d *DB) AddGroupMember(groupID int64, email string, canWrite bool) error {
	u, err := d.GetUserByEmail(strings.ToLower(strings.TrimSpace(email)))
	if err != nil {
		return err
	}
	_, err = d.SQL.Exec(`
		INSERT INTO group_members (group_id, user_id, can_write) VALUES (?, ?, ?)
		ON CONFLICT(group_id, user_id) DO UPDATE SET can_write=excluded.can_write`,
		groupID, u.ID, canWrite)
	return err
}

// RemoveGroupMember removes a member.
func (d *DB) RemoveGroupMember(groupID, userID int64) error {
	_, err := d.SQL.Exec(`DELETE FROM group_members WHERE group_id=? AND user_id=?`, groupID, userID)
	return err
}

// LeaveGroup removes the user's own membership (the owner cannot leave; the
// caller enforces that).
func (d *DB) LeaveGroup(groupID, userID int64) error {
	return d.RemoveGroupMember(groupID, userID)
}

// ----- invitations (members join by accepting, not added directly) -----

// InviteToGroup records a pending invite (no-op if the user is already a member).
func (d *DB) InviteToGroup(groupID, userID int64, canWrite bool) error {
	if member, _ := d.IsGroupMember(groupID, userID); member {
		return nil
	}
	_, err := d.SQL.Exec(`
		INSERT INTO group_invites (group_id, user_id, can_write) VALUES (?, ?, ?)
		ON CONFLICT(group_id, user_id) DO UPDATE SET can_write=excluded.can_write`,
		groupID, userID, canWrite)
	return err
}

// GroupInvitesForUser lists groups a user has been invited to (pending).
func (d *DB) GroupInvitesForUser(userID int64) ([]*Group, error) {
	rows, err := d.SQL.Query(`
		SELECT g.id, g.name, u.email, gi.can_write
		FROM group_invites gi
		JOIN user_groups g ON g.id = gi.group_id
		JOIN users u ON u.id = g.owner_id
		WHERE gi.user_id=? ORDER BY g.name`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []*Group
	for rows.Next() {
		var g Group
		if err := rows.Scan(&g.ID, &g.Name, &g.OwnerEmail, &g.CanWrite); err != nil {
			return nil, err
		}
		out = append(out, &g)
	}
	return out, rows.Err()
}

// PendingInvitesForGroup lists a group's outstanding invites (owner view).
func (d *DB) PendingInvitesForGroup(groupID int64) ([]GroupMember, error) {
	rows, err := d.SQL.Query(`
		SELECT gi.user_id, u.email, gi.can_write
		FROM group_invites gi JOIN users u ON u.id = gi.user_id
		WHERE gi.group_id=? ORDER BY u.email`, groupID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []GroupMember
	for rows.Next() {
		var m GroupMember
		if err := rows.Scan(&m.UserID, &m.Email, &m.CanWrite); err != nil {
			return nil, err
		}
		out = append(out, m)
	}
	return out, rows.Err()
}

// AcceptInvite turns a pending invite into membership.
func (d *DB) AcceptInvite(groupID, userID int64) error {
	var cw bool
	err := d.SQL.QueryRow(`SELECT can_write FROM group_invites WHERE group_id=? AND user_id=?`,
		groupID, userID).Scan(&cw)
	if err != nil {
		if isNoRows(err) {
			return ErrNotFound
		}
		return err
	}
	if _, err := d.SQL.Exec(`
		INSERT INTO group_members (group_id, user_id, can_write) VALUES (?, ?, ?)
		ON CONFLICT(group_id, user_id) DO UPDATE SET can_write=excluded.can_write`,
		groupID, userID, cw); err != nil {
		return err
	}
	return d.DeleteInvite(groupID, userID)
}

// DeleteInvite removes a pending invite (reject by invitee or cancel by owner).
func (d *DB) DeleteInvite(groupID, userID int64) error {
	_, err := d.SQL.Exec(`DELETE FROM group_invites WHERE group_id=? AND user_id=?`, groupID, userID)
	return err
}

// IsGroupMember reports whether a user is a member of a group and their write
// permission.
func (d *DB) IsGroupMember(groupID, userID int64) (member, canWrite bool) {
	var cw bool
	if err := d.SQL.QueryRow(`SELECT can_write FROM group_members WHERE group_id=? AND user_id=?`,
		groupID, userID).Scan(&cw); err != nil {
		return false, false
	}
	return true, cw
}

// NotesSharedToGroup lists notes shared to a group (newest first).
func (d *DB) NotesSharedToGroup(groupID int64) ([]*Note, error) {
	rows, err := d.SQL.Query(`SELECT `+prefixCols("n", noteCols)+`, u.email
		FROM note_group_shares ngs
		JOIN notes n ON n.id = ngs.note_id
		JOIN users u ON u.id = n.owner_id
		WHERE ngs.group_id=? AND n.deleted_at IS NULL
		ORDER BY n.updated_at DESC`, groupID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []*Note
	for rows.Next() {
		var n Note
		if err := rows.Scan(&n.ID, &n.OwnerID, &n.Title, &n.BodyMd, &n.BodyText,
			&n.Summary, &n.SourceType, &n.SourceRef, &n.Status, &n.Error,
			&n.SummaryLang, &n.CategoryID, &n.CreatedAt, &n.UpdatedAt, &n.AIMillis, &n.Stopped, &n.OwnerEmail); err != nil {
			return nil, err
		}
		out = append(out, &n)
	}
	return out, rows.Err()
}

// GroupsUserCanShareTo lists groups a user may share notes to (owned or member).
func (d *DB) GroupsUserCanShareTo(userID int64) ([]*Group, error) {
	rows, err := d.SQL.Query(`
		SELECT id, name FROM user_groups WHERE owner_id=?
		UNION
		SELECT g.id, g.name FROM group_members m JOIN user_groups g ON g.id=m.group_id WHERE m.user_id=?
		ORDER BY name`, userID, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []*Group
	for rows.Next() {
		var g Group
		if err := rows.Scan(&g.ID, &g.Name); err != nil {
			return nil, err
		}
		out = append(out, &g)
	}
	return out, rows.Err()
}

// CanShareToGroup reports whether a user may share a note to a group (owner or
// member of it).
func (d *DB) CanShareToGroup(userID, groupID int64) bool {
	var n int
	d.SQL.QueryRow(`
		SELECT COUNT(*) FROM (
			SELECT id FROM user_groups WHERE id=? AND owner_id=?
			UNION
			SELECT group_id FROM group_members WHERE group_id=? AND user_id=?
		)`, groupID, userID, groupID, userID).Scan(&n)
	return n > 0
}

// ShareNoteToGroup shares a note with a group.
func (d *DB) ShareNoteToGroup(noteID, groupID int64) error {
	_, err := d.SQL.Exec(
		`INSERT OR IGNORE INTO note_group_shares (note_id, group_id) VALUES (?, ?)`, noteID, groupID)
	return err
}

// UnshareNoteFromGroup removes a group share.
func (d *DB) UnshareNoteFromGroup(noteID, groupID int64) error {
	_, err := d.SQL.Exec(`DELETE FROM note_group_shares WHERE note_id=? AND group_id=?`, noteID, groupID)
	return err
}

// GroupSharesForNote lists the groups a note is shared to.
func (d *DB) GroupSharesForNote(noteID int64) ([]*Group, error) {
	rows, err := d.SQL.Query(`
		SELECT g.id, g.name FROM note_group_shares ngs
		JOIN user_groups g ON g.id = ngs.group_id
		WHERE ngs.note_id=? ORDER BY g.name`, noteID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []*Group
	for rows.Next() {
		var g Group
		if err := rows.Scan(&g.ID, &g.Name); err != nil {
			return nil, err
		}
		out = append(out, &g)
	}
	return out, rows.Err()
}

// groupAccess returns whether a note is shared to any group the user is in, and
// the strongest write permission across those memberships.
func (d *DB) groupAccess(noteID, userID int64) (member bool, canWrite bool) {
	var w sql.NullInt64
	d.SQL.QueryRow(`
		SELECT MAX(m.can_write) FROM note_group_shares ngs
		JOIN group_members m ON m.group_id = ngs.group_id
		WHERE ngs.note_id=? AND m.user_id=?`, noteID, userID).Scan(&w)
	if !w.Valid {
		return false, false
	}
	return true, w.Int64 == 1
}
