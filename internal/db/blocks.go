package db

// AddBlock records that blocker has blocked blocked.
func (d *DB) AddBlock(blockerID, blockedID int64) error {
	_, err := d.SQL.Exec(
		`INSERT OR IGNORE INTO user_blocks (blocker_id, blocked_id) VALUES (?, ?)`, blockerID, blockedID)
	return err
}

// RemoveBlock removes a block.
func (d *DB) RemoveBlock(blockerID, blockedID int64) error {
	_, err := d.SQL.Exec(`DELETE FROM user_blocks WHERE blocker_id=? AND blocked_id=?`, blockerID, blockedID)
	return err
}

// ListBlocked returns the users a user has blocked.
func (d *DB) ListBlocked(blockerID int64) ([]*User, error) {
	rows, err := d.SQL.Query(`SELECT `+prefixCols("u", userCols)+`
		FROM user_blocks b JOIN users u ON u.id = b.blocked_id
		WHERE b.blocker_id=? ORDER BY u.email`, blockerID)
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

// BlockedEitherWay reports whether either of two users has blocked the other.
func (d *DB) BlockedEitherWay(a, b int64) bool {
	var n int
	d.SQL.QueryRow(`
		SELECT COUNT(*) FROM user_blocks
		WHERE (blocker_id=? AND blocked_id=?) OR (blocker_id=? AND blocked_id=?)`,
		a, b, b, a).Scan(&n)
	return n > 0
}
