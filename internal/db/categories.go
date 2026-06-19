package db

import "strings"

// normalizeCategoryPath cleans a "Parent / Child" path into "Parent/Child"
// (trims each segment, drops empties). Categories are hierarchical via this path.
func normalizeCategoryPath(name string) string {
	parts := strings.Split(name, "/")
	out := parts[:0]
	for _, p := range parts {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	return strings.Join(out, "/")
}

// UpsertCategory finds or creates a category for an owner, returning its id. The
// name may be a "Parent/Child" path for a sub-category.
func (d *DB) UpsertCategory(ownerID int64, name string) (int64, error) {
	name = normalizeCategoryPath(name)
	if name == "" {
		return 0, nil
	}
	var id int64
	err := d.SQL.QueryRow(`SELECT id FROM categories WHERE owner_id=? AND name=?`, ownerID, name).Scan(&id)
	if err == nil {
		return id, nil
	}
	if !isNoRows(err) {
		return 0, err
	}
	res, err := d.SQL.Exec(`INSERT INTO categories (owner_id, name) VALUES (?, ?)`, ownerID, name)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

// CategoriesWithCounts lists an owner's categories with how many live notes use
// each, for the filter sidebar. Empty categories are omitted.
func (d *DB) CategoriesWithCounts(ownerID int64) ([]CountRow, error) {
	rows, err := d.SQL.Query(`
		SELECT c.name, c.color, COUNT(n.id) AS cnt
		FROM categories c
		JOIN notes n ON n.category_id = c.id AND n.deleted_at IS NULL
		WHERE c.owner_id=?
		GROUP BY c.id
		ORDER BY c.name`, ownerID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []CountRow
	for rows.Next() {
		var r CountRow
		if err := rows.Scan(&r.Name, &r.Color, &r.Count); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}
