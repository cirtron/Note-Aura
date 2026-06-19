package db

// Chunk is a stored text chunk plus its raw embedding bytes (float32 LE).
type Chunk struct {
	NoteID    int64
	Index     int
	Text      string
	Embedding []byte
}

// ReplaceChunks deletes a note's existing chunks and inserts the new set.
func (d *DB) ReplaceChunks(noteID int64, chunks []Chunk) error {
	if _, err := d.SQL.Exec(`DELETE FROM note_chunks WHERE note_id=?`, noteID); err != nil {
		return err
	}
	for _, c := range chunks {
		if _, err := d.SQL.Exec(`
			INSERT INTO note_chunks (note_id, chunk_index, chunk_text, embedding)
			VALUES (?, ?, ?, ?)`, noteID, c.Index, c.Text, c.Embedding); err != nil {
			return err
		}
	}
	return nil
}

// ChunksAccessibleBy returns every chunk in notes the user can read (owned or
// shared), for RAG retrieval. Only chunks of ready notes are returned.
func (d *DB) ChunksAccessibleBy(userID int64) ([]Chunk, error) {
	rows, err := d.SQL.Query(`
		SELECT c.note_id, c.chunk_index, c.chunk_text, c.embedding
		FROM note_chunks c
		JOIN notes n ON n.id = c.note_id
		WHERE n.deleted_at IS NULL AND n.status='ready'
		  AND (n.owner_id=?
		    OR c.note_id IN (SELECT note_id FROM note_shares WHERE shared_with_user_id=?)
		    OR c.note_id IN (SELECT ngs.note_id FROM note_group_shares ngs
		                     JOIN group_members m ON m.group_id = ngs.group_id WHERE m.user_id=?))`,
		userID, userID, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Chunk
	for rows.Next() {
		var c Chunk
		if err := rows.Scan(&c.NoteID, &c.Index, &c.Text, &c.Embedding); err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}
