# Task 8 Report — Smoke Test + Documentation

**Date:** 2026-06-17  
**Task:** Manual smoke test, CHANGELOG consolidation, docs update for Facebook capture + Stop button.

---

## 1. CHANGELOG Consolidation

**Before:** `## 2026-06-17` contained two separate entries:
- "Admin: Facebook cookies setting." — admin-centric, implementation-altitude description mentioning Task 7, worker internals, etc.
- "Stop button for AI processing." — user-facing but mentioned internal flag/status details.

**After:** Two clean user-facing bullet points under `## 2026-06-17 / ### Added`:
1. **Facebook link capture** — covers all three fetch paths (yt-dlp+cookies, headless, OG fallback), the `facebook` badge, and the admin cookies setting in plain language.
2. **Stop button for AI processing** — covers the Stop button, Stopped state (neutral vs red error), Retry flow, and i18n coverage.

The pre-existing entries (i18n, per-account language memory, localized emails, `user_invitations.lang` column, docs note, verification line) are preserved intact below the new entries.

---

## 2. Docs Updated

### README.md
**Changed:** "Web-link & YouTube ingest" bullet split and expanded to cover Facebook (same bullet), plus a new "Stop button" bullet added to the Features list.

**Added text:**
- Facebook posts/videos/reels supported; admins can supply Netscape cookies.txt for authenticated extraction; public links get Open-Graph preview without cookies.
- Stop button bullet: cancel in-progress AI, note lands in "Stopped" state (distinct from "Failed"), can retry when ready.

### INSTALL.md
**Changed:** TOC updated to add `7c. Optional: Facebook capture with cookies` and renumber former 7c (Email→note) to `7d`.

**New section `## 7c`:** Explains:
- No cookies needed for public links (OG fallback always works).
- How to export Netscape cookies.txt from a logged-in browser session (using a browser extension).
- How to paste into Admin → AI settings.
- Two richer fetch paths: yt-dlp+cookies (video/reels), authenticated headless (text posts) — each linking to their respective prerequisites sections (§7 and §7b).
- Cookie expiry note (re-export when authenticated capture stops working).

### USER_GUIDE.md
**Section 3 (Adding notes):** "Add from a link (web or YouTube)" heading updated to "(web, YouTube, or Facebook)"; new paragraph added explaining Facebook link capture, the `facebook` badge, cookies vs. OG fallback.

**Section 4 (How AI processing works):** New "Stopping AI processing" sub-section added before "If a note shows failed": explains the Stop button, the Stopped state (neutral box, indigo Retry), and how to re-run.

**Section 13 (Admin guide / AI settings):** New bullet added for "Facebook cookies (Netscape cookies.txt)" admin setting.

**Section 11 (Email → note):** Updated INSTALL.md cross-reference from §7c to §7d.

**Footer:** Updated last-updated date from 2026-06-13 to 2026-06-17.

### USER_GUIDE.zh-Hant.md
Mirrored all four changes from USER_GUIDE.md in Traditional Chinese:
- Section 3: "從連結新增(網頁或 YouTube)" → "(網頁、YouTube 或 Facebook)"; Facebook explanation paragraph added.
- Section 4: New "停止 AI 處理" sub-section added before the "失敗" section.
- Section 13 (AI 設定): New Facebook cookies bullet in Chinese.
- Section 11: §7c → §7d cross-reference updated.
- Footer: 2026-06-13 → 2026-06-17.

---

## 3. Smoke Test

### Setup
- Binary: `note-aura-smoke.exe` built from `C:\Project\Note-Aura\go build -o note-aura-smoke.exe .`
- Config: `C:\tmp\smoke-config.yaml` — port 8099, DB at `C:\tmp\note-aura-smoke\db\smoke.db`, uploads at `C:\tmp\note-aura-smoke\uploads`, `ai.ollama_url: "http://127.0.0.1:1"` (dead Ollama), no SMTP, `initial_user: admin@smoke.test / smokepass123`.
- Server started successfully (PID 16708 → 5736 after a restart), confirmed seeded admin.

### Stop Flow (Step 1)
All steps confirmed against `C:\tmp\note-aura-smoke\db\smoke.db` using a Go query tool (`modernc.org/sqlite`):

1. Created a URL note via `POST /capture` (source: `https://example.com/test-page`) → note ID 1, location `/notes/1`.
2. Note initially went to `failed` (dead Ollama, 5-second timeout). Retried via `POST /notes/1/retry`, then immediately `POST /notes/1/stop`.
3. **DB after stop:**
   ```
   notes: id=1 | status=failed | stopped=1 | error='stopped by user'
   jobs:  COUNT(*) WHERE note_id=1 = 0
   ```
   All jobs deleted, note flagged stopped.
