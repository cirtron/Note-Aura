package db

// CountNotes returns the number of live (non-deleted) notes across all users.
func (d *DB) CountNotes() (int, error) {
	var n int
	err := d.SQL.QueryRow(`SELECT COUNT(*) FROM notes WHERE deleted_at IS NULL`).Scan(&n)
	return n, err
}

// CountNotesByOwner returns a user's live note count.
func (d *DB) CountNotesByOwner(ownerID int64) (int, error) {
	var n int
	err := d.SQL.QueryRow(`SELECT COUNT(*) FROM notes WHERE owner_id=? AND deleted_at IS NULL`, ownerID).Scan(&n)
	return n, err
}

// CountGroups returns the total number of groups.
func (d *DB) CountGroups() (int, error) {
	var n int
	err := d.SQL.QueryRow(`SELECT COUNT(*) FROM user_groups`).Scan(&n)
	return n, err
}

// JobStats counts processing-queue jobs by state.
type JobStats struct {
	Queued  int
	Running int
	Failed  int
}

// JobCounts returns current job-queue stats.
func (d *DB) JobCounts() JobStats {
	var s JobStats
	d.SQL.QueryRow(`SELECT
		COALESCE(SUM(status='queued'),0),
		COALESCE(SUM(status='running'),0),
		COALESCE(SUM(status='failed'),0) FROM jobs`).Scan(&s.Queued, &s.Running, &s.Failed)
	return s
}

// RecentNotesWithOwner lists the most recently updated notes across all users
// (admin overview). Only metadata is loaded.
func (d *DB) RecentNotesWithOwner(limit int) ([]*Note, error) {
	return d.RecentNotesWithOwnerPage(limit, 0)
}

// RecentNotesWithOwnerPage is RecentNotesWithOwner with an offset (pagination).
func (d *DB) RecentNotesWithOwnerPage(limit, offset int) ([]*Note, error) {
	rows, err := d.SQL.Query(`
		SELECT n.id, n.title, n.status, n.source_type, n.updated_at, u.email
		FROM notes n JOIN users u ON u.id = n.owner_id
		WHERE n.deleted_at IS NULL
		ORDER BY n.updated_at DESC LIMIT ? OFFSET ?`, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []*Note
	for rows.Next() {
		var n Note
		if err := rows.Scan(&n.ID, &n.Title, &n.Status, &n.SourceType, &n.UpdatedAt, &n.OwnerEmail); err != nil {
			return nil, err
		}
		out = append(out, &n)
	}
	return out, rows.Err()
}
