package db

import "time"

// SystemLog is one recorded system event (mail, server error, email-in, etc.).
type SystemLog struct {
	ID        int64
	CreatedAt time.Time
	Level     string // info | warn | error
	Category  string
	Message   string
}

// maxSystemLogs bounds how many log rows are kept (oldest trimmed on insert).
const maxSystemLogs = 5000

// AddLog records a system event. Implements syslog.Sink.
func (d *DB) AddLog(level, category, message string) error {
	_, err := d.SQL.Exec(`INSERT INTO system_logs (level, category, message) VALUES (?, ?, ?)`,
		level, category, message)
	if err == nil {
		// Keep only the most recent maxSystemLogs rows.
		d.SQL.Exec(`DELETE FROM system_logs WHERE id <= (SELECT MAX(id) FROM system_logs) - ?`, maxSystemLogs)
	}
	return err
}

// ListLogs returns log rows (newest first), optionally filtered by level and/or
// category (empty = no filter).
func (d *DB) ListLogs(level, category string, limit, offset int) ([]SystemLog, error) {
	rows, err := d.SQL.Query(`
		SELECT id, created_at, level, category, message FROM system_logs
		WHERE (?='' OR level=?) AND (?='' OR category=?)
		ORDER BY id DESC LIMIT ? OFFSET ?`,
		level, level, category, category, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []SystemLog
	for rows.Next() {
		var l SystemLog
		if err := rows.Scan(&l.ID, &l.CreatedAt, &l.Level, &l.Category, &l.Message); err != nil {
			return nil, err
		}
		out = append(out, l)
	}
	return out, rows.Err()
}

// CountLogs counts log rows matching the (optional) level/category filter.
func (d *DB) CountLogs(level, category string) (int, error) {
	var n int
	err := d.SQL.QueryRow(`SELECT COUNT(*) FROM system_logs
		WHERE (?='' OR level=?) AND (?='' OR category=?)`,
		level, level, category, category).Scan(&n)
	return n, err
}

// LogCategories returns the distinct categories present (for the filter UI).
func (d *DB) LogCategories() ([]string, error) {
	rows, err := d.SQL.Query(`SELECT DISTINCT category FROM system_logs WHERE category!='' ORDER BY category`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var c string
		if err := rows.Scan(&c); err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

// ClearLogs deletes all system log rows.
func (d *DB) ClearLogs() error {
	_, err := d.SQL.Exec(`DELETE FROM system_logs`)
	return err
}
