package db

import (
	"database/sql"
	"strings"
)

const noteCols = `id, owner_id, title, body_md, body_text, summary, source_type, source_ref, status, error, summary_lang, category_id, created_at, updated_at, ai_ms, stopped`

func scanNote(s interface{ Scan(...any) error }) (*Note, error) {
	var n Note
	if err := s.Scan(&n.ID, &n.OwnerID, &n.Title, &n.BodyMd, &n.BodyText,
		&n.Summary, &n.SourceType, &n.SourceRef, &n.Status, &n.Error,
		&n.SummaryLang, &n.CategoryID, &n.CreatedAt, &n.UpdatedAt, &n.AIMillis, &n.Stopped); err != nil {
		return nil, err
	}
	return &n, nil
}

// SetNoteAITime records how long (ms) the most recent AI processing run took.
func (d *DB) SetNoteAITime(id, ms int64) error {
	_, err := d.SQL.Exec(`UPDATE notes SET ai_ms=? WHERE id=?`, ms, id)
	return err
}

// fillCategory resolves a note's category name/color from its category_id. Call
// only after the source result set is closed (single-connection pool).
func (d *DB) fillCategory(n *Note) {
	if !n.CategoryID.Valid {
		return
	}
	d.SQL.QueryRow(`SELECT name, color FROM categories WHERE id=?`, n.CategoryID.Int64).
		Scan(&n.CategoryName, &n.CategoryColor)
}

