package db

import "time"

// CreateSession stores a session row.
func (d *DB) CreateSession(id string, userID int64, expiresAt time.Time) error {
	_, err := d.SQL.Exec(
		`INSERT INTO sessions (id, user_id, expires_at) VALUES (?, ?, ?)`,
		id, userID, expiresAt)
	return err
}

// GetSession returns a non-expired session or ErrNotFound.
func (d *DB) GetSession(id string) (*Session, error) {
	var s Session
	err := d.SQL.QueryRow(
		`SELECT id, user_id, expires_at, created_at FROM sessions WHERE id=?`, id,
	).Scan(&s.ID, &s.UserID, &s.ExpiresAt, &s.CreatedAt)
	if err != nil {
		if isNoRows(err) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	if time.Now().After(s.ExpiresAt) {
		_ = d.DeleteSession(id)
		return nil, ErrNotFound
	}
	return &s, nil
}

// DeleteSession removes a session row (logout / expiry).
func (d *DB) DeleteSession(id string) error {
	_, err := d.SQL.Exec(`DELETE FROM sessions WHERE id=?`, id)
	return err
}
