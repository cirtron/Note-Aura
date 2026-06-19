# Note-Aura

A lean, AI-first **knowledge inbox**. Capture anything — typed notes, web links,
YouTube videos, images, files, or **email** — and AI titles, summarizes, tags, and
indexes it for you. Retrieve by keyword search or by **asking your notes** (RAG
with citations).

This is a ground-up rebuild of an earlier PHP/MySQL Note-Aura, re-platformed onto
Go + Fiber + SQLite to match the rest of the stack.

## Features

- **Capture → process → retrieve loop.** Saving is instant; a background worker
  pool runs the slow AI work and flips the note from `processing` to `ready`.
  Content is stored even if AI is unavailable (the note offers **Retry**).
- **Auto-organize on capture** — AI generates a title, summary, tags, and a
  **category** (only for fields you leave empty), in the summary language you pick
  (or auto-matched to the content).
- **Web-link, YouTube & Facebook ingest** — paste any link; the page text or
  video transcript is fetched and summarized. Captures the **full** page text
  (incl. JSON-LD article bodies), with an optional **headless-browser** fallback
  for JavaScript-rendered sites. Facebook posts, videos, and reels are
  supported — admins can supply a Netscape `cookies.txt` (Admin → AI settings)
  for authenticated extraction; public links get the Open-Graph preview without
  cookies.
- **Stop button** — cancel a note's in-progress AI processing at any time. The
  note lands in a neutral "Stopped" state (distinct from the error "Failed" state)
  and can be retried when ready.
- **Image OCR** — drop a photo/screenshot; text is extracted (vision model).
- **File upload** — attach files (per-role allowed types); text files become the
  note body, others are stored as downloadable attachments.
- **Email → note** — each user gets a private plus-addressed inbox address; mail
  sent there becomes a note (a link-only email fetches the linked page).
- **Ask your notes (RAG)** — embeddings-based semantic retrieval feeds a chat
  model that answers with citations back to your notes.
- **Multi-user** with email/password accounts, **email verification**, and
  **invitations**; **per-note sharing** (read-only or can-edit) and **groups**
  (with co-admins and read/write control).
- **Calendar** — notes can have an event date, start/end time, and all-day flag;
  a month view + per-day agenda shows them. Optional **email reminders** and
  **public holidays** per country.
- **Markdown editor** (EasyMDE) with live preview + inline images; bodies render
  via goldmark and are sanitized with bluemonday.
- **Organize & browse** — hierarchical **categories** (`Parent/Child` sub-categories),
  tags, keyword search, **sort** (modified / created / title / category / tag /
  source), and **pagination** with a selectable page size. Created & modified
  timestamps on each note.
- **Notes management** — multi-select **bulk delete**, and **import/export** all
  your notes to/from a JSON file (in Settings).
- **Roles & quotas** — per-role storage, AI access & daily limits, group/invite
  limits, and allowed upload types; per-user overrides.
- **Pluggable AI backend** — local **Ollama** by default; override per user with
  any OpenAI-compatible endpoint (OpenAI, Gemini compat, OpenRouter, …). Users on
  their own external server can customize their own prompts.
- **Admin dashboard** — usage per user, recent notes, server monitor; manage
  users (suspend/delete), roles, branding, models/prompts (incl. web/YouTube-
  specific and category prompts), holidays, registration, email, and an
  **HTTPS on/off toggle**.
- **Multi-language UI** — English / 繁體中文 / 简体中文 / 日本語 (follows the
  browser by default; switch via the header selector and the choice is remembered
  on your account). Transactional emails (verification, password reset,
  invitations) are sent in the relevant person's language — invitations in the
  language the inviter picks.

## Stack

Go + Fiber, SQLite (`modernc.org/sqlite`, pure Go — no CGO), FTS5 for keyword
search, in-Go cosine similarity for vector search, server-rendered
`html/template` views.

## Running

1. Install [Ollama](https://ollama.com) and pull the default models (or set an
   OpenAI-compatible backend later in Settings):
   ```
   ollama pull llama3.1
   ollama pull nomic-embed-text
   ollama pull deepseek-ocr
   ```
2. Copy and edit config:
   ```
   cp config.example.yaml config.yaml
   ```
3. Build and run:
   ```
   go build -o note-aura.exe .
   ./note-aura.exe
   ```
4. Open http://localhost:8090 and register an account (the first account becomes
   the admin).

**Maintenance helpers** (run in the project folder):

```powershell
.\update.ps1     # Windows: update to new code, keep all data (stop → rebuild → restart)
.\reset.exe      # wipe ALL data back to a brand-new system (build: go build -o reset.exe ./cmd/reset/)
```

```bash
./update.sh      # Linux/macOS: same stop → rebuild → restart (auto-detects systemd)
```

`update.ps1` / `update.sh` never touch your database or uploads; `reset.exe`
permanently deletes them (stop the server first). See [INSTALL.md](INSTALL.md#9-backup-upgrade--reset).

To serve over **HTTPS**, point Note-Aura at your cert + key (PEM) in `config.yaml`
and set `session.secure: true` + an `https://` `base_url`:

```yaml
tls:
  cert_file: "certs/note-aura.crt"   # use fullchain if you have intermediates
  key_file:  "certs/note-aura.key"   # unencrypted PEM key
```

> **Optional integrations:** YouTube ingest needs [`yt-dlp`](https://github.com/yt-dlp/yt-dlp)
> on `PATH`; the headless web-link fallback needs Chrome/Chromium; Email→note,
> reminders, verification, and invitations need SMTP/IMAP configured. See
> [INSTALL.md](INSTALL.md).

See **[INSTALL.md](INSTALL.md)** for full setup and **[USER_GUIDE.md](USER_GUIDE.md)**
(English) / **[USER_GUIDE.zh-Hant.md](USER_GUIDE.zh-Hant.md)** (繁體中文) for usage.

## Layout

```
main.go                 wiring: config, db, worker pool, server, email poller
internal/
  config/   db/         YAML config; SQLite schema + queries (FTS sync)
  auth/                 bcrypt + session tokens
  ai/                   Provider interface; Ollama + OpenAI-compatible impls
  ingest/               HTML→text, URL fetch (JSON-LD + headless), YouTube transcript
  rag/                  chunking, embedding (de)serialize, cosine, top-k
  worker/               async job pipeline (fetch/OCR → title/summary/tags/embed)
  mailer/   reminder/   outbound SMTP; calendar reminder scheduler
  emailin/              inbound IMAP → note poller
  server/               Fiber routes + handlers
cmd/reset/              standalone "wipe all data" tool (builds to reset.exe)
web/templates/          server-rendered pages (Markdown editor)
update.ps1              update the binary in place, preserving all data
```
