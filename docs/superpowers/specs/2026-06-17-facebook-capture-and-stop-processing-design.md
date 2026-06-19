# Facebook Link Capture + Stop-Processing Button — Design

**Date:** 2026-06-17
**Status:** Draft for review
**Working tree:** `C:\Project\Note-Aura` — **not a git repo** ("commit" steps are no-ops; the
checkpoint after each task is a green `go build ./...` + the task's tests).

## Goal

Two independent features for the capture/processing flow:

1. **Facebook link capture.** Besides YouTube, a pasted Facebook link is fetched into the
   note body (authenticated, using admin-supplied cookies for fuller text), then the normal
   AI pipeline generates title, category, tags, and summary.
2. **Stop button during AI processing.** While a note is in `processing`, the owner can press
   **Stop** to cancel the in-flight AI work. The note lands in a distinct **Stopped** state
   (reusing the failed-note Retry path) rather than the red "failed" error.

## Locked decisions (from brainstorming)

- **Facebook depth:** *authenticated for fuller text.* The admin supplies Facebook cookies; we
  use them with yt-dlp (videos/reels) and an authenticated headless render (text posts), with a
  no-cookie Open-Graph fallback.
- **Stop result:** *distinct "Stopped" state* — reuse `status='failed'` plus a `stopped` flag so
  the UI shows a neutral "Stopped" with Retry, and the existing Retry pathway re-runs it.

## Key constraint driving the design

`notes.source_type` and `notes.status` both have **SQLite CHECK constraints**
(`source_type IN ('manual','url','youtube','image')`, `status IN ('processing','ready','failed')`).
SQLite cannot `ALTER` a CHECK; widening one needs a full table rebuild on a live DB. To avoid
that risk we **add no new enum values**:

- Facebook stays `source_type='url'`; we branch on URL content (`IsFacebook`) instead.
- Stop reuses `status='failed'` plus a new boolean column `stopped` (added via idempotent
  `ALTER TABLE ADD COLUMN`, which SQLite allows).

---

## Feature A — Facebook link capture

### Data flow

Capture (unchanged) → note saved as `source_type='url'`, `source_ref=<fb url>` → worker
`materialize()` detects `IsFacebook(source_ref)` → `ingest.FetchFacebook(...)` returns
`{Title, Text}` → normal AI pipeline (title/summary/tags/category) runs on that text.

**No change to `capture.go` / `emailnote.go`** — Facebook URLs already route to `source_type='url'`
today; the new behavior lives entirely in the worker's materialize branch + a new ingest unit.

### Components

1. **`ingest.IsFacebook(url) bool`** (in `internal/ingest/facebook.go`)
   Matches `facebook.com`, `m.facebook.com`, `web.facebook.com`, `fb.com`, `fb.watch`, and FB
   paths (`/reel/`, `/watch`, `/videos/`, `/share/`, `/posts/`, `/story.php`). Case-insensitive.

2. **`ingest.FetchFacebook(ctx, url, cookies string, headlessOK bool) (*Fetched, error)`**
   Tries extractors in order of richness; first non-empty result wins:
   - **A. yt-dlp + cookies** (video/reel/watch URLs): write `cookies` to a temp Netscape
     `cookies.txt`, run `yt-dlp --no-warnings --cookies <file> --dump-json <url>` → title,
     description, uploader, upload_date; if a subtitle/auto-caption track exists, fetch + clean it
     (reusing `cleanVTT`). This spec also introduces a small `validVTT(s) bool` helper — requires the
     `WEBVTT` signature — used here to reject non-subtitle payloads; it is independent of (but shared
     with) the separate YouTube anti-bot fix. Compose a Markdown note like `composeYouTubeNote`
     (metadata header + description + transcript).
   - **B. authenticated headless** (text posts, or when A yields no body): when `headlessOK`
     (i.e. `EnableHeadless` + Chrome present), parse `cookies` into `network.SetCookies`, navigate
     with chromedp, grab `OuterHTML`, run through the existing `buildFetched`.
   - **C. fallback:** plain `FetchURL(ctx, url)` → Open-Graph `og:title`/`og:description`. Works
     with no cookies, so a public link always yields at least its preview.
   - Returns the richest `Fetched`; error only if all paths yield nothing.

3. **Cookies configuration** — admin setting `facebook.cookies` (Netscape `cookies.txt` text)
   stored in `app_settings`:
   - `internal/server/admin.go`: read in `getAdmin`, save in `postAdmin`
     (`set("facebook.cookies", c.FormValue("facebook_cookies"))`).
   - `web/templates/admin.html`: a `<textarea name="facebook_cookies">` with a short help line.
   - ⚠️ Stored **plaintext in the DB** — documented as a self-hosted trade-off.

4. **Worker wiring** — in `materialize`, `case "url":` first checks `IsFacebook(note.SourceRef)`;
   if so, loads `facebook.cookies` from app settings and calls `FetchFacebook(ctx, url, cookies,
   ingest.EnableHeadless)`. Otherwise the existing `FetchURL` path runs unchanged.

5. **UI badge (nicety):** `note_view.html` shows a `facebook` badge (derived from
   `IsFacebook(SourceRef)`) instead of `url`. No stored-enum change.

### Out of scope (YAGNI)

- Per-user Facebook cookies (global admin cookies only for now).
- A Facebook-specific AI prompt (reuses the existing web prompt).

---

## Feature B — Stop button during AI processing

### Data flow

User clicks **Stop** on a processing note → `POST /notes/:id/stop` → handler (a) deletes the
note's jobs so nothing requeues, (b) cancels the running AI context via the worker, (c) sets the
note to the Stopped state. The existing 2.5s status poller sees `status != processing` and reloads
into the Stopped view, which offers **Retry**.

### Components

1. **DB**
   - Schema: add `stopped INTEGER NOT NULL DEFAULT 0` to the `notes` table const **and** an
     idempotent migration `ALTER TABLE notes ADD COLUMN stopped INTEGER NOT NULL DEFAULT 0`.
   - `Note` struct gains `Stopped bool`; all `SELECT`/scan sites that build a full `Note` include it.
   - `StopNote(noteID)`: `UPDATE notes SET status='failed', stopped=1, error='stopped by user',
     updated_at=… WHERE id=?`.
   - `DeleteJobsForNote(noteID)`: `DELETE FROM jobs WHERE note_id=?` (removes queued + running rows
     so `FailJob` can't requeue a stopped note).
   - Retry clears the flag: `retryNote` sets `stopped=0` alongside `status='processing'` before
     re-enqueuing.

2. **Worker cancellation registry** (`internal/worker/worker.go`)
   - Add `cancels map[int64]context.CancelFunc` + `sync.Mutex`.
   - `process(job)`: register `cancel` keyed by `job.NoteID` before `run`; `defer` deregister.
   - `Cancel(noteID int64)`: look up and invoke the cancel func (no-op if absent).
   - On cancellation, `run` returns `ctx.Err()`; `process` detects `errors.Is(err,
     context.Canceled)` and returns **without** `FailJob`/`SetNoteStatus` — the stop handler already
     set the final state, and the job row is already deleted.

3. **Server**
   - `POST /notes/:id/stop` → `stopNote`: authz (owner/can-edit), only acts when the note is
     `processing`; calls `DeleteJobsForNote`, `worker.Cancel(id)`, `StopNote(id)`; redirects back.
   - Route registered next to the existing `/notes/:id/retry`.

4. **Template** (`web/templates/note_view.html`)
   - Processing pill area: a **Stop** button (`POST /notes/:id/stop`) for owner/editor.
   - Stopped rendering: when `status=='failed'` **and** `.Note.Stopped`, show a neutral "Stopped"
     pill + message + the existing **Retry** form (not the red error box).

5. **i18n** — new keys in all four languages (enforced by `TestLanguageKeyParity`):
   `notes.stop` ("Stop"), `notes.stopped` ("Stopped"), `note.stoppedmsg`
   ("AI processing was stopped."), reusing `note.retry`.

---

## Error handling

- **Facebook:** each extractor is best-effort; failures fall through to the next, and a total
  failure surfaces as the note's normal `failed` + Retry (same as a dead URL today). Missing/expired
  cookies simply degrade to the Open-Graph fallback.
- **Stop race:** if Stop lands after the worker already finished, `Cancel` is a no-op and
  `DeleteJobsForNote` removes the (done) row; `StopNote` still marks it Stopped — acceptable.
- **Temp cookies file:** written under the OS temp dir and removed via `defer`.

## Testing

- `ingest`: `IsFacebook` truth table; Netscape-cookies → chromedp-cookies parse; `validVTT`
  signature guard (shared with the YouTube path).
- `db`: `stopped` column round-trip; `StopNote` sets status/stopped/error; `retryNote` clears
  `stopped`; `DeleteJobsForNote` removes rows.
- `worker`: cancel registry register/cancel/deregister.
- `server`: `stopNote` transitions a processing note → Stopped and deletes its jobs.
- `i18n`: `TestLanguageKeyParity` stays green.

## Documentation (standing rule)

- `README.md`: Facebook capture + Stop button bullets.
- `INSTALL.md`: `facebook.cookies` admin setting + how to export cookies; headless requirement.
- `USER_GUIDE.md` + `USER_GUIDE.zh-Hant.md`: capturing Facebook links, the cookies setting, using Stop.
- `CHANGELOG.md`: dated entry for both features.

## Units summary

`ingest` (FB detect/fetch + shared VTT guard) · `db` (`stopped` column, job deletion, StopNote) ·
`worker` (cancel registry, FB materialize branch) · `server` (stop handler, admin cookies) ·
`templates`/`i18n` (Stop UI, FB badge, keys).
