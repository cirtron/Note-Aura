package db

import "time"

// CreateUser inserts a user and returns its id.
func (d *DB) CreateUser(email, passwordHash string, isAdmin, emailVerified bool, verifyToken string) (int64, error) {
	res, err := d.SQL.Exec(
		`INSERT INTO users (email, password_hash, is_admin, email_verified, verify_token) VALUES (?, ?, ?, ?, ?)`,
		email, passwordHash, isAdmin, emailVerified, verifyToken)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

const userCols = `id, email, password_hash, is_admin, role_slug, capacity_override_mb, ollama_daily_override, email_verified, verify_token, invite_override, suspended, suspended_until, failed_logins, locked_until, created_at, last_seen_at, last_seen_ip, email_token, reset_token, reset_expires`

func scanUser(s interface{ Scan(...any) error }) (*User, error) {
	var u User
	if err := s.Scan(&u.ID, &u.Email, &u.PasswordHash, &u.IsAdmin,
		&u.RoleSlug, &u.CapacityOverrideMB, &u.OllamaDailyOverride,
		&u.EmailVerified, &u.VerifyToken, &u.InviteOverride, &u.Suspended, &u.SuspendedUntil,
		&u.FailedLogins, &u.LockedUntil,
		&u.CreatedAt, &u.LastSeenAt, &u.LastSeenIP, &u.EmailToken, &u.ResetToken, &u.ResetExpires); err != nil {
		return nil, err
	}
	return &u, nil
}

// SetResetToken stores a password-reset token and its expiry (unix seconds).
func (d *DB) SetResetToken(userID int64, token string, expiresUnix int64) error {
	_, err := d.SQL.Exec(`UPDATE users SET reset_token=?, reset_expires=? WHERE id=?`, token, expiresUnix, userID)
	return err
}

// GetUserByResetToken returns the user holding a non-empty, unexpired reset
// token, or ErrNotFound.
func (d *DB) GetUserByResetToken(token string) (*User, error) {
	if token == "" {
		return nil, ErrNotFound
	}
	row := d.SQL.QueryRow(`SELECT `+userCols+` FROM users WHERE reset_token=?`, token)
	u, err := scanUser(row)
	if err != nil {
		if isNoRows(err) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	if u.ResetExpires == 0 || time.Now().Unix() > u.ResetExpires {
		return nil, ErrNotFound // expired
	}
	return u, nil
}

// SetPassword updates a user's password hash and clears any reset token.
func (d *DB) SetPassword(userID int64, passwordHash string) error {
	_, err := d.SQL.Exec(`UPDATE users SET password_hash=?, reset_token='', reset_expires=0 WHERE id=?`,
		passwordHash, userID)
	return err
}

// SetUserEmailToken sets (or regenerates) a user's inbound-email token.
func (d *DB) SetUserEmailToken(userID int64, token string) error {
	_, err := d.SQL.Exec(`UPDATE users SET email_token=? WHERE id=?`, token, userID)
	return err
}

// GetUserByEmailToken returns the (non-suspended) user owning a non-empty inbound
// email token, or ErrNotFound.
func (d *DB) GetUserByEmailToken(token string) (*User, error) {
	if token == "" {
		return nil, ErrNotFound
	}
	row := d.SQL.QueryRow(`SELECT `+userCols+` FROM users WHERE email_token=?`, token)
	u, err := scanUser(row)
	if err != nil {
		if isNoRows(err) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return u, nil
}

// TouchUserSeen records that a user was just active (last visited time + IP).
func (d *DB) TouchUserSeen(userID int64, ip string) error {
	_, err := d.SQL.Exec(`UPDATE users SET last_seen_at=CURRENT_TIMESTAMP, last_seen_ip=? WHERE id=?`, ip, userID)
	return err
}

// SetUserSuspended suspends or reactivates a user and kicks their sessions.
// untilTime, when non-nil, sets an auto-expiry; nil means permanent.
func (d *DB) SetUserSuspended(userID int64, suspended bool, untilTime *time.Time) error {
	_, err := d.SQL.Exec(`UPDATE users SET suspended=?, suspended_until=? WHERE id=?`, suspended, untilTime, userID)
	if err == nil && suspended {
		d.SQL.Exec(`DELETE FROM sessions WHERE user_id=?`, userID)
	}
	return err
}

// ClearSuspendedUntil lifts a timed suspension that has expired.
func (d *DB) ClearSuspendedUntil(userID int64) error {
	_, err := d.SQL.Exec(`UPDATE users SET suspended=0, suspended_until=NULL WHERE id=?`, userID)
	return err
}

// RecordFailedLogin increments the failed-login counter and, if it reaches
// maxAttempts, sets locked_until = now + lockoutMinutes. Returns whether the
// account was just locked.
func (d *DB) RecordFailedLogin(userID int64, maxAttempts, lockoutMinutes int) (locked bool, err error) {
	_, err = d.SQL.Exec(`UPDATE users SET failed_logins=failed_logins+1 WHERE id=?`, userID)
	if err != nil {
		return false, err
	}
	var count int
	if err = d.SQL.QueryRow(`SELECT failed_logins FROM users WHERE id=?`, userID).Scan(&count); err != nil {
		return false, err
	}
	if maxAttempts > 0 && count >= maxAttempts {
		until := time.Now().Add(time.Duration(lockoutMinutes) * time.Minute)
		_, err = d.SQL.Exec(`UPDATE users SET locked_until=? WHERE id=?`, until, userID)
		return true, err
	}
	return false, nil
}

// ClearFailedLogins resets the counter and removes any lockout (on successful login).
func (d *DB) ClearFailedLogins(userID int64) error {
	_, err := d.SQL.Exec(`UPDATE users SET failed_logins=0, locked_until=NULL WHERE id=?`, userID)
	return err
}

// AdminClearLock clears the lockout on a user (admin action).
func (d *DB) AdminClearLock(userID int64) error {
	return d.ClearFailedLogins(userID)
}

// ForceLogout deletes all active sessions for a user.
func (d *DB) ForceLogout(userID int64) error {
	_, err := d.SQL.Exec(`DELETE FROM sessions WHERE user_id=?`, userID)
	return err
}

// DeleteUser removes a user and (via ON DELETE CASCADE) their notes, groups,
// shares, sessions, etc.
func (d *DB) DeleteUser(userID int64) error {
	_, err := d.SQL.Exec(`DELETE FROM users WHERE id=?`, userID)
	return err
}

// GetUserByVerifyToken returns the user holding the given (non-empty) email
// verification token, or ErrNotFound.
func (d *DB) GetUserByVerifyToken(token string) (*User, error) {
	if token == "" {
		return nil, ErrNotFound
	}
	row := d.SQL.QueryRow(`SELECT `+userCols+` FROM users WHERE verify_token=?`, token)
	u, err := scanUser(row)
	if err != nil {
		if isNoRows(err) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return u, nil
}

// SetEmailVerified marks a user verified and clears their token.
func (d *DB) SetEmailVerified(userID int64) error {
	_, err := d.SQL.Exec(`UPDATE users SET email_verified=1, verify_token='' WHERE id=?`, userID)
	return err
}

// SetVerifyToken stores a new verification token (for resend).
func (d *DB) SetVerifyToken(userID int64, token string) error {
	_, err := d.SQL.Exec(`UPDATE users SET verify_token=? WHERE id=?`, token, userID)
	return err
}

// SetUserInviteOverride sets (or clears, when n is nil) a user's invitation-limit
// override.
func (d *DB) SetUserInviteOverride(userID int64, n *int64) error {
	_, err := d.SQL.Exec(`UPDATE users SET invite_override=? WHERE id=?`, n, userID)
	return err
}

// ListUsersByRole returns all users with the given role slug, oldest first.
func (d *DB) ListUsersByRole(roleSlug string) ([]*User, error) {
	rows, err := d.SQL.Query(`SELECT `+userCols+` FROM users WHERE role_slug=? ORDER BY id`, roleSlug)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []*User
	for rows.Next() {
		u, err := scanUser(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, u)
	}
	return out, rows.Err()
}

// ListUsers returns all users (for admin management), oldest first.
func (d *DB) ListUsers() ([]*User, error) {
	rows, err := d.SQL.Query(`SELECT ` + userCols + ` FROM users ORDER BY id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []*User
	for rows.Next() {
		u, err := scanUser(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, u)
	}
	return out, rows.Err()
}

// SetUserRole assigns a role to a user.
func (d *DB) SetUserRole(userID int64, roleSlug string) error {
	_, err := d.SQL.Exec(`UPDATE users SET role_slug=? WHERE id=?`, roleSlug, userID)
	return err
}

// SetUserCapacityOverride sets (or clears, when mb is nil) a user's capacity
// override in MB.
func (d *DB) SetUserCapacityOverride(userID int64, mb *int64) error {
	_, err := d.SQL.Exec(`UPDATE users SET capacity_override_mb=? WHERE id=?`, mb, userID)
	return err
}

// SetUserOllamaOverride sets (or clears, when n is nil) a user's daily Ollama
// limit override.
func (d *DB) SetUserOllamaOverride(userID int64, n *int64) error {
	_, err := d.SQL.Exec(`UPDATE users SET ollama_daily_override=? WHERE id=?`, n, userID)
	return err
}

// CountAdmins reports how many admin users exist (for bootstrapping the first
// admin on registration).
func (d *DB) CountAdmins() (int, error) {
	var n int
	err := d.SQL.QueryRow(`SELECT COUNT(*) FROM users WHERE is_admin=1`).Scan(&n)
	return n, err
}

// SetUserAdmin promotes or demotes a user to/from platform admin.
func (d *DB) SetUserAdmin(userID int64, isAdmin bool) error {
	_, err := d.SQL.Exec(`UPDATE users SET is_admin=? WHERE id=?`, isAdmin, userID)
	return err
}

// GetUserByEmail returns one user or ErrNotFound.
func (d *DB) GetUserByEmail(email string) (*User, error) {
	row := d.SQL.QueryRow(`SELECT `+userCols+` FROM users WHERE email=?`, email)
	u, err := scanUser(row)
	if err != nil {
		if isNoRows(err) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return u, nil
}

// GetUser returns one user by id or ErrNotFound.
func (d *DB) GetUser(id int64) (*User, error) {
	row := d.SQL.QueryRow(`SELECT `+userCols+` FROM users WHERE id=?`, id)
	u, err := scanUser(row)
	if err != nil {
		if isNoRows(err) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return u, nil
}

// CountUsers reports the number of users (for first-run seeding).
func (d *DB) CountUsers() (int, error) {
	var n int
	err := d.SQL.QueryRow(`SELECT COUNT(*) FROM users`).Scan(&n)
	return n, err
}
