# Facebook Link Capture + Stop-Processing Button — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Capture Facebook links (authenticated) into note content for the normal AI pipeline, and let users stop a note's in-progress AI work, landing it in a distinct "Stopped" state.

**Architecture:** Facebook stays `source_type='url'`; the worker branches on `ingest.IsFacebook(source_ref)` to a new `ingest.FetchFacebook` (yt-dlp+cookies → authenticated headless → Open-Graph fallback). Stop adds a `stopped` boolean column (no CHECK rebuild), a worker cancel-registry keyed by note id, and a `POST /notes/:id/stop` handler.

**Tech Stack:** Go 1.26, Fiber v2, SQLite (`modernc.org/sqlite`), chromedp (headless), yt-dlp (external), Go `html/template`, `internal/i18n`.

## Global Constraints

- **Not a git repo.** "Commit" steps are no-ops; the checkpoint after each task is `cd /c/Project/Note-Aura && go build ./...` + the task's tests green. Do not `git init`.
- **No new SQLite CHECK enum values.** `source_type` stays in `('manual','url','youtube','image')`; `status` stays in `('processing','ready','failed')`. Facebook = `url` + content branch; Stop = `failed` + `stopped` flag.
- **Migrations are idempotent `ALTER TABLE ... ADD COLUMN` only** (next to the existing `ALTER TABLE jobs ADD COLUMN params ...` at `internal/db/db.go:411`).
- **i18n parity:** every new UI key must exist in all four language maps in `internal/i18n/translations.go` (`en`, `zh-Hant`, `zh-Hans`, `ja`); `TestLanguageKeyParity` enforces it.
- **Run build:** `cd /c/Project/Note-Aura && go build ./...` · **Run all tests:** `go test ./...`

---

# Part 1 — Stop-Processing Button

## Task 1: `stopped` column + stop/retry/job-delete DB methods

**Files:**
- Modify: `internal/db/db.go` (notes schema const ~line 199-214; migration block ~line 411)
- Modify: `internal/db/notes.go` (`noteCols` line 8; `scanNote` lines 10-18; inline scan lines 211-213; add methods)
- Modify: `internal/db/groups.go` (inline scan ~line 274)
- Modify: `internal/db/db.go` `Note` struct (lines 64-88) — add `Stopped bool`
- Modify: `internal/db/jobs.go` (add `DeleteJobsForNote`)
- Test: `internal/db/stopped_test.go` (create)

**Interfaces:**
- Produces: `Note.Stopped bool`; `(*DB) StopNote(id int64) error`; `(*DB) ClearStopped(id int64) error`; `(*DB) DeleteJobsForNote(noteID int64) error`.

- [ ] **Step 1: Write the failing test**

Create `internal/db/stopped_test.go`:

```go
package db

import "testing"

func TestStopNoteAndRetryClears(t *testing.T) {
	d, err := Open(":memory:")
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer d.Close()

	uid, _ := d.CreateUser("u@example.com", "h", true, true, "")
	nid, err := d.CreateNote(&Note{OwnerID: uid, Title: "X", BodyMd: "hi", BodyText: "hi", SourceType: "url", Status: "processing"})
	if err != nil {
		t.Fatalf("create note: %v", err)
	}
	if err := d.EnqueueJob(nid, "process", "summary,tags"); err != nil {
		t.Fatalf("enqueue: %v", err)
	}

	if err := d.StopNote(nid); err != nil {
		t.Fatalf("stop: %v", err)
	}
	n, _ := d.GetNote(nid)
	if n.Status != "failed" || !n.Stopped {
		t.Fatalf("after stop: status=%q stopped=%v, want failed/true", n.Status, n.Stopped)
	}

	if err := d.DeleteJobsForNote(nid); err != nil {
		t.Fatalf("delete jobs: %v", err)
	}
	if _, err := d.ClaimJob(); err != ErrNotFound {
		t.Fatalf("expected no claimable job after delete, got %v", err)
	}

	if err := d.ClearStopped(nid); err != nil {
		t.Fatalf("clear: %v", err)
	}
	n, _ = d.GetNote(nid)
	if n.Stopped {
		t.Fatalf("ClearStopped left stopped=true")
	}
}
```

- [ ] **Step 2: Run the test to verify it fails**

Run: `go test ./internal/db/ -run TestStopNoteAndRetryClears -v`
Expected: FAIL — compile errors (`Note` has no `Stopped`; `StopNote`/`ClearStopped`/`DeleteJobsForNote` undefined).

- [ ] **Step 3: Add the schema column + migration**

In `internal/db/db.go`, in the `notes` table const, add after the `error` line (line 202):

```sql
    stopped     INTEGER NOT NULL DEFAULT 0,
```

In the migration block (next to `ALTER TABLE jobs ADD COLUMN params ...`, ~line 411) add:

```go
	sqlDB.Exec(`ALTER TABLE notes ADD COLUMN stopped INTEGER NOT NULL DEFAULT 0`)
```

- [ ] **Step 4: Add the `Stopped` field**

In `internal/db/db.go`, in the `Note` struct, add after `AIMillis` (line 78):

