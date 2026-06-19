// Package db owns the SQLite connection, schema, and queries.
package db

import (
	"database/sql"
	"errors"
	"fmt"
	"time"

	_ "modernc.org/sqlite"
)

// ErrNotFound is returned by query helpers when a row does not exist.
var ErrNotFound = errors.New("not found")

// DB wraps *sql.DB so query helpers hang off one type.
type DB struct {
	SQL *sql.DB
}

// ----- model structs -----

type User struct {
	ID                  int64
	Email               string
	PasswordHash        string
	IsAdmin             bool
	RoleSlug            string
	CapacityOverrideMB  sql.NullInt64 // per-user override; NULL = use role default
	OllamaDailyOverride sql.NullInt64 // per-user daily Ollama-use override; NULL = role
	EmailVerified       bool
	VerifyToken         string
	InviteOverride      sql.NullInt64 // per-user invitation-limit override; NULL = role
	Suspended           bool
	CreatedAt           time.Time
	LastSeenAt          sql.NullTime // last time the user was active; NULL = never since this was added
	LastSeenIP          string       // IP address of the last visit
	EmailToken          string       // secret token for inbound email→note plus-addressing
	ResetToken          string       // password-reset token ("" = none)
	ResetExpires        int64        // reset token expiry (unix seconds; 0 = none)
}

// Role groups privilege/quota settings that an admin can assign to users.
type Role struct {
	Slug             string
	Label            string
	IsBuiltin        bool
	CapacityMB       int64 // storage limit in MB; 0 = unlimited
	MaxGroups        int64 // groups a user may CREATE; <0 = unlimited, 0 = none
	CanUseAI         bool  // may use the built-in Ollama backend
	OllamaDailyLimit int64 // max Ollama AI uses per day; 0 = unlimited
	CanEditPrompts   bool   // may customize their own AI prompts (cloud backend)
	InviteLimit      int64  // new-user invitations allowed; <0 = unlimited, 0 = none
	UploadTypes      string // allowed upload categories/extensions (comma list); "" = none, "*" = any
}

type Session struct {
	ID        string
	UserID    int64
	ExpiresAt time.Time
	CreatedAt time.Time
}

type Note struct {
	ID         int64
	OwnerID    int64
	Title      string
	BodyMd     string // Markdown source
	BodyText   string
	Summary    string
	SourceType string // manual | url | youtube | image
	SourceRef  string
	Status     string // processing | ready | failed
	Error      string
	SummaryLang string // AI summary/title language code ("" = auto)
	CreatedAt  time.Time
	UpdatedAt  time.Time
	AIMillis   int64 // wall-clock ms the last AI processing run took (0 = none)
	Stopped    bool  // true when the owner stopped AI processing (status will be 'failed')

	CategoryID sql.NullInt64

	// Schedule (calendar). Loaded on demand via fillSchedule / range queries.
	EventDate       string // YYYY-MM-DD ("" = unscheduled)
	StartTime       string // HH:MM ("" = none)
	EndTime         string // HH:MM
	AllDay          bool
	ReminderMinutes sql.NullInt64 // minutes before start; NULL = no reminder

	// Joined / computed fields (not columns).
	Tags          []string
	CategoryName  string
	CategoryColor string
	CanEdit       bool // for shared notes
	OwnerEmail    string
}

type Category struct {
	ID      int64
	OwnerID int64
	Name    string
	Color   string
}

// CountRow is a name + usage count for filter sidebars.
type CountRow struct {
	Name  string
	Color string
	Count int
}

type Tag struct {
	ID      int64
	OwnerID int64
	Name    string
}

type Share struct {
	NoteID            int64
	SharedWithUserID  int64
	SharedWithEmail   string
	CanEdit           bool
	CreatedAt         time.Time
}

type Group struct {
	ID        int64
	OwnerID   int64
	Name      string
	CreatedAt time.Time

	// Computed for listings.
	OwnerEmail  string
	MemberCount int
	IsOwner     bool
	CanWrite    bool // for the current member's view
}

type GroupMember struct {
	UserID   int64
	Email    string
	CanWrite bool
	IsAdmin  bool
}

type Attachment struct {
	ID      int64
	NoteID  int64
	Path    string
	Mime    string
	Bytes   int64
	OCRText string
}

type Job struct {
	ID        int64
	NoteID    int64
	Kind      string
	Status    string // queued | running | failed | done
	Attempts  int
	LastError string
	Params    string // comma list of AI fields to (re)generate
}

