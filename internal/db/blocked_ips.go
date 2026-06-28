package db

import "time"

// BlockIP adds an IP to the block list (idempotent — updates reason if already present).
func (d *DB) BlockIP(ip, reason string) error {
	_, err := d.SQL.Exec(`
		INSERT INTO blocked_ips (ip, reason, blocked_at) VALUES (?, ?, CURRENT_TIMESTAMP)
		ON CONFLICT(ip) DO UPDATE SET reason=excluded.reason, blocked_at=excluded.blocked_at`,
		ip, reason)
	return err
}

// UnblockIP removes an IP from the block list.
func (d *DB) UnblockIP(id int64) error {
	_, err := d.SQL.Exec(`DELETE FROM blocked_ips WHERE id=?`, id)
	return err
}

// IsIPBlocked returns true if the given IP is in the block list.
func (d *DB) IsIPBlocked(ip string) bool {
	var n int
	d.SQL.QueryRow(`SELECT COUNT(*) FROM blocked_ips WHERE ip=?`, ip).Scan(&n)
	return n > 0
}

// ListBlockedIPs returns all blocked IPs, newest first.
func (d *DB) ListBlockedIPs() ([]BlockedIP, error) {
	rows, err := d.SQL.Query(`SELECT id, ip, reason, blocked_at FROM blocked_ips ORDER BY blocked_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []BlockedIP
	for rows.Next() {
		var b BlockedIP
		if err := rows.Scan(&b.ID, &b.IP, &b.Reason, &b.BlockedAt); err != nil {
			return nil, err
		}
		out = append(out, b)
	}
	return out, rows.Err()
}

// AutoExpireSuspensions clears timed suspensions whose deadline has passed.
// Called once per request in middleware; the UPDATE is cheap when no rows match.
func (d *DB) AutoExpireSuspensions() {
	d.SQL.Exec(`UPDATE users SET suspended=0, suspended_until=NULL
		WHERE suspended=1 AND suspended_until IS NOT NULL AND suspended_until <= ?`, time.Now())
}
