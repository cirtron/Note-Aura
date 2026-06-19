package db

// fillSchedule loads a note's calendar fields. Call after the source result set
// is closed (single-connection pool).
func (d *DB) fillSchedule(n *Note) {
	d.SQL.QueryRow(
		`SELECT event_date, start_time, end_time, all_day, reminder_minutes FROM notes WHERE id=?`, n.ID,
	).Scan(&n.EventDate, &n.StartTime, &n.EndTime, &n.AllDay, &n.ReminderMinutes)
}

// SetNoteSchedule updates a note's calendar fields. Passing an empty eventDate
// clears the schedule. reminderMinutes is nil for no reminder. Any change resets
// reminder_sent_at so the (possibly new) reminder can fire again.
func (d *DB) SetNoteSchedule(noteID int64, eventDate, startTime, endTime string, allDay bool, reminderMinutes *int) error {
	if eventDate == "" {
		startTime, endTime, allDay, reminderMinutes = "", "", false, nil
	}
	_, err := d.SQL.Exec(`
		UPDATE notes SET event_date=?, start_time=?, end_time=?, all_day=?,
		    reminder_minutes=?, reminder_sent_at=NULL, updated_at=CURRENT_TIMESTAMP
		WHERE id=?`,
		eventDate, startTime, endTime, allDay, reminderMinutes, noteID)
	return err
}

// NotesByDateRange returns the owner's scheduled notes with event_date within
// [from, to] (inclusive, YYYY-MM-DD), ordered by date then time (timed before
// all-day).
func (d *DB) NotesByDateRange(ownerID int64, from, to string) ([]*Note, error) {
	rows, err := d.SQL.Query(`
		SELECT id, title, event_date, start_time, end_time, all_day, status
		FROM notes
		WHERE owner_id=? AND deleted_at IS NULL AND event_date!='' AND event_date>=? AND event_date<=?
		ORDER BY event_date, (start_time=''), start_time`, ownerID, from, to)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []*Note
	for rows.Next() {
		var n Note
		if err := rows.Scan(&n.ID, &n.Title, &n.EventDate, &n.StartTime, &n.EndTime, &n.AllDay, &n.Status); err != nil {
			return nil, err
		}
		out = append(out, &n)
	}
	return out, rows.Err()
}

// DueReminder is a pending reminder candidate for the scheduler to evaluate.
type DueReminder struct {
	NoteID     int64
	OwnerEmail string
	Title      string
	EventDate  string
	StartTime  string
	AllDay     bool
	Minutes    int64
}

// PendingReminders returns scheduled notes that have a reminder set and not yet
// sent. The scheduler decides which are actually due.
func (d *DB) PendingReminders() ([]DueReminder, error) {
	rows, err := d.SQL.Query(`
		SELECT n.id, u.email, n.title, n.event_date, n.start_time, n.all_day, n.reminder_minutes
		FROM notes n JOIN users u ON u.id = n.owner_id
		WHERE n.deleted_at IS NULL AND n.event_date!=''
		  AND n.reminder_minutes IS NOT NULL AND n.reminder_sent_at IS NULL`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []DueReminder
	for rows.Next() {
		var r DueReminder
		if err := rows.Scan(&r.NoteID, &r.OwnerEmail, &r.Title, &r.EventDate, &r.StartTime, &r.AllDay, &r.Minutes); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// MarkReminderSent records that a note's reminder has been delivered (or skipped).
func (d *DB) MarkReminderSent(noteID int64) error {
	_, err := d.SQL.Exec(`UPDATE notes SET reminder_sent_at=CURRENT_TIMESTAMP WHERE id=?`, noteID)
	return err
}