const schema = `
CREATE TABLE IF NOT EXISTS users (
    id            INTEGER PRIMARY KEY AUTOINCREMENT,
    email         TEXT    NOT NULL UNIQUE,
    password_hash TEXT    NOT NULL,
    is_admin      INTEGER NOT NULL DEFAULT 0,
    created_at    TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS user_invitations (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    inviter_id  INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    email       TEXT    NOT NULL,
    token       TEXT    NOT NULL UNIQUE,
    lang        TEXT    NOT NULL DEFAULT '',
    accepted_at TIMESTAMP,
    created_at  TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX IF NOT EXISTS idx_invitations_inviter ON user_invitations(inviter_id);

CREATE TABLE IF NOT EXISTS sessions (
    id         TEXT    PRIMARY KEY,
    user_id    INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    expires_at TIMESTAMP NOT NULL,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS notes (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    owner_id    INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    title       TEXT    NOT NULL DEFAULT '',
    body_html   TEXT    NOT NULL DEFAULT '',
    body_md     TEXT    NOT NULL DEFAULT '',
    body_text   TEXT    NOT NULL DEFAULT '',
    summary     TEXT    NOT NULL DEFAULT '',
    source_type TEXT    NOT NULL DEFAULT 'manual' CHECK (source_type IN ('manual','url','youtube','image')),
    source_ref  TEXT    NOT NULL DEFAULT '',
    status      TEXT    NOT NULL DEFAULT 'ready' CHECK (status IN ('processing','ready','failed')),
    error       TEXT    NOT NULL DEFAULT '',
    stopped     INTEGER NOT NULL DEFAULT 0,
    summary_lang TEXT   NOT NULL DEFAULT '',
    category_id INTEGER REFERENCES categories(id),
    event_date  TEXT    NOT NULL DEFAULT '',
    start_time  TEXT    NOT NULL DEFAULT '',
    end_time    TEXT    NOT NULL DEFAULT '',
    all_day     INTEGER NOT NULL DEFAULT 0,
    reminder_minutes INTEGER,
    reminder_sent_at TIMESTAMP,
    created_at  TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at  TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    deleted_at  TIMESTAMP
);
CREATE INDEX IF NOT EXISTS idx_notes_owner ON notes(owner_id);

CREATE TABLE IF NOT EXISTS categories (
    id       INTEGER PRIMARY KEY AUTOINCREMENT,
    owner_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    name     TEXT    NOT NULL,
    color    TEXT    NOT NULL DEFAULT '',
    UNIQUE(owner_id, name)
);

CREATE TABLE IF NOT EXISTS tags (
    id       INTEGER PRIMARY KEY AUTOINCREMENT,
    owner_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    name     TEXT    NOT NULL,
    UNIQUE(owner_id, name)
);

CREATE TABLE IF NOT EXISTS note_tags (
    note_id INTEGER NOT NULL REFERENCES notes(id) ON DELETE CASCADE,
    tag_id  INTEGER NOT NULL REFERENCES tags(id) ON DELETE CASCADE,
    source  TEXT    NOT NULL DEFAULT 'manual' CHECK (source IN ('ai','manual')),
    PRIMARY KEY (note_id, tag_id)
);

CREATE TABLE IF NOT EXISTS user_groups (
    id         INTEGER PRIMARY KEY AUTOINCREMENT,
    owner_id   INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    name       TEXT    NOT NULL,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX IF NOT EXISTS idx_groups_owner ON user_groups(owner_id);

CREATE TABLE IF NOT EXISTS group_members (
    group_id  INTEGER NOT NULL REFERENCES user_groups(id) ON DELETE CASCADE,
    user_id   INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    can_write INTEGER NOT NULL DEFAULT 0,
    is_admin  INTEGER NOT NULL DEFAULT 0,
    joined_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (group_id, user_id)
);

CREATE TABLE IF NOT EXISTS group_invites (
    group_id   INTEGER NOT NULL REFERENCES user_groups(id) ON DELETE CASCADE,
    user_id    INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    can_write  INTEGER NOT NULL DEFAULT 0,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (group_id, user_id)
);

CREATE TABLE IF NOT EXISTS user_blocks (
    blocker_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    blocked_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (blocker_id, blocked_id)
);

CREATE TABLE IF NOT EXISTS note_group_shares (
    note_id    INTEGER NOT NULL REFERENCES notes(id) ON DELETE CASCADE,
    group_id   INTEGER NOT NULL REFERENCES user_groups(id) ON DELETE CASCADE,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (note_id, group_id)
);

CREATE TABLE IF NOT EXISTS note_shares (
    note_id             INTEGER NOT NULL REFERENCES notes(id) ON DELETE CASCADE,
    shared_with_user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    can_edit            INTEGER NOT NULL DEFAULT 0,
    created_at          TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (note_id, shared_with_user_id)
);

CREATE TABLE IF NOT EXISTS attachments (
    id         INTEGER PRIMARY KEY AUTOINCREMENT,
    note_id    INTEGER NOT NULL REFERENCES notes(id) ON DELETE CASCADE,
    path       TEXT    NOT NULL,
    mime       TEXT    NOT NULL DEFAULT '',
    bytes      INTEGER NOT NULL DEFAULT 0,
    ocr_text   TEXT    NOT NULL DEFAULT '',
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX IF NOT EXISTS idx_attach_note ON attachments(note_id);

CREATE TABLE IF NOT EXISTS note_chunks (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    note_id     INTEGER NOT NULL REFERENCES notes(id) ON DELETE CASCADE,
    chunk_index INTEGER NOT NULL,
    chunk_text  TEXT    NOT NULL,
    embedding   BLOB    NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_chunks_note ON note_chunks(note_id);

CREATE TABLE IF NOT EXISTS jobs (
    id         INTEGER PRIMARY KEY AUTOINCREMENT,
    note_id    INTEGER NOT NULL REFERENCES notes(id) ON DELETE CASCADE,
    kind       TEXT    NOT NULL DEFAULT 'process',
    params     TEXT    NOT NULL DEFAULT '',
    status     TEXT    NOT NULL DEFAULT 'queued' CHECK (status IN ('queued','running','failed','done')),
    attempts   INTEGER NOT NULL DEFAULT 0,
    last_error TEXT    NOT NULL DEFAULT '',
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX IF NOT EXISTS idx_jobs_status ON jobs(status);

CREATE TABLE IF NOT EXISTS user_settings (
    user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    k       TEXT    NOT NULL,
    v       TEXT    NOT NULL DEFAULT '',
    PRIMARY KEY (user_id, k)
);

CREATE TABLE IF NOT EXISTS app_settings (
    k TEXT PRIMARY KEY,
    v TEXT NOT NULL DEFAULT ''
);

CREATE TABLE IF NOT EXISTS holidays (
    country_code TEXT NOT NULL,
    date         TEXT NOT NULL,   -- YYYY-MM-DD
    name         TEXT NOT NULL,
    PRIMARY KEY (country_code, date, name)
);
CREATE INDEX IF NOT EXISTS idx_holidays_date ON holidays(date);

CREATE TABLE IF NOT EXISTS roles (
    slug        TEXT PRIMARY KEY,
    label       TEXT    NOT NULL,
    is_builtin  INTEGER NOT NULL DEFAULT 0,
    capacity_mb INTEGER NOT NULL DEFAULT 0,   -- 0 = unlimited
    max_groups  INTEGER NOT NULL DEFAULT 0,   -- groups a user may create; <0 = unlimited, 0 = none
    can_use_ai  INTEGER NOT NULL DEFAULT 1,   -- may use built-in Ollama
    ollama_daily_limit INTEGER NOT NULL DEFAULT 0,  -- Ollama uses/day; 0 = unlimited
    can_edit_prompts INTEGER NOT NULL DEFAULT 0  -- may customize own prompts (cloud)
);

CREATE TABLE IF NOT EXISTS ollama_usage (
    user_id INTEGER NOT NULL,
    day     TEXT    NOT NULL,   -- YYYY-MM-DD (local)
    count   INTEGER NOT NULL DEFAULT 0,
    PRIMARY KEY (user_id, day)
);

CREATE VIRTUAL TABLE IF NOT EXISTS notes_fts USING fts5(title, body_text);
`

