package db

import "strings"

// TagsForNote returns the tag names attached to a note.
func (d *DB) TagsForNote(noteID int64) ([]string, error) {
	rows, err := d.SQL.Query(`
		SELECT t.name FROM note_tags nt
		JOIN tags t ON t.id = nt.tag_id
		WHERE nt.note_id=? ORDER BY t.name`, noteID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, err
		}
		out = append(out, name)
	}
	return out, rows.Err()
}

// TagsWithCounts lists an owner's tags with how many live notes use each, for
// the filter sidebar, most-used first.
func (d *DB) TagsWithCounts(ownerID int64) ([]CountRow, error) {
	rows, err := d.SQL.Query(`
		SELECT t.name, COUNT(nt.note_id) AS cnt
		FROM tags t
		JOIN note_tags nt ON nt.tag_id = t.id
		JOIN notes n ON n.id = nt.note_id AND n.deleted_at IS NULL
		WHERE t.owner_id=?
		GROUP BY t.id
		ORDER BY cnt DESC, t.name`, ownerID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []CountRow
	for rows.Next() {
		var r CountRow
		if err := rows.Scan(&r.Name, &r.Count); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// upsertTag finds or creates a tag for an owner, returning its id.
func (d *DB) upsertTag(ownerID int64, name string) (int64, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return 0, nil
	}
	var id int64
	err := d.SQL.QueryRow(`SELECT id FROM tags WHERE owner_id=? AND name=?`, ownerID, name).Scan(&id)
	if err == nil {
		return id, nil
	}
	if !isNoRows(err) {
		return 0, err
	}
	res, err := d.SQL.Exec(`INSERT INTO tags (owner_id, name) VALUES (?, ?)`, ownerID, name)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

// SetNoteTags replaces a note's tags of the given source ('ai' or 'manual')
// with the provided names. Tags of the other source are left untouched.
func (d *DB) SetNoteTags(ownerID, noteID int64, source string, names []string) error {
	if _, err := d.SQL.Exec(`DELETE FROM note_tags WHERE note_id=? AND source=?`, noteID, source); err != nil {
		return err
	}
	seen := map[string]bool{}
	for _, name := range names {
		key := strings.ToLower(strings.TrimSpace(name))
		if key == "" || seen[key] {
			continue
		}
		seen[key] = true
		tagID, err := d.upsertTag(ownerID, strings.TrimSpace(name))
		if err != nil {
			return err
		}
		if tagID == 0 {
			continue
		}
		if _, err := d.SQL.Exec(
			`INSERT OR IGNORE INTO note_tags (note_id, tag_id, source) VALUES (?, ?, ?)`,
			noteID, tagID, source); err != nil {
			return err
		}
	}
	return nil
}
