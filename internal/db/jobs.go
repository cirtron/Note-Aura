package db

// EnqueueJob adds a processing job for a note. params is a comma-separated list
// of which AI fields to (re)generate (e.g. "title,summary,tags"); empty means
// the worker's default (all).
func (d *DB) EnqueueJob(noteID int64, kind, params string) error {
	_, err := d.SQL.Exec(
		`INSERT INTO jobs (note_id, kind, status, params) VALUES (?, ?, 'queued', ?)`, noteID, kind, params)
	return err
}

// ClaimJob atomically picks the oldest queued job and marks it running.
// Returns ErrNotFound when the queue is empty.
func (d *DB) ClaimJob() (*Job, error) {
	var j Job
	err := d.SQL.QueryRow(`
		SELECT id, note_id, kind, status, attempts, last_error, params
		FROM jobs WHERE status='queued' ORDER BY id LIMIT 1`).
		Scan(&j.ID, &j.NoteID, &j.Kind, &j.Status, &j.Attempts, &j.LastError, &j.Params)
	if err != nil {
		if isNoRows(err) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	res, err := d.SQL.Exec(
		`UPDATE jobs SET status='running', attempts=attempts+1, updated_at=CURRENT_TIMESTAMP
		 WHERE id=? AND status='queued'`, j.ID)
	if err != nil {
		return nil, err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		// Lost the race; let the caller poll again.
		return nil, ErrNotFound
	}
	j.Attempts++
	return &j, nil
}

// CompleteJob marks a job done.
func (d *DB) CompleteJob(id int64) error {
	_, err := d.SQL.Exec(`UPDATE jobs SET status='done', updated_at=CURRENT_TIMESTAMP WHERE id=?`, id)
	return err
}

// FailJob records an error and either requeues (attempts < max) or marks failed.
func (d *DB) FailJob(id int64, attempts, maxAttempts int, errMsg string) error {
	status := "queued"
	if attempts >= maxAttempts {
		status = "failed"
	}
	_, err := d.SQL.Exec(
		`UPDATE jobs SET status=?, last_error=?, updated_at=CURRENT_TIMESTAMP WHERE id=?`,
		status, errMsg, id)
	return err
}

// RequeueStuckJobs resets jobs left 'running' from a previous crash to 'queued'.
func (d *DB) RequeueStuckJobs() error {
	_, err := d.SQL.Exec(`UPDATE jobs SET status='queued' WHERE status='running'`)
	return err
}

// DeleteJobsForNote removes all queued/running jobs for a note so a stopped note
// cannot be requeued by the worker's FailJob path.
func (d *DB) DeleteJobsForNote(noteID int64) error {
	_, err := d.SQL.Exec(`DELETE FROM jobs WHERE note_id=?`, noteID)
	return err
}
