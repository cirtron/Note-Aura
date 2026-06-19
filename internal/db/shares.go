package db

// ShareNote shares noteID with the user identified by email. Returns ErrNotFound
// if no such user exists. Re-sharing updates can_edit.
func (d *DB) ShareNote(noteID int64, email string, canEdit bool) error {
	target, err := d.GetUserByEmail(email)
	if err != nil {
		return err
	}
	_, err = d.SQL.Exec(`
		INSERT INTO note_shares (note_id, shared_with_user_id, can_edit)
		VALUES (?, ?, ?)
		ON CONFLICT(note_id, shared_with_user_id) DO UPDATE SET can_edit=excluded.can_edit`,
		noteID, target.ID, canEdit)
	return err
}

// UnshareNote removes a share.
func (d *DB) UnshareNote(noteID, sharedWithUserID int64) error {
	_, err := d.SQL.Exec(`DELETE FROM note_shares WHERE note_id=? AND shared_with_user_id=?`,
		noteID, sharedWithUserID)
	return err
}

// SharesForNote lists who a note is shared with.
func (d *DB) SharesForNote(noteID int64) ([]Share, error) {
	rows, err := d.SQL.Query(`
		SELECT s.note_id, s.shared_with_user_id, u.email, s.can_edit, s.created_at
		FROM note_shares s
		JOIN users u ON u.id = s.shared_with_user_id
		WHERE s.note_id=? ORDER BY u.email`, noteID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Share
	for rows.Next() {
		var s Share
		if err := rows.Scan(&s.NoteID, &s.SharedWithUserID, &s.SharedWithEmail, &s.CanEdit, &s.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, s)
	}
	return out, rows.Err()
}
