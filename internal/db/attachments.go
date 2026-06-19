package db

// CreateAttachment records an uploaded file for a note.
func (d *DB) CreateAttachment(noteID int64, path, mime string, bytes int64) (int64, error) {
	res, err := d.SQL.Exec(`
		INSERT INTO attachments (note_id, path, mime, bytes) VALUES (?, ?, ?, ?)`,
		noteID, path, mime, bytes)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

// AttachmentsForNote lists a note's attachments.
func (d *DB) AttachmentsForNote(noteID int64) ([]Attachment, error) {
	rows, err := d.SQL.Query(`
		SELECT id, note_id, path, mime, bytes, ocr_text FROM attachments
		WHERE note_id=? ORDER BY id`, noteID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Attachment
	for rows.Next() {
		var a Attachment
		if err := rows.Scan(&a.ID, &a.NoteID, &a.Path, &a.Mime, &a.Bytes, &a.OCRText); err != nil {
			return nil, err
		}
		out = append(out, a)
	}
	return out, rows.Err()
}

// SetAttachmentOCR stores extracted text for an attachment.
func (d *DB) SetAttachmentOCR(id int64, text string) error {
	_, err := d.SQL.Exec(`UPDATE attachments SET ocr_text=? WHERE id=?`, text, id)
	return err
}
