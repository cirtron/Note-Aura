package db

import "strings"

// Holiday is one public holiday for a country on a date.
type Holiday struct {
	CountryCode string
	Date        string // YYYY-MM-DD
	Name        string
}

// InsertHolidays upserts holidays (idempotent on country+date+name).
func (d *DB) InsertHolidays(hs []Holiday) (int, error) {
	n := 0
	for _, h := range hs {
		h.CountryCode = strings.ToUpper(strings.TrimSpace(h.CountryCode))
		h.Date = strings.TrimSpace(h.Date)
		h.Name = strings.TrimSpace(h.Name)
		if h.CountryCode == "" || h.Date == "" || h.Name == "" {
			continue
		}
		res, err := d.SQL.Exec(
			`INSERT OR IGNORE INTO holidays (country_code, date, name) VALUES (?, ?, ?)`,
			h.CountryCode, h.Date, h.Name)
		if err != nil {
			return n, err
		}
		if c, _ := res.RowsAffected(); c > 0 {
			n++
		}
	}
	return n, nil
}

// HolidaysByCountriesRange returns holidays for the given countries with date in
// [from, to], ordered by date.
func (d *DB) HolidaysByCountriesRange(countries []string, from, to string) ([]Holiday, error) {
	if len(countries) == 0 {
		return nil, nil
	}
	placeholders := strings.TrimSuffix(strings.Repeat("?,", len(countries)), ",")
	args := make([]any, 0, len(countries)+2)
	for _, c := range countries {
		args = append(args, strings.ToUpper(strings.TrimSpace(c)))
	}
	args = append(args, from, to)
	rows, err := d.SQL.Query(
		`SELECT country_code, date, name FROM holidays
		 WHERE country_code IN (`+placeholders+`) AND date>=? AND date<=?
		 ORDER BY date, country_code`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Holiday
	for rows.Next() {
		var h Holiday
		if err := rows.Scan(&h.CountryCode, &h.Date, &h.Name); err != nil {
			return nil, err
		}
		out = append(out, h)
	}
	return out, rows.Err()
}

// DistinctHolidayCountries lists country codes that have holiday data, with the
// number of entries (CountRow.Name = country code).
func (d *DB) DistinctHolidayCountries() ([]CountRow, error) {
	rows, err := d.SQL.Query(
		`SELECT country_code, COUNT(*) FROM holidays GROUP BY country_code ORDER BY country_code`)
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

// DeleteHolidaysByCountry removes all holidays for a country.
func (d *DB) DeleteHolidaysByCountry(cc string) error {
	_, err := d.SQL.Exec(`DELETE FROM holidays WHERE country_code=?`, strings.ToUpper(strings.TrimSpace(cc)))
	return err
}
