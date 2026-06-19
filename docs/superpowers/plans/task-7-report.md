# Task 7 Report — Worker Facebook Branch + Badge

## Files Changed

### `internal/worker/worker.go`
Replaced the simple `case "url":` block in `materialize` with a Facebook-branching version. When `ingest.IsFacebook(note.SourceRef)` is true, it loads `facebook.cookies` from `w.db.GetAppSettings()` and calls `ingest.FetchFacebook(ctx, note.SourceRef, cookies, ingest.EnableHeadless)`. Non-Facebook URLs continue to use the original `ingest.FetchURL` path.

### `internal/server/server.go`
- Added `"note-aura/internal/ingest"` to the import block (it was NOT already imported).
- Added `engine.AddFunc("isFacebook", ingest.IsFacebook)` next to the existing `countryName`/`t`/`langs` registrations (line 79-81 area).

### `web/templates/note_view.html`
Replaced line 9's source badge (`{{.Note.SourceType}}`) with the conditional version:
`{{if and (eq .Note.SourceType "url") (isFacebook .Note.SourceRef)}}facebook{{else}}{{.Note.SourceType}}{{end}}`
The Stop/Stopped UI from Task 3 (lines 10-12) was not touched.

## Import Note

`internal/server/server.go` did NOT already import `note-aura/internal/ingest`. The import was added as part of this task.

## Build / Vet / Test

Commands run from `C:\Project\Note-Aura`:
```
go build ./... && go vet ./... && go test ./... 2>&1 | tail -25
```

Output:
```
?       note-aura                    [no test files]
?       note-aura/cmd/reset          [no test files]
ok      note-aura/internal/ai        (cached)
?       note-aura/internal/auth      [no test files]
?       note-aura/internal/config    [no test files]
ok      note-aura/internal/db        (cached)
ok      note-aura/internal/emailin   0.442s
?       note-aura/internal/holidays  [no test files]
ok      note-aura/internal/i18n      (cached)
ok      note-aura/internal/ingest    0.325s
?       note-aura/internal/mailer    [no test files]
ok      note-aura/internal/markdown  (cached)
?       note-aura/internal/rag       [no test files]
ok      note-aura/internal/reminder  (cached)
ok      note-aura/internal/server    0.692s
?       note-aura/internal/syslog    [no test files]
ok      note-aura/internal/worker    0.291s
go: unlinkat C:\Users\SW\AppData\Local\Temp\go-build...\emailin.test.exe: The process cannot access the file because it is being used by another process.
```

Build: clean. Vet: clean. All test packages: PASS. The trailing `unlinkat` warning is a Windows file-lock artefact from the test runner cleanup — not a test failure (the `emailin` package itself showed `ok`).

## Concerns

None. The Windows `unlinkat` message is cosmetic only and does not indicate any test failure.
