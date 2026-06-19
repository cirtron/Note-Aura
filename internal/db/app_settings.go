package db

// GetAppSettings returns all global settings as a map.
func (d *DB) GetAppSettings() (map[string]string, error) {
	rows, err := d.SQL.Query(`SELECT k, v FROM app_settings`)
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

// SetAppSetting upserts a global setting. An empty value deletes the key so the
// built-in / config.yaml default applies again.
func (d *DB) SetAppSetting(k, v string) error {
	if v == "" {
		_, err := d.SQL.Exec(`DELETE FROM app_settings WHERE k=?`, k)
		return err
	}
	_, err := d.SQL.Exec(`
		INSERT INTO app_settings (k, v) VALUES (?, ?)
		ON CONFLICT(k) DO UPDATE SET v=excluded.v`, k, v)
	return err
}
