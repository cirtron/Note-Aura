package db

import (
	"database/sql"
	"time"
)

// Invitation is a pending/accepted invite for a new user to join the platform.
type Invitation struct {
	ID         int64
	InviterID  int64
	Email      string
	Token      string
	Lang       string
	Accepted   bool
	AcceptedAt sql.NullTime
	CreatedAt  time.Time
}

// CreateInvitation records a new-user invitation in the inviter-chosen language.
func (d *DB) CreateInvitation(inviterID int64, email, token, lang string) error {
	_, err := d.SQL.Exec(
		`INSERT INTO user_invitations (inviter_id, email, token, lang) VALUES (?, ?, ?, ?)`,
		inviterID, email, token, lang)
	return err
}

// CountInvitationsBy returns how many invitations a user has sent (toward limit).
func (d *DB) CountInvitationsBy(inviterID int64) (int, error) {
	var n int
	err := d.SQL.QueryRow(`SELECT COUNT(*) FROM user_invitations WHERE inviter_id=?`, inviterID).Scan(&n)
	return n, err
}

// GetInvitationByToken returns a pending (not yet accepted) invitation.
func (d *DB) GetInvitationByToken(token string) (*Invitation, error) {
	if token == "" {
		return nil, ErrNotFound
	}
	var inv Invitation
	err := d.SQL.QueryRow(
		`SELECT id, inviter_id, email, token, lang, accepted_at, created_at FROM user_invitations WHERE token=?`, token).
		Scan(&inv.ID, &inv.InviterID, &inv.Email, &inv.Token, &inv.Lang, &inv.AcceptedAt, &inv.CreatedAt)
	if err != nil {
		if isNoRows(err) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	inv.Accepted = inv.AcceptedAt.Valid
	return &inv, nil
}

// MarkInvitationAccepted stamps an invitation as accepted.
func (d *DB) MarkInvitationAccepted(token string) error {
	_, err := d.SQL.Exec(`UPDATE user_invitations SET accepted_at=CURRENT_TIMESTAMP WHERE token=?`, token)
	return err
}

// InvitationWithInviter is an invitation plus its inviter's email (admin view).
type InvitationWithInviter struct {
	Invitation
	InviterEmail string
}

// ListAllInvitations lists every invitation with the inviter's email (admin),
// newest first.
func (d *DB) ListAllInvitations() ([]InvitationWithInviter, error) {
	rows, err := d.SQL.Query(
		`SELECT i.id, i.inviter_id, i.email, i.token, i.lang, i.accepted_at, i.created_at, u.email
		 FROM user_invitations i JOIN users u ON u.id = i.inviter_id
		 ORDER BY i.created_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []InvitationWithInviter
	for rows.Next() {
		var v InvitationWithInviter
		if err := rows.Scan(&v.ID, &v.InviterID, &v.Email, &v.Token, &v.Lang, &v.AcceptedAt, &v.CreatedAt, &v.InviterEmail); err != nil {
			return nil, err
		}
		v.Accepted = v.AcceptedAt.Valid
		out = append(out, v)
	}
	return out, rows.Err()
}

// GetInvitation returns one invitation by id, or ErrNotFound.
func (d *DB) GetInvitation(id int64) (*Invitation, error) {
	var inv Invitation
	err := d.SQL.QueryRow(
		`SELECT id, inviter_id, email, token, lang, accepted_at, created_at FROM user_invitations WHERE id=?`, id).
		Scan(&inv.ID, &inv.InviterID, &inv.Email, &inv.Token, &inv.Lang, &inv.AcceptedAt, &inv.CreatedAt)
	if err != nil {
		if isNoRows(err) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	inv.Accepted = inv.AcceptedAt.Valid
	return &inv, nil
}

// DeleteInvitation removes an invitation by id.
func (d *DB) DeleteInvitation(id int64) error {
	_, err := d.SQL.Exec(`DELETE FROM user_invitations WHERE id=?`, id)
	return err
}

// ListInvitationsBy lists a user's sent invitations (newest first).
func (d *DB) ListInvitationsBy(inviterID int64) ([]Invitation, error) {
	rows, err := d.SQL.Query(
		`SELECT id, inviter_id, email, token, lang, accepted_at, created_at
		 FROM user_invitations WHERE inviter_id=? ORDER BY created_at DESC`, inviterID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Invitation
	for rows.Next() {
		var inv Invitation
		if err := rows.Scan(&inv.ID, &inv.InviterID, &inv.Email, &inv.Token, &inv.Lang, &inv.AcceptedAt, &inv.CreatedAt); err != nil {
			return nil, err
		}
		inv.Accepted = inv.AcceptedAt.Valid
		out = append(out, inv)
	}
	return out, rows.Err()
}
