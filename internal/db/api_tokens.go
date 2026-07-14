package db

import (
	"database/sql"
	"time"

	"note-aura/internal/auth"
)

// APIToken represents a long-lived bearer token for the mobile / API clients.
type APIToken struct {
	ID         string
	UserID     int64
	Name       string
	CreatedAt  time.Time
	LastUsedAt sql.NullTime
	ExpiresAt  sql.NullTime
}

// CreateAPIToken generates a new 64-char hex token, stores it, and returns it.
func (d *DB) CreateAPIToken(userID int64, name string) (string, error) {
	token, err := auth.NewToken()
	if err != nil {
		return "", err
	}
	_, err = d.SQL.Exec(
		`INSERT INTO api_tokens (id, user_id, name) VALUES (?, ?, ?)`,
		token, userID, name,
	)
	if err != nil {
		return "", err
	}
	return token, nil
}

// GetAPITokenUser looks up the token, updates last_used_at, and returns the
// owner User. Returns ErrNotFound when the token doesn't exist or is expired.
func (d *DB) GetAPITokenUser(token string) (*User, error) {
	var userID int64
	var expiresAt sql.NullTime
	err := d.SQL.QueryRow(
		`SELECT user_id, expires_at FROM api_tokens WHERE id = ?`, token,
	).Scan(&userID, &expiresAt)
	if err != nil {
		if isNoRows(err) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	if expiresAt.Valid && time.Now().After(expiresAt.Time) {
		return nil, ErrNotFound
	}
	// Update last_used_at (best-effort, don't fail the request on error).
	d.SQL.Exec(`UPDATE api_tokens SET last_used_at = CURRENT_TIMESTAMP WHERE id = ?`, token)

	return d.GetUser(userID)
}

// DeleteAPIToken removes a token (logout).
func (d *DB) DeleteAPIToken(token string) error {
	_, err := d.SQL.Exec(`DELETE FROM api_tokens WHERE id = ?`, token)
	return err
}

// ListAPITokens returns all tokens for a user (for a future "sessions" UI).
func (d *DB) ListAPITokens(userID int64) ([]APIToken, error) {
	rows, err := d.SQL.Query(
		`SELECT id, user_id, name, created_at, last_used_at, expires_at
		   FROM api_tokens WHERE user_id = ? ORDER BY created_at DESC`, userID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []APIToken
	for rows.Next() {
		var t APIToken
		if err := rows.Scan(&t.ID, &t.UserID, &t.Name, &t.CreatedAt, &t.LastUsedAt, &t.ExpiresAt); err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	return out, rows.Err()
}
