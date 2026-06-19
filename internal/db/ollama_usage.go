package db

// OllamaUsedToday returns how many Ollama AI uses a user has consumed today
// (local date).
func (d *DB) OllamaUsedToday(userID int64) int64 {
	var n int64
	d.SQL.QueryRow(
		`SELECT count FROM ollama_usage WHERE user_id=? AND day=date('now','localtime')`, userID,
	).Scan(&n)
	return n
}

// IncrementOllamaUsage records one Ollama AI use for a user today.
func (d *DB) IncrementOllamaUsage(userID int64) error {
	_, err := d.SQL.Exec(`
		INSERT INTO ollama_usage (user_id, day, count) VALUES (?, date('now','localtime'), 1)
		ON CONFLICT(user_id, day) DO UPDATE SET count = count + 1`, userID)
	return err
}