```go
	Stopped    bool // true when the owner stopped AI processing (status will be 'failed')
```

- [ ] **Step 5: Thread `stopped` through `noteCols` and the three scan sites**

In `internal/db/notes.go`, append `stopped` to `noteCols` (line 8), at the end:

```go
const noteCols = `id, owner_id, title, body_md, body_text, summary, source_type, source_ref, status, error, summary_lang, category_id, created_at, updated_at, ai_ms, stopped`
```

In `scanNote` (lines 12-14), add `&n.Stopped` as the final scan target:

```go
	if err := s.Scan(&n.ID, &n.OwnerID, &n.Title, &n.BodyMd, &n.BodyText,
		&n.Summary, &n.SourceType, &n.SourceRef, &n.Status, &n.Error,
		&n.SummaryLang, &n.CategoryID, &n.CreatedAt, &n.UpdatedAt, &n.AIMillis, &n.Stopped); err != nil {
```

In `internal/db/notes.go` inline scan (lines 211-213), insert `&n.Stopped,` after `&n.AIMillis,` and before `&n.OwnerEmail`:

```go
		if err := rows.Scan(&n.ID, &n.OwnerID, &n.Title, &n.BodyMd, &n.BodyText,
			&n.Summary, &n.SourceType, &n.SourceRef, &n.Status, &n.Error,
			&n.SummaryLang, &n.CategoryID, &n.CreatedAt, &n.UpdatedAt, &n.AIMillis, &n.Stopped, &n.OwnerEmail); err != nil {
```

In `internal/db/groups.go` inline scan (~line 274), likewise insert `&n.Stopped,` after `&n.AIMillis,` and before `&n.OwnerEmail` (or whatever trailing joined column follows). The scan order must match `prefixCols("n", noteCols)+", u.email"`.

- [ ] **Step 6: Add the three methods**

In `internal/db/notes.go`, after `SetNoteStatus` (line 152):

```go
// StopNote marks a note's AI processing as stopped by the owner. It reuses the
// 'failed' status (no CHECK change) plus the stopped flag, which the UI renders
// as a neutral "Stopped" with Retry.
func (d *DB) StopNote(id int64) error {
	_, err := d.SQL.Exec(
		`UPDATE notes SET status='failed', stopped=1, error='stopped by user', updated_at=CURRENT_TIMESTAMP WHERE id=?`, id)
	return err
}

// ClearStopped resets the stopped flag (called when a stopped note is retried).
func (d *DB) ClearStopped(id int64) error {
	_, err := d.SQL.Exec(`UPDATE notes SET stopped=0 WHERE id=?`, id)
	return err
}
```

In `internal/db/jobs.go`, add:

```go
// DeleteJobsForNote removes all queued/running jobs for a note so a stopped note
// cannot be requeued by the worker's FailJob path.
func (d *DB) DeleteJobsForNote(noteID int64) error {
	_, err := d.SQL.Exec(`DELETE FROM jobs WHERE note_id=?`, noteID)
	return err
}
```

- [ ] **Step 7: Run the test to verify it passes**

Run: `go test ./internal/db/ -run TestStopNoteAndRetryClears -v`
Expected: PASS

- [ ] **Step 8: Build + full db tests**

