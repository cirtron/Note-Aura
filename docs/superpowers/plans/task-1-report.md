# Task 1 Report: `stopped` column + stop/retry/job-delete DB methods

## Files Changed

### `internal/db/db.go`
- **Notes schema const (~line 202):** Added `stopped INTEGER NOT NULL DEFAULT 0,` after the `error` line.
- **Migration block (~line 411):** Added `sqlDB.Exec(`ALTER TABLE notes ADD COLUMN stopped INTEGER NOT NULL DEFAULT 0`)` after the `ALTER TABLE jobs ADD COLUMN params` line.
- **`Note` struct (~line 79):** Added `Stopped bool // true when the owner stopped AI processing (status will be 'failed')` after `AIMillis`.

### `internal/db/notes.go`
- **`noteCols` (line 8):** Appended `, stopped` to the column list.
- **`scanNote` (lines 12-14):** Added `&n.Stopped` as final scan target (after `&n.AIMillis`).
- **Inline scan in `ListSharedWithUser` (~line 211):** Added `&n.Stopped,` after `&n.AIMillis,` and before `&n.OwnerEmail`.
- **Added `StopNote(id int64) error`:** Sets `status='failed'`, `stopped=1`, `error='stopped by user'`.
- **Added `ClearStopped(id int64) error`:** Sets `stopped=0` for the given note id.

### `internal/db/groups.go`
- **Inline scan in `ListGroupNotes` (~line 274):** Added `&n.Stopped,` after `&n.AIMillis,` and before `&n.OwnerEmail`.

### `internal/db/jobs.go`
- **Added `DeleteJobsForNote(noteID int64) error`:** Deletes all rows in `jobs` for the given note id.

### `internal/db/stopped_test.go` (new file)
- Created the test per the plan spec.

## Test Commands and Output

### Step 2 — Confirm test fails before implementation:
```
$ go test ./internal/db/ -run TestStopNoteAndRetryClears -v
# note-aura/internal/db [note-aura/internal/db.test]
internal\db\stopped_test.go:21:14: d.StopNote undefined (type *DB has no field or method StopNote)
internal\db\stopped_test.go:25:32: n.Stopped undefined (type *Note has no field or method Stopped)
internal\db\stopped_test.go:26:78: n.Stopped undefined (type *Note has no field or method Stopped)
internal\db\stopped_test.go:29:14: d.DeleteJobsForNote undefined (type *DB has no field or method DeleteJobsForNote)
internal\db\stopped_test.go:36:14: d.ClearStopped undefined (type *DB has no field or method ClearStopped)
internal\db\stopped_test.go:40:7: n.Stopped undefined (type *Note has no field or method Stopped)
FAIL	note-aura/internal/db [build failed]
```

### Step 7 — Confirm test passes after implementation:
```
$ go test ./internal/db/ -run TestStopNoteAndRetryClears -v
=== RUN   TestStopNoteAndRetryClears
--- PASS: TestStopNoteAndRetryClears (0.01s)
PASS
ok  	note-aura/internal/db	0.462s
```

### Step 8 — Build + full db tests:
```
$ go build ./... && go test ./internal/db/
BUILD OK
ok  	note-aura/internal/db	2.427s
```

## Concerns

None. All three scan sites were updated correctly (scanNote, ListSharedWithUser inline scan in notes.go, ListGroupNotes inline scan in groups.go). The migration is idempotent. No CHECK constraints were added or modified. All db tests pass.