// CreateNote inserts a note and returns its id. Pass status "ready" for a
// finished manual note, or "processing" when an AI job will fill it in.
func (d *DB) CreateNote(n *Note) (int64, error) {
	res, err := d.SQL.Exec(`
		INSERT INTO notes (owner_id, title, body_md, body_text, summary, source_type, source_ref, status, summary_lang)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		n.OwnerID, n.Title, n.BodyMd, n.BodyText, n.Summary, n.SourceType, n.SourceRef, n.Status, n.SummaryLang)
	if err != nil {
		return 0, err
	}
	id, err := res.LastInsertId()
	if err != nil {
		return 0, err
	}
	d.syncFTS(id, n.Title, n.BodyText)
	return id, nil
}

// GetNote returns a non-deleted note by id, or ErrNotFound.
func (d *DB) GetNote(id int64) (*Note, error) {
	row := d.SQL.QueryRow(`SELECT `+noteCols+` FROM notes WHERE id=? AND deleted_at IS NULL`, id)
	n, err := scanNote(row)
	if err != nil {
		if isNoRows(err) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	n.Tags, _ = d.TagsForNote(id)
	d.fillCategory(n)
	d.fillSchedule(n)
	return n, nil
}

// BackfillMarkdown is a one-time migration: notes created by the old Quill/HTML
// editor have body_html but no body_md. convert turns the stored HTML into plain
// text/Markdown. Returns the number of notes converted.
func (d *DB) BackfillMarkdown(convert func(html string) string) (int, error) {
	rows, err := d.SQL.Query(`SELECT id, body_html FROM notes
		WHERE (body_md IS NULL OR body_md='') AND body_html IS NOT NULL AND body_html!=''`)
	if err != nil {
		return 0, err
	}
	type row struct {
		id   int64
		html string
	}
	var todo []row
	for rows.Next() {
		var r row
		if err := rows.Scan(&r.id, &r.html); err != nil {
			rows.Close()
			return 0, err
		}
		todo = append(todo, r)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return 0, err
	}
	n := 0
	for _, r := range todo {
		md := convert(r.html)
		if _, err := d.SQL.Exec(`UPDATE notes SET body_md=?, body_text=? WHERE id=?`, md, md, r.id); err != nil {
			return n, err
		}
		var title string
		d.SQL.QueryRow(`SELECT title FROM notes WHERE id=?`, r.id).Scan(&title)
		d.syncFTS(r.id, title, md)
		n++
	}
	return n, nil
}

// SetNoteSummaryLang updates a note's AI summary/title language code.
func (d *DB) SetNoteSummaryLang(noteID int64, lang string) error {
	_, err := d.SQL.Exec(`UPDATE notes SET summary_lang=? WHERE id=?`, lang, noteID)
	return err
}

// SetNoteCategory assigns (or clears, when categoryID is nil) a note's category.
func (d *DB) SetNoteCategory(noteID int64, categoryID *int64) error {
	_, err := d.SQL.Exec(`UPDATE notes SET category_id=?, updated_at=CURRENT_TIMESTAMP WHERE id=?`,
		categoryID, noteID)
	return err
}

// UpdateNoteContent updates the user-editable fields and re-syncs FTS.
func (d *DB) UpdateNoteContent(id int64, title, bodyMd, bodyText string) error {
	_, err := d.SQL.Exec(`
		UPDATE notes SET title=?, body_md=?, body_text=?, updated_at=CURRENT_TIMESTAMP
		WHERE id=?`, title, bodyMd, bodyText, id)
	if err != nil {
		return err
	}
	d.syncFTS(id, title, bodyText)
	return nil
}

// ApplyAIResult writes the AI-generated title/summary/body and marks the note
// ready. Used by the worker after processing.
func (d *DB) ApplyAIResult(id int64, title, summary, bodyMd, bodyText string) error {
	_, err := d.SQL.Exec(`
		UPDATE notes SET title=?, summary=?, body_md=?, body_text=?, status='ready', error='',
		    updated_at=CURRENT_TIMESTAMP
		WHERE id=?`, title, summary, bodyMd, bodyText, id)
	if err != nil {
		return err
	}
	d.syncFTS(id, title, bodyText)
	return nil
}

// SetNoteStatus updates status and error message.
func (d *DB) SetNoteStatus(id int64, status, errMsg string) error {
	_, err := d.SQL.Exec(`UPDATE notes SET status=?, error=?, updated_at=CURRENT_TIMESTAMP WHERE id=?`,
		status, errMsg, id)
	return err
}

// StopNote marks a note's AI processing as stopped by the owner. It reuses the
// 'failed' status (no CHECK change) plus the stopped flag, which the UI renders
// as a neutral "Stopped" with Retry.
func (d *DB) StopNote(id int64) error {
	_, err := d.SQL.Exec(
		`UPDATE notes SET status='failed', stopped=1, error='stopped by user', updated_at=CURRENT_TIMESTAMP WHERE id=?`, id)
	return err
}

// ClearStopped resets the stopped flag (called when a stopped note is retried).
func (d *DB) ClearStopped(id int64) error {
	_, err := d.SQL.Exec(`UPDATE notes SET stopped=0 WHERE id=?`, id)
	return err
}

// SoftDeleteNote marks a note deleted and removes it from the search index.
func (d *DB) SoftDeleteNote(id int64) error {
	_, err := d.SQL.Exec(`UPDATE notes SET deleted_at=CURRENT_TIMESTAMP WHERE id=?`, id)
	if err != nil {
		return err
	}
	d.SQL.Exec(`DELETE FROM notes_fts WHERE rowid=?`, id)
	return nil
}

// ListNotes returns the owner's notes, newest first.
func (d *DB) ListNotes(ownerID int64) ([]*Note, error) {
	rows, err := d.SQL.Query(`SELECT `+noteCols+`
		FROM notes WHERE owner_id=? AND deleted_at IS NULL
		ORDER BY updated_at DESC`, ownerID)
	if err != nil {
		return nil, err
	}
	return d.collectNotes(rows)
}

// SearchNotes runs an FTS5 query scoped to the owner's notes.
func (d *DB) SearchNotes(ownerID int64, query string) ([]*Note, error) {
	match := ftsQuery(query)
	if match == "" {
		return d.ListNotes(ownerID)
	}
	rows, err := d.SQL.Query(`SELECT `+prefixCols("n", noteCols)+`
		FROM notes_fts f
		JOIN notes n ON n.id = f.rowid
		WHERE f.notes_fts MATCH ? AND n.owner_id=? AND n.deleted_at IS NULL
		ORDER BY rank`, match, ownerID)
	if err != nil {
		return nil, err
	}
	return d.collectNotes(rows)
}

// ListSharedWithUser returns notes shared with the user, either directly or via
// a group they belong to.
func (d *DB) ListSharedWithUser(userID int64) ([]*Note, error) {
	rows, err := d.SQL.Query(`SELECT `+prefixCols("n", noteCols)+`, u.email
		FROM notes n
		JOIN users u ON u.id = n.owner_id
		WHERE n.deleted_at IS NULL AND (
		    n.id IN (SELECT note_id FROM note_shares WHERE shared_with_user_id=?)
		 OR n.id IN (SELECT ngs.note_id FROM note_group_shares ngs
		             JOIN group_members m ON m.group_id = ngs.group_id WHERE m.user_id=?))
		ORDER BY n.updated_at DESC`, userID, userID)
	if err != nil {
		return nil, err
	}
	var out []*Note
	for rows.Next() {
		var n Note
		if err := rows.Scan(&n.ID, &n.OwnerID, &n.Title, &n.BodyMd, &n.BodyText,
			&n.Summary, &n.SourceType, &n.SourceRef, &n.Status, &n.Error,
			&n.SummaryLang, &n.CategoryID, &n.CreatedAt, &n.UpdatedAt, &n.AIMillis, &n.Stopped, &n.OwnerEmail); err != nil {
			rows.Close()
			return nil, err
		}
		out = append(out, &n)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, err
	}
	for _, n := range out {
		n.Tags, _ = d.TagsForNote(n.ID)
		d.fillCategory(n)
		_, n.CanEdit, _ = d.NoteAccess(n.ID, userID)
	}
	return out, nil
}

func (d *DB) collectNotes(rows *sql.Rows) ([]*Note, error) {
	var out []*Note
	for rows.Next() {
		n, err := scanNote(rows)
		if err != nil {
			rows.Close()
			return nil, err
		}
		out = append(out, n)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, err
	}
	// Fetch tags/category AFTER the result set is closed: with a single DB
	// connection a nested query while rows are open would deadlock.
	for _, n := range out {
		n.Tags, _ = d.TagsForNote(n.ID)
		d.fillCategory(n)
	}
	return out, nil
}

// ----- access control -----

// NoteAccess reports whether userID may read the note and whether they may edit
// it. Owner has full rights; sharee rights come from note_shares.can_edit.
func (d *DB) NoteAccess(noteID, userID int64) (canRead, canEdit bool, err error) {
	var ownerID int64
	err = d.SQL.QueryRow(`SELECT owner_id FROM notes WHERE id=? AND deleted_at IS NULL`, noteID).Scan(&ownerID)
	if err != nil {
		if isNoRows(err) {
			return false, false, ErrNotFound
		}
		return false, false, err
	}
	if ownerID == userID {
		return true, true, nil
	}
	// Direct (per-user) share.
	var ce bool
	err = d.SQL.QueryRow(`SELECT can_edit FROM note_shares WHERE note_id=? AND shared_with_user_id=?`,
		noteID, userID).Scan(&ce)
	if err == nil {
		canRead, canEdit = true, ce
	} else if !isNoRows(err) {
		return false, false, err
	}
	// Group share.
	if member, gw := d.groupAccess(noteID, userID); member {
		canRead = true
		canEdit = canEdit || gw
	}
	return canRead, canEdit, nil
}

// ----- FTS sync -----

func (d *DB) syncFTS(noteID int64, title, bodyText string) {
	d.SQL.Exec(`DELETE FROM notes_fts WHERE rowid=?`, noteID)
	d.SQL.Exec(`INSERT INTO notes_fts(rowid, title, body_text) VALUES (?, ?, ?)`,
		noteID, title, bodyText)
}

// ftsQuery turns a free-text search into a safe FTS5 MATCH expression: each
// whitespace-separated term becomes a prefix match, AND-ed together.
func ftsQuery(q string) string {
	fields := strings.Fields(q)
	if len(fields) == 0 {
		return ""
	}
	var terms []string
	for _, f := range fields {
		f = strings.Map(func(r rune) rune {
			if r == '"' || r == '*' || r == '(' || r == ')' || r == ':' {
				return -1
			}
			return r
		}, f)
		if f == "" {
			continue
		}
		terms = append(terms, `"`+f+`"*`)
	}
	return strings.Join(terms, " ")
}

func prefixCols(prefix, cols string) string {
	parts := strings.Split(cols, ", ")
	for i, p := range parts {
		parts[i] = prefix + "." + p
	}
	return strings.Join(parts, ", ")
}