Run: `go build ./... && go test ./internal/db/`
Expected: clean build; all db tests pass (proves the extra `stopped` column didn't break existing scans).

- [ ] **Step 9: Checkpoint commit**

```bash
git add internal/db/db.go internal/db/notes.go internal/db/groups.go internal/db/jobs.go internal/db/stopped_test.go
git commit -m "feat(db): stopped flag + StopNote/ClearStopped/DeleteJobsForNote"
```

---

## Task 2: Worker cancel registry

**Files:**
- Modify: `internal/worker/worker.go` (imports; `Worker` struct lines 29-34; `New` lines 38-40; `process` lines 80-93; add `Cancel`)
- Test: `internal/worker/cancel_test.go` (create)

**Interfaces:**
- Consumes: nothing new.
- Produces: `(*Worker) Cancel(noteID int64)` — cancels the in-flight context for that note if one is running.

- [ ] **Step 1: Write the failing test**

Create `internal/worker/cancel_test.go`:

```go
package worker

import (
	"context"
	"testing"
	"time"
)

func TestCancelRegistry(t *testing.T) {
	w := &Worker{cancels: map[int64]context.CancelFunc{}}

	ctx, cancel := context.WithCancel(context.Background())
	w.register(42, cancel)

	w.Cancel(42) // should cancel ctx
	select {
	case <-ctx.Done():
	case <-time.After(time.Second):
		t.Fatal("Cancel did not cancel the registered context")
	}

	w.deregister(42)
	w.Cancel(42) // no-op, must not panic
}
```

- [ ] **Step 2: Run the test to verify it fails**

Run: `go test ./internal/worker/ -run TestCancelRegistry -v`
Expected: FAIL — `Worker` has no `cancels` field; `register`/`deregister`/`Cancel` undefined.

- [ ] **Step 3: Add the registry to the struct + constructor**

In `internal/worker/worker.go`, add `"sync"` to imports. Change the `Worker` struct (lines 29-34) to:

```go
type Worker struct {
	db       *db.DB
	fallback ai.GlobalConfig // config.yaml defaults; admin app_settings overlay it
	timeout  time.Duration
	notify   chan struct{}

	mu      sync.Mutex
	cancels map[int64]context.CancelFunc // note id -> cancel for the in-flight run
}
```

Change `New` (lines 38-40) to initialize the map:

```go
func New(database *db.DB, fallback ai.GlobalConfig, timeout time.Duration) *Worker {
	return &Worker{db: database, fallback: fallback, timeout: timeout,
		notify: make(chan struct{}, 1), cancels: map[int64]context.CancelFunc{}}
}
```

- [ ] **Step 4: Add register/deregister/Cancel**

In `internal/worker/worker.go`, add:

```go
func (w *Worker) register(noteID int64, cancel context.CancelFunc) {
	w.mu.Lock()
	w.cancels[noteID] = cancel
	w.mu.Unlock()
}

func (w *Worker) deregister(noteID int64) {
	w.mu.Lock()
	delete(w.cancels, noteID)
	w.mu.Unlock()
}

// Cancel aborts the in-flight AI run for a note, if one is currently running.
func (w *Worker) Cancel(noteID int64) {
	w.mu.Lock()
	cancel := w.cancels[noteID]
	w.mu.Unlock()
	if cancel != nil {
		cancel()
	}
}
```

- [ ] **Step 5: Register the cancel in `process` and skip status flips on cancellation**

In `internal/worker/worker.go`, replace `process` (lines 80-93) with:

```go
func (w *Worker) process(job *db.Job) {
	ctx, cancel := context.WithTimeout(context.Background(), w.timeout)
	defer cancel()
	w.register(job.NoteID, cancel)
	defer w.deregister(job.NoteID)

	if err := w.run(ctx, job); err != nil {
		// A user Stop cancels ctx and has already set the note's final state and
		// deleted its jobs — don't fight it by re-marking failed/requeuing.
		if errors.Is(err, context.Canceled) {
			return
		}
		syslog.Errorf("worker", "note %d job %d failed (attempt %d): %v", job.NoteID, job.ID, job.Attempts, err)
		_ = w.db.FailJob(job.ID, job.Attempts, maxAttempts, err.Error())
		if job.Attempts >= maxAttempts {
			_ = w.db.SetNoteStatus(job.NoteID, "failed", err.Error())
		}
		return
	}
	_ = w.db.CompleteJob(job.ID)
}
```

Add `"errors"` to the import block.

- [ ] **Step 6: Run the test to verify it passes**

Run: `go test ./internal/worker/ -run TestCancelRegistry -v`
Expected: PASS

- [ ] **Step 7: Build + worker tests**

Run: `go build ./... && go test ./internal/worker/`
Expected: clean build; worker tests pass.

- [ ] **Step 8: Checkpoint commit**

```bash
git add internal/worker/worker.go internal/worker/cancel_test.go
git commit -m "feat(worker): cancel registry to stop a running AI job"
```

---

## Task 3: Stop endpoint, retry-clears-stopped, and Stop/Stopped UI

**Files:**
- Modify: `internal/server/notes.go` (add `stopNote`; edit `retryNote` lines 335-358)
- Modify: `internal/server/server.go` (route, after line 147)
- Modify: `web/templates/note_view.html` (processing pill lines 8-12; failed block lines 31-40; Stop button)
- Modify: `internal/i18n/translations.go` (new keys ×4 languages)
- Test: `internal/server/stop_test.go` (create)

**Interfaces:**
- Consumes: `(*DB) StopNote`, `(*DB) ClearStopped`, `(*DB) DeleteJobsForNote` (Task 1); `(*Worker) Cancel` (Task 2); existing `(*DB) NoteAccess(id, uid) (isOwner, canEdit bool, err error)`.

- [ ] **Step 1: Write the failing test**

Create `internal/server/stop_test.go`:

```go
package server

import (
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v2"

	"note-aura/internal/db"
)

func TestStopNoteHandler(t *testing.T) {
	database, err := db.Open(":memory:")
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { database.Close() })

	uid, _ := database.CreateUser("u@example.com", "h", true, true, "")
	nid, _ := database.CreateNote(&db.Note{OwnerID: uid, Title: "X", BodyMd: "hi", BodyText: "hi", SourceType: "url", Status: "processing"})
	_ = database.EnqueueJob(nid, "process", "summary")
	user, _ := database.GetUser(uid)

	s := &Server{db: database, worker: nil}

	app := fiber.New()
	app.Use(func(c *fiber.Ctx) error { c.Locals(userLocalKey, user); return c.Next() })
	app.Post("/notes/:id/stop", s.stopNote)

	req := httptest.NewRequest("POST", "/notes/1/stop", nil)
	resp, _ := app.Test(req)
	if resp.StatusCode != fiber.StatusFound {
		t.Fatalf("status = %d, want 302", resp.StatusCode)
	}
	n, _ := database.GetNote(nid)
	if n.Status != "failed" || !n.Stopped {
		t.Fatalf("note not stopped: status=%q stopped=%v", n.Status, n.Stopped)
	}
	if _, err := database.ClaimJob(); err != db.ErrNotFound {
		t.Fatalf("jobs not deleted after stop")
	}
}
```

Note: `Server.worker` is `*worker.Worker`; the handler must tolerate a nil worker (calling `Cancel` only when non-nil) so this DB-focused test needs no running pool.

- [ ] **Step 2: Run the test to verify it fails**

Run: `go test ./internal/server/ -run TestStopNoteHandler -v`
Expected: FAIL — `s.stopNote` undefined.

- [ ] **Step 3: Add the `stopNote` handler + nil-safe Cancel**

In `internal/server/notes.go`, after `retryNote` (line 358), add:

```go
// stopNote cancels a note's in-progress AI processing and marks it Stopped.
func (s *Server) stopNote(c *fiber.Ctx) error {
	u := currentUser(c)
	id, _ := strconv.ParseInt(c.Params("id"), 10, 64)
	_, canEdit, err := s.db.NoteAccess(id, u.ID)
	if err == db.ErrNotFound {
		return fiber.NewError(fiber.StatusNotFound, "note not found")
	}
	if err != nil || !canEdit {
		return fiber.NewError(fiber.StatusForbidden, "you cannot stop this note")
	}
	// Delete jobs first so the worker's FailJob path can't requeue; then cancel any
	// in-flight run; then record the stopped state.
	_ = s.db.DeleteJobsForNote(id)
	if s.worker != nil {
		s.worker.Cancel(id)
	}
	if err := s.db.StopNote(id); err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, err.Error())
	}
	return c.Redirect("/notes/"+strconv.FormatInt(id, 10), fiber.StatusFound)
}
```

- [ ] **Step 4: Make retry clear the stopped flag**

In `internal/server/notes.go`, in `retryNote`, immediately after the successful `SetNoteStatus(id, "processing", "")` (line 345-347), add:

```go
	_ = s.db.ClearStopped(id)
```

- [ ] **Step 5: Register the route**

In `internal/server/server.go`, after the `/notes/:id/retry` route (line 147):

```go
	app.Post("/notes/:id/stop", s.requireAuth, s.stopNote)
```

- [ ] **Step 6: Run the handler test to verify it passes**

Run: `go test ./internal/server/ -run TestStopNoteHandler -v`
Expected: PASS

- [ ] **Step 7: Add i18n keys (all four languages)**

In `internal/i18n/translations.go`, add to each language map (after the `"ask.title"` line, matching the Task-6e insertion point used previously). Values per language:

en:
```go
		"notes.stop":      "Stop",
		"notes.stopped":   "Stopped",
		"note.stoppedmsg": "AI processing was stopped.",
```
zh-Hant:
```go
		"notes.stop":      "停止",
		"notes.stopped":   "已停止",
		"note.stoppedmsg": "AI 處理已停止。",
```
zh-Hans:
```go
		"notes.stop":      "停止",
		"notes.stopped":   "已停止",
		"note.stoppedmsg": "AI 处理已停止。",
```
ja:
```go
		"notes.stop":      "停止",
		"notes.stopped":   "停止しました",
		"note.stoppedmsg": "AI 処理を停止しました。",
```

Then run `gofmt -w internal/i18n/translations.go`.

- [ ] **Step 8: Add the Stop button + Stopped view to the template**

In `web/templates/note_view.html`, change the processing pill (lines 10-12) to add an inline Stop form for editors:

```html
        {{if eq .Note.Status "processing"}}<span id="status-pill" class="text-xs text-amber-600">● {{t .Lang "notes.processing"}}</span>
          {{if .CanEdit}}<form method="post" action="/notes/{{.Note.ID}}/stop" class="inline"><button class="text-xs border rounded px-2 py-0.5 text-neutral-600 hover:bg-neutral-50">■ {{t .Lang "notes.stop"}}</button></form>{{end}}{{end}}
        {{if eq .Note.Status "failed"}}<span id="status-pill" class="text-xs {{if .Note.Stopped}}text-neutral-500{{else}}text-red-600{{end}}">● {{if .Note.Stopped}}{{t .Lang "notes.stopped"}}{{else}}{{t .Lang "notes.failed"}}{{end}}</span>{{end}}
```

Replace the failed block (lines 31-40) so a Stopped note shows neutral copy instead of the red error:

```html
  {{if eq .Note.Status "failed"}}
  <div class="mt-3 {{if .Note.Stopped}}bg-neutral-50 border-neutral-200{{else}}bg-red-50 border-red-200{{end}} border rounded p-3">
    <p class="text-sm {{if .Note.Stopped}}text-neutral-700{{else}}text-red-700{{end}}">{{if .Note.Stopped}}{{t .Lang "note.stoppedmsg"}}{{else}}{{t .Lang "note.aifailed"}}{{if .Note.Error}}: {{.Note.Error}}{{end}}{{end}}</p>
    {{if .CanEdit}}
    <form method="post" action="/notes/{{.Note.ID}}/retry" class="mt-2">
      <button class="{{if .Note.Stopped}}bg-indigo-600{{else}}bg-red-600{{end}} text-white rounded px-3 py-1.5 text-sm">{{t .Lang "note.retry"}}</button>
    </form>
    {{end}}
  </div>
  {{end}}
```

- [ ] **Step 9: Build + parity + full server tests**

Run: `go build ./... && go test ./internal/i18n/ -run TestLanguageKeyParity && go test ./internal/server/`
Expected: clean build; parity PASS; server tests PASS.

- [ ] **Step 10: Checkpoint commit**

```bash
git add internal/server/notes.go internal/server/server.go internal/server/stop_test.go web/templates/note_view.html internal/i18n/translations.go
git commit -m "feat(notes): Stop button + Stopped state with Retry"
```

---

# Part 2 — Facebook Link Capture

## Task 4: `IsFacebook` + `validVTT` helpers

**Files:**
- Create: `internal/ingest/facebook.go`
- Modify: `internal/ingest/youtube.go` (use `validVTT` in `cleanVTT` callers — optional reuse; not required here)
- Test: `internal/ingest/facebook_test.go` (create)

**Interfaces:**
- Produces: `ingest.IsFacebook(url string) bool`; `ingest.validVTT(s string) bool`.

- [ ] **Step 1: Write the failing test**

Create `internal/ingest/facebook_test.go`:

```go
package ingest

import "testing"

func TestIsFacebook(t *testing.T) {
	yes := []string{
		"https://www.facebook.com/watch/?v=123",
		"https://facebook.com/someone/posts/456",
		"https://m.facebook.com/story.php?story_fbid=1",
		"https://fb.watch/abcdEFG/",
		"https://www.facebook.com/reel/789",
	}
	no := []string{
		"https://www.youtube.com/watch?v=abc",
		"https://example.com/facebook-clone",
		"https://notfacebook.example.com/x",
	}
	for _, u := range yes {
		if !IsFacebook(u) {
			t.Errorf("IsFacebook(%q) = false, want true", u)
		}
	}
	for _, u := range no {
		if IsFacebook(u) {
			t.Errorf("IsFacebook(%q) = true, want false", u)
		}
	}
}

func TestValidVTT(t *testing.T) {
	if !validVTT("WEBVTT\n\n00:00:01.000 --> 00:00:02.000\nhi") {
		t.Error("real VTT rejected")
	}
	if validVTT("Transcript\nG o o g l e\nSorry...\nWe're sorry... automated queries") {
		t.Error("anti-bot HTML accepted as VTT")
	}
}
```

- [ ] **Step 2: Run the test to verify it fails**

Run: `go test ./internal/ingest/ -run 'TestIsFacebook|TestValidVTT' -v`
Expected: FAIL — `IsFacebook`/`validVTT` undefined.

- [ ] **Step 3: Implement the helpers**

Create `internal/ingest/facebook.go` (helpers only for now; fetch added in Task 5):

```go
package ingest

import "strings"

// IsFacebook reports whether a URL points at Facebook content.
func IsFacebook(u string) bool {
	u = strings.ToLower(u)
	for _, h := range []string{"facebook.com/", "m.facebook.com/", "web.facebook.com/", "fb.com/", "fb.watch/"} {
		if strings.Contains(u, h) {
			return true
		}
	}
	return false
}

// validVTT reports whether s is a real WebVTT subtitle payload (starts with the
// WEBVTT signature). Guards against anti-bot HTML pages being stored as captions.
func validVTT(s string) bool {
	return strings.HasPrefix(strings.TrimSpace(s), "WEBVTT")
}
```

- [ ] **Step 4: Run the test to verify it passes**

Run: `go test ./internal/ingest/ -run 'TestIsFacebook|TestValidVTT' -v`
Expected: PASS

- [ ] **Step 5: Checkpoint commit**

```bash
git add internal/ingest/facebook.go internal/ingest/facebook_test.go
git commit -m "feat(ingest): IsFacebook + validVTT helpers"
```

---

## Task 5: `FetchFacebook` (yt-dlp + headless + OG fallback)

**Files:**
- Modify: `internal/ingest/facebook.go`
- Test: `internal/ingest/facebook_test.go` (add cookies-parse test)

**Interfaces:**
- Consumes: `IsFacebook`, `validVTT` (Task 4); existing `dumpMeta`-style yt-dlp use, `cleanVTT`, `composeYouTubeNote`-style composition, `FetchURL`, `fetchHeadless`, `Fetched`, `EnableHeadless`.
- Produces: `ingest.FetchFacebook(ctx context.Context, rawURL, cookies string, headlessOK bool) (*Fetched, error)`; `ingest.parseNetscapeCookies(s string) []*http.Cookie` (used by the headless path).

- [ ] **Step 1: Write the failing test (cookies parser)**

Add to `internal/ingest/facebook_test.go`:

```go
func TestParseNetscapeCookies(t *testing.T) {
	txt := "# Netscape HTTP Cookie File\n" +
		".facebook.com\tTRUE\t/\tTRUE\t0\tc_user\t123456\n" +
		".facebook.com\tTRUE\t/\tTRUE\t0\txs\tabcDEF\n"
	cs := parseNetscapeCookies(txt)
	if len(cs) != 2 {
		t.Fatalf("got %d cookies, want 2", len(cs))
	}
	if cs[0].Name != "c_user" || cs[0].Value != "123456" || cs[0].Domain != ".facebook.com" {
		t.Errorf("cookie[0] = %+v", cs[0])
	}
}
```

- [ ] **Step 2: Run the test to verify it fails**

Run: `go test ./internal/ingest/ -run TestParseNetscapeCookies -v`
Expected: FAIL — `parseNetscapeCookies` undefined.

- [ ] **Step 3: Implement `FetchFacebook`, cookies parsing, and the yt-dlp/headless paths**

Append to `internal/ingest/facebook.go`:

```go
import (
	"context"
	"encoding/json"
	"net/http"
	"os"
	"os/exec"
	"strconv"
	"strings"
)

// FetchFacebook captures a Facebook URL's content, richest path first:
//  1. yt-dlp + cookies (videos/reels/watch) -> title, description, transcript
//  2. authenticated headless render (text posts) when headlessOK
//  3. plain Open-Graph fallback (works without cookies)
func FetchFacebook(ctx context.Context, rawURL, cookies string, headlessOK bool) (*Fetched, error) {
	// Path 1: yt-dlp with cookies.
	if f := fetchFacebookYtdlp(ctx, rawURL, cookies); f != nil {
		return f, nil
	}
	// Path 2: authenticated headless.
	if headlessOK && EnableHeadless {
		if f := fetchFacebookHeadless(ctx, rawURL, cookies); f != nil {
			return f, nil
		}
	}
	// Path 3: Open-Graph fallback.
	if f, err := FetchURL(ctx, rawURL); err == nil {
		return f, nil
	}
	return nil, fmt.Errorf("facebook: no content extractable for %s", rawURL)
}

// fetchFacebookYtdlp uses yt-dlp (with optional cookies) to pull metadata and any
// caption track. Returns nil if yt-dlp is unavailable or yields nothing.
func fetchFacebookYtdlp(ctx context.Context, rawURL, cookies string) *Fetched {
	if _, err := exec.LookPath("yt-dlp"); err != nil {
		return nil
	}
	cookieFile, cleanup := writeCookieFile(cookies)
	defer cleanup()

	args := []string{"--no-warnings", "--dump-json"}
	if cookieFile != "" {
		args = append(args, "--cookies", cookieFile)
	}
	args = append(args, rawURL)
	out, err := exec.CommandContext(ctx, "yt-dlp", args...).Output()
	if err != nil || len(out) == 0 {
		return nil
	}
	var m ytMeta
	if json.Unmarshal(out, &m) != nil {
		return nil
	}
	transcript := facebookTranscript(ctx, rawURL, cookieFile, m)
	body := composeYouTubeNote(m, transcript) // same metadata+description+transcript shape
	if strings.TrimSpace(body) == "" {
		return nil
	}
	return &Fetched{Title: strings.TrimSpace(m.Title), Text: body}
}

// facebookTranscript downloads the chosen caption track via yt-dlp and returns the
// cleaned text, only if it is a real VTT (rejects anti-bot HTML).
func facebookTranscript(ctx context.Context, rawURL, cookieFile string, m ytMeta) string {
	lang := pickCaptionLang(m)
	if lang == "" {
		return ""
	}
	args := []string{"--no-warnings", "--skip-download", "--write-auto-subs", "--write-subs",
		"--sub-langs", lang, "--sub-format", "vtt", "-o", "-"}
	if cookieFile != "" {
		args = append(args, "--cookies", cookieFile)
	}
	args = append(args, rawURL)
	out, err := exec.CommandContext(ctx, "yt-dlp", args...).Output()
	if err != nil || !validVTT(string(out)) {
		return ""
	}
	return cleanVTT(string(out))
}

// fetchFacebookHeadless renders the URL in headless Chrome with the supplied
// cookies injected, then extracts text. Returns nil on any failure.
func fetchFacebookHeadless(ctx context.Context, rawURL, cookies string) *Fetched {
	cs := parseNetscapeCookies(cookies)
	return fetchHeadlessWithCookies(ctx, rawURL, cs)
}

// writeCookieFile materializes cookies (Netscape format) to a temp file for
// yt-dlp's --cookies. Returns "" + a no-op cleanup when cookies is empty.
func writeCookieFile(cookies string) (path string, cleanup func()) {
	if strings.TrimSpace(cookies) == "" {
		return "", func() {}
	}
	f, err := os.CreateTemp("", "fbcookies-*.txt")
	if err != nil {
		return "", func() {}
	}
	_, _ = f.WriteString(cookies)
	_ = f.Close()
	return f.Name(), func() { _ = os.Remove(f.Name()) }
}

// parseNetscapeCookies parses a Netscape cookies.txt body into http.Cookies.
func parseNetscapeCookies(s string) []*http.Cookie {
	var out []*http.Cookie
	for _, line := range strings.Split(s, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		f := strings.Split(line, "\t")
		if len(f) < 7 {
			continue
		}
		out = append(out, &http.Cookie{Domain: f[0], Path: f[2], Name: f[5], Value: f[6]})
	}
	return out
}
```

Add the missing `"fmt"` import to the file's import block. `strconv` is imported for future use; remove it if unused to satisfy the compiler.

- [ ] **Step 4: Add the cookie-aware headless renderer**

In `internal/ingest/headless.go`, add (reuses the existing chromedp setup):

```go
import (
	"github.com/chromedp/cdproto/network"
	"net/http"
)

// fetchHeadlessWithCookies renders rawURL in headless Chrome after setting the
// given cookies (for authenticated pages). Returns nil on any failure.
func fetchHeadlessWithCookies(ctx context.Context, rawURL string, cookies []*http.Cookie) *Fetched {
	actx, acancel := chromedp.NewContext(ctx)
	defer acancel()
	tctx, tcancel := context.WithTimeout(actx, headlessNavTimeout)
	defer tcancel()

	setCookies := chromedp.ActionFunc(func(ctx context.Context) error {
		for _, c := range cookies {
			if err := network.SetCookie(c.Name, c.Value).WithDomain(c.Domain).WithPath(c.Path).Do(ctx); err != nil {
				return err
			}
		}
		return nil
	})

	var htmlContent string
	err := chromedp.Run(tctx,
		network.Enable(),
		setCookies,
		chromedp.Navigate(rawURL),
		chromedp.Sleep(HeadlessWait),
		chromedp.OuterHTML("html", &htmlContent, chromedp.ByQuery),
	)
	if err != nil || htmlContent == "" {
		log.Printf("ingest: authenticated headless render of %s failed: %v", rawURL, err)
		return nil
	}
	return buildFetched([]byte(htmlContent))
}
```

- [ ] **Step 5: Resolve the chromedp cdproto dependency**

Run: `cd /c/Project/Note-Aura && GOFLAGS=-mod=mod go build ./... 2>&1 | head`
Expected: clean (the `github.com/chromedp/cdproto/network` package ships with the already-present chromedp module). If `go.sum` needs it: `GOFLAGS=-mod=mod go mod tidy`.

- [ ] **Step 6: Run the ingest tests**

Run: `go test ./internal/ingest/ -v`
Expected: PASS (helpers + cookies parser; network-dependent fetch paths are not unit-tested here).

- [ ] **Step 7: Checkpoint commit**

```bash
git add internal/ingest/facebook.go internal/ingest/headless.go internal/ingest/facebook_test.go go.mod go.sum
git commit -m "feat(ingest): authenticated FetchFacebook (yt-dlp + headless + OG fallback)"
```

---

## Task 6: Admin `facebook.cookies` setting

**Files:**
- Modify: `internal/server/admin.go` (`getAdmin` ~line 152; `postAdmin` `set(...)` block ~line 255-263)
- Modify: `web/templates/admin.html` (add a textarea in the AI-settings form)

**Interfaces:**
- Consumes: `(*DB) GetAppSettings`, `SetAppSetting` (existing).
- Produces: app setting key `"facebook.cookies"`, template var `FacebookCookies`.

- [ ] **Step 1: Surface the stored value in `getAdmin`**

In `internal/server/admin.go`, in `getAdmin`, near the model assignments (after line 152) add:

```go
	m["FacebookCookies"] = app["facebook.cookies"]
```

- [ ] **Step 2: Persist it in `postAdmin`**

In `internal/server/admin.go`, in the `set(k, v)` block (~line 255-263), add:

```go
	set("facebook.cookies", strings.TrimSpace(c.FormValue("facebook_cookies")))
```

(`strings` is already imported in this file.)

- [ ] **Step 3: Add the textarea to the admin form**

In `web/templates/admin.html`, inside the AI-settings `<form>` (near the model fields), add:

```html
  <label class="block mt-4 text-sm font-medium">Facebook cookies (Netscape cookies.txt)</label>
  <p class="text-xs text-neutral-500 mb-1">Paste an exported cookies.txt from a logged-in Facebook session to capture fuller post/video content. Leave blank to use public Open-Graph previews only. Stored in the database.</p>
  <textarea name="facebook_cookies" rows="4" class="w-full border rounded px-2 py-1 text-xs font-mono">{{.FacebookCookies}}</textarea>
```

- [ ] **Step 4: Build**

Run: `go build ./...`
Expected: clean.

- [ ] **Step 5: Checkpoint commit**

```bash
git add internal/server/admin.go web/templates/admin.html
git commit -m "feat(admin): Facebook cookies setting for authenticated capture"
```

---

## Task 7: Worker materialize branch + Facebook badge

**Files:**
- Modify: `internal/worker/worker.go` (`materialize` `case "url":` ~lines 288-293)
- Modify: `web/templates/note_view.html` (source badge line 9)

**Interfaces:**
- Consumes: `ingest.IsFacebook`, `ingest.FetchFacebook` (Tasks 4-5); `(*DB) GetAppSettings`; `ingest.EnableHeadless`.

- [ ] **Step 1: Branch the url case to Facebook**

In `internal/worker/worker.go`, replace the `case "url":` block in `materialize` (lines 288-293):

```go
	case "url":
		if ingest.IsFacebook(note.SourceRef) {
			cookies := ""
			if app, err := w.db.GetAppSettings(); err == nil {
				cookies = app["facebook.cookies"]
			}
			f, err := ingest.FetchFacebook(ctx, note.SourceRef, cookies, ingest.EnableHeadless)
			if err != nil {
				return "", "", "", err
			}
			return f.Text, f.Text, f.Title, nil
		}
		f, err := ingest.FetchURL(ctx, note.SourceRef)
		if err != nil {
			return "", "", "", err
		}
		return f.Text, f.Text, f.Title, nil
```

- [ ] **Step 2: Show a Facebook badge in the note view**

In `web/templates/note_view.html`, replace the source badge (line 9):

```html
        <span class="text-xs px-1.5 py-0.5 rounded bg-neutral-100 text-neutral-500 uppercase">{{if and (eq .Note.SourceType "url") (isFacebook .Note.SourceRef)}}facebook{{else}}{{.Note.SourceType}}{{end}}</span>
```

Register the `isFacebook` template helper next to the existing `engine.AddFunc` calls in `internal/server/server.go` (lines 79-81, near `countryName`/`langs`):

```go
	engine.AddFunc("isFacebook", ingest.IsFacebook)
```

(`internal/server/server.go` already imports `note-aura/internal/ingest`; confirm and add it if not.)

- [ ] **Step 3: Build + full tests**

Run: `go build ./... && go test ./...`
Expected: clean build; all tests pass.

- [ ] **Step 4: Checkpoint commit**

```bash
git add internal/worker/worker.go web/templates/note_view.html internal/server/
git commit -m "feat(capture): route Facebook URLs to authenticated extraction + badge"
```

---

## Task 8: Manual smoke test + documentation

**Files:** `README.md`, `INSTALL.md`, `USER_GUIDE.md`, `USER_GUIDE.zh-Hant.md`, `CHANGELOG.md` (no code).

- [ ] **Step 1: Manual smoke — Stop**

Build and run on a throwaway config (port 8099, temp db/uploads, dead Ollama, as in the prior i18n smoke test). Capture a URL note (AI will hang on the dead Ollama → stays `processing`), open it, click **Stop**. Expected: pill flips to **Stopped**, neutral box with **Retry**; DB shows `status='failed', stopped=1` and no rows in `jobs` for that note. Click **Retry** → returns to `processing`, `stopped=0`.

- [ ] **Step 2: Manual smoke — Facebook (no cookies)**

Paste a public Facebook post URL. Expected: note captures at least the Open-Graph title + caption (fallback path), badge shows `facebook`, and the AI pipeline fills title/summary/tags/category (or offers Retry if AI is down). With cookies configured in Admin and headless enabled, richer text/transcript is captured.

- [ ] **Step 3: Update README.md**

Add to the Features list: Facebook link capture (authenticated via admin cookies; public links get the preview) and a Stop button to cancel AI processing.

- [ ] **Step 4: Update INSTALL.md**

Document the **Facebook cookies** admin setting (export a `cookies.txt` from a logged-in browser session; paste into Admin → AI settings), note the headless requirement for full text posts, and that it's optional (public links still get previews).

- [ ] **Step 5: Update USER_GUIDE.md + USER_GUIDE.zh-Hant.md**

In the capture section: pasting a Facebook link captures its content like a web link/YouTube. In the note section: while a note is processing, a **Stop** button cancels AI; the note becomes **Stopped** and can be retried. Mirror both additions in the zh-Hant guide.

- [ ] **Step 6: Update CHANGELOG.md**

Add a dated `2026-06-17` entry (newest on top) summarizing both features.

- [ ] **Step 7: Final verification**

Run: `go build ./... && go vet ./... && go test ./...`
Expected: clean build, clean vet, all tests pass.

- [ ] **Step 8: Checkpoint commit**

```bash
git add README.md INSTALL.md USER_GUIDE.md USER_GUIDE.zh-Hant.md CHANGELOG.md
git commit -m "docs: document Facebook capture and Stop button"
```

---

## Self-Review (completed by plan author)

- **Spec coverage:** Feature A → Tasks 4-7 (detect, fetch, cookies, worker branch, badge). Feature B → Tasks 1-3 (column/methods, cancel registry, handler/UI). Tests → each task. Docs/CHANGELOG → Task 8. No CHECK rebuilds (Task 1 column + Feature-A url-branch). i18n parity preserved (Task 3 Step 7).
- **Signature consistency:** `StopNote`/`ClearStopped`/`DeleteJobsForNote` (Task 1) consumed in Task 3. `Worker.Cancel`/`register`/`deregister` (Task 2) consumed in Task 3. `IsFacebook`/`validVTT` (Task 4) + `FetchFacebook`/`parseNetscapeCookies` (Task 5) consumed in Task 7. `fetchHeadlessWithCookies` (Task 5) used by `fetchFacebookHeadless`. `Note.Stopped` defined Task 1, read in Task 3 template.
- **Placeholder scan:** none — every code step shows the code; `strconv` caveat called out in Task 5 Step 3.
- **Verified integration point:** the template helper registration is `engine.AddFunc(...)` in `internal/server/server.go:79-81` (confirmed), so Task 7 Step 2 adds `engine.AddFunc("isFacebook", ingest.IsFacebook)` there.
