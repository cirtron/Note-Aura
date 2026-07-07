package db

import "strings"

// ---- banned usernames ----

// AddBannedUsername adds a username (email local-part) to the ban list.
// Idempotent — updates the note if the username already exists.
func (d *DB) AddBannedUsername(username, note string) error {
	_, err := d.SQL.Exec(`
		INSERT INTO banned_usernames (username, note) VALUES (?, ?)
		ON CONFLICT(username) DO UPDATE SET note=excluded.note`,
		strings.ToLower(username), note)
	return err
}

// RemoveBannedUsername removes a username ban by ID.
func (d *DB) RemoveBannedUsername(id int64) error {
	_, err := d.SQL.Exec(`DELETE FROM banned_usernames WHERE id=?`, id)
	return err
}

// ListBannedUsernames returns all banned usernames, newest first.
func (d *DB) ListBannedUsernames() ([]BannedUsername, error) {
	rows, err := d.SQL.Query(
		`SELECT id, username, note, created_at FROM banned_usernames ORDER BY created_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []BannedUsername
	for rows.Next() {
		var b BannedUsername
		if err := rows.Scan(&b.ID, &b.Username, &b.Note, &b.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, b)
	}
	return out, rows.Err()
}

// IsUsernameBanned returns true if the given local-part is on the ban list.
func (d *DB) IsUsernameBanned(localPart string) (bool, error) {
	var n int
	err := d.SQL.QueryRow(
		`SELECT COUNT(*) FROM banned_usernames WHERE username=?`,
		strings.ToLower(localPart)).Scan(&n)
	return n > 0, err
}

// ---- banned email patterns ----

// AddBannedEmailPattern adds a full email address or domain to the ban list.
// Idempotent — updates the note if the pattern already exists.
func (d *DB) AddBannedEmailPattern(pattern, note string) error {
	_, err := d.SQL.Exec(`
		INSERT INTO banned_email_patterns (pattern, note) VALUES (?, ?)
		ON CONFLICT(pattern) DO UPDATE SET note=excluded.note`,
		strings.ToLower(pattern), note)
	return err
}

// RemoveBannedEmailPattern removes a pattern ban by ID.
func (d *DB) RemoveBannedEmailPattern(id int64) error {
	_, err := d.SQL.Exec(`DELETE FROM banned_email_patterns WHERE id=?`, id)
	return err
}

// ListBannedEmailPatterns returns all banned email patterns, newest first.
func (d *DB) ListBannedEmailPatterns() ([]BannedEmailPattern, error) {
	rows, err := d.SQL.Query(
		`SELECT id, pattern, note, created_at FROM banned_email_patterns ORDER BY created_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []BannedEmailPattern
	for rows.Next() {
		var b BannedEmailPattern
		if err := rows.Scan(&b.ID, &b.Pattern, &b.Note, &b.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, b)
	}
	return out, rows.Err()
}

// IsEmailBanned returns true if the given email address matches any ban pattern.
// A pattern containing "@" is matched as an exact address; otherwise it is
// matched as a domain (the email's host part must equal the pattern).
func (d *DB) IsEmailBanned(email string) (bool, error) {
	email = strings.ToLower(email)
	parts := strings.SplitN(email, "@", 2)
	if len(parts) != 2 {
		return false, nil
	}
	domain := parts[1]

	rows, err := d.SQL.Query(`SELECT pattern FROM banned_email_patterns`)
	if err != nil {
		return false, err
	}
	defer rows.Close()
	for rows.Next() {
		var pattern string
		if err := rows.Scan(&pattern); err != nil {
			return false, err
		}
		if strings.Contains(pattern, "@") {
			if pattern == email {
				return true, nil
			}
		} else {
			if pattern == domain {
				return true, nil
			}
		}
	}
	return false, rows.Err()
}