4. **Retry after stop:** `POST /notes/1/retry` → DB shows `stopped=0` (ClearStopped called). Note returned to processing, then failed again on the dead Ollama.
5. **UI verification:** Stopped state page shows:
   - `<span id="status-pill" class="text-xs text-neutral-500">● Stopped</span>` (neutral, not red)
   - `<div class="mt-3 bg-neutral-50 border-neutral-200 border rounded p-3">AI processing was stopped.</div>`
   - `<button class="bg-indigo-600 text-white ...">↻ Retry AI processing</button>`

### Facebook Routing (Step 2)
No live Facebook fetch attempted (network access not reliable in this environment). Unit tests cover:
- `TestIsFacebook` — all positive and negative URL patterns pass (`internal/ingest`).
- `TestValidVTT` — real VTT accepted, anti-bot HTML rejected.
- `TestParseNetscapeCookies` — 2-cookie Netscape file parsed correctly.
- Worker branch (`ingest.IsFacebook` → `ingest.FetchFacebook`) and badge template (`isFacebook` helper) covered by build and server tests.
This is noted as a network-limitation, not a feature failure.

### Real DB Untouched
```
C:\Project\Note-Aura\data\note-aura.db
Modify: 2026-06-10 08:41:51 +0100
```
Timestamp unchanged throughout the smoke test.

### Cleanup
- `note-aura-smoke.exe` deleted: confirmed absent.
- `C:\tmp\note-aura-smoke\` deleted: confirmed absent.
- Smoke config `C:\tmp\smoke-config.yaml` and temp query files in `C:\tmp\` left (not project files).

---

## 4. Final Build / Vet / Test Output

```
cd /c/Project/Note-Aura
go build ./...   → BUILD OK (no output, exit 0)
go vet ./...     → VET OK (no output, exit 0)
go test ./...    →
  ok   note-aura/internal/ai
  ok   note-aura/internal/db
  ok   note-aura/internal/emailin
  ok   note-aura/internal/i18n
  ok   note-aura/internal/ingest
  ok   note-aura/internal/markdown
  ok   note-aura/internal/reminder
  ok   note-aura/internal/server
  ok   note-aura/internal/worker
  (no test files for: note-aura, cmd/reset, auth, config, holidays, mailer, rag, syslog)
```

All packages build clean, vet clean, tests pass.

---

## Fix: stopped-note resurrection race

**Date:** 2026-06-17

### Lines Guarded in `internal/worker/worker.go` (`run()`)

Six `if err := ctx.Err(); err != nil { return err }` guards were inserted, one immediately before each unconditional DB persistence write that could overwrite the `stopped=1 / status=failed` state set by the Stop handler:

| Location (post-edit line) | Write guarded | Why |
|---|---|---|
| Before `w.db.ApplyAIResult` in the `!doAI` branch (~line 188) | `ApplyAIResult` (no-AI path) | `materialize()` calls provider and is cancelable; Stop can fire after it returns but before this write |
| Before first `w.db.ApplyAIResult` in the doAI path (~line 221) | `ApplyAIResult` (title + body early save) | After `provider.Title(ctx, ...)` returns; window exists between title resolution and the early-save write |
| Before second `w.db.ApplyAIResult` (~line 237) | `ApplyAIResult` (with summary) | After `provider.Summarize(ctx, ...)` returns; same race window |
| Before `w.db.SetNoteTags` inside `want["tags"]` block (~line 248) | `SetNoteTags` | After `provider.Tags(ctx, ...)` returns |
| Before `w.db.SetNoteCategory` inside `want["category"]` block (~line 268) | `SetNoteCategory` | After `provider.Category(ctx, ...)` returns; the check is inside the success branch, keeping it tight |
| Before `w.db.ReplaceChunks` inside the embed block (~line 296) | `ReplaceChunks` | After `provider.Embed(ctx, ...)` returns |

`IncrementOllamaUsage` (after `ReplaceChunks`) was intentionally left unguarded: it is a quota counter, not a note-state write, and does not touch `status` or `stopped`; writing it even on a cancelled context is harmless and idiomatic.

### Build / Vet / Test Output

```
cd /c/Project/Note-Aura
go build ./... && go vet ./... && go test ./internal/worker/ ./internal/server/ ./internal/db/ 2>&1 | tail -20

ok  	note-aura/internal/worker	0.636s
ok  	note-aura/internal/server	(cached)
ok  	note-aura/internal/db	(cached)
```

Clean build, clean vet, all targeted tests pass.