// Open opens (creating if needed) the SQLite database and applies the schema.
func Open(path string) (*DB, error) {
	dsn := path + "?_pragma=foreign_keys(1)&_pragma=busy_timeout(5000)"
	sqlDB, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("open sqlite: %w", err)
	}
	// modernc.org/sqlite is safest with a single writer connection.
	sqlDB.SetMaxOpenConns(1)
	if _, err := sqlDB.Exec(schema); err != nil {
		return nil, fmt.Errorf("apply schema: %w", err)
	}
	// Lightweight migrations for databases created by an earlier version. Each
	// ALTER fails harmlessly with "duplicate column name" when already applied.
	sqlDB.Exec(`ALTER TABLE users ADD COLUMN is_admin INTEGER NOT NULL DEFAULT 0`)
	sqlDB.Exec(`ALTER TABLE notes ADD COLUMN category_id INTEGER`)
	sqlDB.Exec(`ALTER TABLE notes ADD COLUMN body_md TEXT NOT NULL DEFAULT ''`)
	sqlDB.Exec(`ALTER TABLE notes ADD COLUMN event_date TEXT NOT NULL DEFAULT ''`)
	sqlDB.Exec(`ALTER TABLE notes ADD COLUMN start_time TEXT NOT NULL DEFAULT ''`)
	sqlDB.Exec(`ALTER TABLE notes ADD COLUMN end_time TEXT NOT NULL DEFAULT ''`)
	sqlDB.Exec(`ALTER TABLE notes ADD COLUMN all_day INTEGER NOT NULL DEFAULT 0`)
	sqlDB.Exec(`ALTER TABLE notes ADD COLUMN reminder_minutes INTEGER`)
	sqlDB.Exec(`ALTER TABLE notes ADD COLUMN reminder_sent_at TIMESTAMP`)
	sqlDB.Exec(`ALTER TABLE notes ADD COLUMN summary_lang TEXT NOT NULL DEFAULT ''`)
	sqlDB.Exec(`ALTER TABLE notes ADD COLUMN ai_ms INTEGER NOT NULL DEFAULT 0`)
	sqlDB.Exec(`CREATE TABLE IF NOT EXISTS system_logs (
		id         INTEGER PRIMARY KEY AUTOINCREMENT,
		created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
		level      TEXT NOT NULL DEFAULT 'info',
		category   TEXT NOT NULL DEFAULT '',
		message    TEXT NOT NULL DEFAULT ''
	)`)
	sqlDB.Exec(`CREATE INDEX IF NOT EXISTS idx_syslog_id ON system_logs(id DESC)`)
	sqlDB.Exec(`ALTER TABLE users ADD COLUMN role_slug TEXT NOT NULL DEFAULT 'user'`)
	sqlDB.Exec(`ALTER TABLE users ADD COLUMN capacity_override_mb INTEGER`)
	sqlDB.Exec(`ALTER TABLE users ADD COLUMN ollama_daily_override INTEGER`)
	sqlDB.Exec(`ALTER TABLE roles ADD COLUMN ollama_daily_limit INTEGER NOT NULL DEFAULT 0`)
	sqlDB.Exec(`ALTER TABLE roles ADD COLUMN can_edit_prompts INTEGER NOT NULL DEFAULT 0`)
	sqlDB.Exec(`ALTER TABLE roles ADD COLUMN invite_limit INTEGER NOT NULL DEFAULT 3`)
	sqlDB.Exec(`ALTER TABLE roles ADD COLUMN upload_types TEXT NOT NULL DEFAULT 'image'`)
	sqlDB.Exec(`ALTER TABLE group_members ADD COLUMN is_admin INTEGER NOT NULL DEFAULT 0`)
	sqlDB.Exec(`ALTER TABLE users ADD COLUMN email_verified INTEGER NOT NULL DEFAULT 1`)
	sqlDB.Exec(`ALTER TABLE users ADD COLUMN verify_token TEXT NOT NULL DEFAULT ''`)
	sqlDB.Exec(`ALTER TABLE users ADD COLUMN invite_override INTEGER`)
	sqlDB.Exec(`ALTER TABLE users ADD COLUMN suspended INTEGER NOT NULL DEFAULT 0`)
	sqlDB.Exec(`ALTER TABLE users ADD COLUMN last_seen_at TIMESTAMP`)
	sqlDB.Exec(`ALTER TABLE users ADD COLUMN last_seen_ip TEXT NOT NULL DEFAULT ''`)
	sqlDB.Exec(`ALTER TABLE users ADD COLUMN email_token TEXT NOT NULL DEFAULT ''`)
	sqlDB.Exec(`ALTER TABLE users ADD COLUMN reset_token TEXT NOT NULL DEFAULT ''`)
	sqlDB.Exec(`ALTER TABLE users ADD COLUMN reset_expires INTEGER NOT NULL DEFAULT 0`)
	sqlDB.Exec(`ALTER TABLE jobs ADD COLUMN params TEXT NOT NULL DEFAULT ''`)
	sqlDB.Exec(`ALTER TABLE notes ADD COLUMN stopped INTEGER NOT NULL DEFAULT 0`)
	sqlDB.Exec(`ALTER TABLE user_invitations ADD COLUMN lang TEXT NOT NULL DEFAULT ''`)
	// Seed the built-in default role (referenced by users.role_slug default).
	sqlDB.Exec(`INSERT OR IGNORE INTO roles (slug, label, is_builtin, capacity_mb, max_groups, can_use_ai, ollama_daily_limit)
		VALUES ('user', 'User', 1, 100, 3, 1, 0)`)
	return &DB{SQL: sqlDB}, nil
}

// Close closes the underlying connection.
func (d *DB) Close() error { return d.SQL.Close() }

func isNoRows(err error) bool { return errors.Is(err, sql.ErrNoRows) }
