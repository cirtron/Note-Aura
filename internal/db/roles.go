package db

import (
	"database/sql"
	"strings"
)

func scanRole(s interface{ Scan(...any) error }) (*Role, error) {
	var r Role
	if err := s.Scan(&r.Slug, &r.Label, &r.IsBuiltin, &r.CapacityMB, &r.MaxGroups,
		&r.CanUseAI, &r.OllamaDailyLimit, &r.CanEditPrompts, &r.InviteLimit, &r.UploadTypes); err != nil {
		return nil, err
	}
	return &r, nil
}

const roleCols = `slug, label, is_builtin, capacity_mb, max_groups, can_use_ai, ollama_daily_limit, can_edit_prompts, invite_limit, upload_types`

// ListRoles returns all roles (built-in first, then alphabetical).
func (d *DB) ListRoles() ([]*Role, error) {
	rows, err := d.SQL.Query(`SELECT ` + roleCols + ` FROM roles ORDER BY is_builtin DESC, slug`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []*Role
	for rows.Next() {
		r, err := scanRole(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// GetRole returns a role or ErrNotFound.
func (d *DB) GetRole(slug string) (*Role, error) {
	row := d.SQL.QueryRow(`SELECT `+roleCols+` FROM roles WHERE slug=?`, slug)
	r, err := scanRole(row)
	if err != nil {
		if isNoRows(err) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return r, nil
}

// UpsertRole creates or updates a role. is_builtin is preserved on update.
func (d *DB) UpsertRole(r Role) error {
	r.Slug = strings.ToLower(strings.TrimSpace(r.Slug))
	if r.Slug == "" {
		return nil
	}
	_, err := d.SQL.Exec(`
		INSERT INTO roles (slug, label, is_builtin, capacity_mb, max_groups, can_use_ai, ollama_daily_limit, can_edit_prompts, invite_limit, upload_types)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(slug) DO UPDATE SET
		    label=excluded.label, capacity_mb=excluded.capacity_mb,
		    max_groups=excluded.max_groups, can_use_ai=excluded.can_use_ai,
		    ollama_daily_limit=excluded.ollama_daily_limit, can_edit_prompts=excluded.can_edit_prompts,
		    invite_limit=excluded.invite_limit, upload_types=excluded.upload_types`,
		r.Slug, r.Label, r.IsBuiltin, r.CapacityMB, r.MaxGroups, r.CanUseAI, r.OllamaDailyLimit, r.CanEditPrompts, r.InviteLimit, r.UploadTypes)
	return err
}

// DeleteRole removes a custom role and reassigns its users to 'user'. Built-in
// roles cannot be deleted.
func (d *DB) DeleteRole(slug string) error {
	var builtin bool
	if err := d.SQL.QueryRow(`SELECT is_builtin FROM roles WHERE slug=?`, slug).Scan(&builtin); err != nil {
		if isNoRows(err) {
			return nil
		}
		return err
	}
	if builtin {
		return nil
	}
	if _, err := d.SQL.Exec(`UPDATE users SET role_slug='user' WHERE role_slug=?`, slug); err != nil {
		return err
	}
	_, err := d.SQL.Exec(`DELETE FROM roles WHERE slug=?`, slug)
	return err
}

// UserHasCloudAI reports whether the user has configured their own
// OpenAI-compatible backend (base URL + API key) in settings.
func (d *DB) UserHasCloudAI(userID int64) bool {
	var base, key string
	d.SQL.QueryRow(`SELECT v FROM user_settings WHERE user_id=? AND k='ai_base_url'`, userID).Scan(&base)
	d.SQL.QueryRow(`SELECT v FROM user_settings WHERE user_id=? AND k='ai_api_key'`, userID).Scan(&key)
	return strings.TrimSpace(base) != "" && strings.TrimSpace(key) != ""
}

// UserAICapability reports whether a user may use AI at all, and whether they
// would run on the built-in Ollama backend. AI is allowed when the user has
// their own cloud key OR their role permits Ollama. Admins always may.
// usingOllama is true when no personal cloud backend is configured (so the
// daily Ollama limit applies).
func (d *DB) UserAICapability(userID int64) (canAI, usingOllama bool) {
	var isAdmin bool
	var roleSlug string
	if err := d.SQL.QueryRow(`SELECT is_admin, role_slug FROM users WHERE id=?`, userID).Scan(&isAdmin, &roleSlug); err != nil {
		return true, true // fail open
	}
	hasCloud := d.UserHasCloudAI(userID)
	usingOllama = !hasCloud
	if isAdmin {
		return true, usingOllama
	}
	var ollamaAllowed bool
	if err := d.SQL.QueryRow(`SELECT can_use_ai FROM roles WHERE slug=?`, roleSlug).Scan(&ollamaAllowed); err != nil {
		ollamaAllowed = true
	}
	return hasCloud || ollamaAllowed, usingOllama
}

// InviteLimit resolves a user's new-user invitation limit (<0 = unlimited,
// 0 = none). Admins are unlimited.
func (d *DB) InviteLimit(userID int64) int64 {
	var isAdmin bool
	var roleSlug string
	var override sql.NullInt64
	if err := d.SQL.QueryRow(`SELECT is_admin, role_slug, invite_override FROM users WHERE id=?`, userID).
		Scan(&isAdmin, &roleSlug, &override); err != nil {
		return 0
	}
	if isAdmin {
		return -1
	}
	if override.Valid {
		return override.Int64
	}
	var limit int64
	d.SQL.QueryRow(`SELECT invite_limit FROM roles WHERE slug=?`, roleSlug).Scan(&limit)
	return limit
}

// OllamaDailyLimit resolves a user's daily Ollama-use limit (0 = unlimited).
// Admins are unlimited.
func (d *DB) OllamaDailyLimit(userID int64) int64 {
	var isAdmin bool
	var roleSlug string
	var override sql.NullInt64
	if err := d.SQL.QueryRow(`SELECT is_admin, role_slug, ollama_daily_override FROM users WHERE id=?`, userID).
		Scan(&isAdmin, &roleSlug, &override); err != nil {
		return 0
	}
	if isAdmin {
		return 0
	}
	if override.Valid {
		return override.Int64
	}
	var limit int64
	d.SQL.QueryRow(`SELECT ollama_daily_limit FROM roles WHERE slug=?`, roleSlug).Scan(&limit)
	return limit
}

// StorageUsedBytes returns the bytes a user's notes consume from attachments and
// note text. Inline editor images (on disk) are added by the caller.
func (d *DB) StorageUsedBytes(userID int64) int64 {
	var attach, text int64
	d.SQL.QueryRow(`
		SELECT COALESCE(SUM(a.bytes),0) FROM attachments a
		JOIN notes n ON n.id = a.note_id
		WHERE n.owner_id=? AND n.deleted_at IS NULL`, userID).Scan(&attach)
	d.SQL.QueryRow(`
		SELECT COALESCE(SUM(LENGTH(body_md)),0) FROM notes
		WHERE owner_id=? AND deleted_at IS NULL`, userID).Scan(&text)
	return attach + text
}
