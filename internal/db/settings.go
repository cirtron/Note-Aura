package db

// GetUserSettings returns all per-user settings as a map.
func (d *DB) GetUserSettings(userID int64) (map[string]string, error) {
	rows, err := d.SQL.Query(`SELECT k, v FROM user_settings WHERE user_id=?`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string]string{}
	for rows.Next() {
		var k, v string
		if err := rows.Scan(&k, &v); err != nil {
			return nil, err
		}
		out[k] = v
	}
	return out, rows.Err()
}

// SetUserSetting upserts a single setting. An empty value deletes the key so
// the default (Ollama) backend is used again.
func (d *DB) SetUserSetting(userID int64, k, v string) error {
	if v == "" {
		_, err := d.SQL.Exec(`DELETE FROM user_settings WHERE user_id=? AND k=?`, userID, k)
		return err
	}
	_, err := d.SQL.Exec(`
		INSERT INTO user_settings (user_id, k, v) VALUES (?, ?, ?)
		ON CONFLICT(user_id, k) DO UPDATE SET v=excluded.v`, userID, k, v)
	return err
}
