# Note-Aura — Installation Guide

Note-Aura is a single Go binary plus a SQLite database file. It runs on
**Windows, Linux, and macOS**. The AI features talk to a local
[Ollama](https://ollama.com) server by default (each user can switch to an
OpenAI-compatible cloud backend in Settings).

- [1. Prerequisites](#1-prerequisites)
- [2. Install Ollama and pull models](#2-install-ollama-and-pull-models)
- [3. Get the code](#3-get-the-code)
- [4. Configure](#4-configure)
- [5. Build & run](#5-build--run) — [Windows](#windows) · [Linux](#linux) · [macOS](#macos)
- [5b. Serving over HTTPS (TLS)](#5b-serving-over-https-tls)
- [6. First login & admin setup](#6-first-login--admin-setup)
- [6b. Optional: calendar reminder emails](#6b-optional-calendar-reminder-emails)
- [7. Optional: YouTube ingest](#7-optional-youtube-ingest)
- [7b. Optional: full web-link capture (headless)](#7b-optional-full-web-link-capture-headless)
- [7c. Optional: Facebook capture with cookies](#7c-optional-facebook-capture-with-cookies)
- [7d. Optional: Email → note (IMAP)](#7d-optional-email--note-imap)
- [8. Running as a background service](#8-running-as-a-background-service)
- [9. Backup, upgrade & reset](#9-backup-upgrade--reset)
- [10. Troubleshooting](#10-troubleshooting)

---

## 1. Prerequisites

| Requirement | Version | Notes |
|---|---|---|
| **Go** | 1.25+ | Only needed to build. https://go.dev/dl/ |
| **Ollama** | latest | For local AI. Optional if every user uses a cloud key. |
| **yt-dlp** | latest | Optional — only for YouTube ingest. |
| **Chrome / Chromium** | latest | Optional — only for the headless web-link fallback (`fetch.headless: true`). Not needed by default. |
| **SMTP server** | — | Optional — email verification, invitations, reminders. |
| **IMAP mailbox** | — | Optional — inbound **Email → note** capture. |

No database server is required — SQLite is embedded (pure Go, no CGO, no
C compiler needed).

> **Minimum Linux server requirements:** only the Note-Aura binary itself.
> `yt-dlp` is only needed for YouTube capture; Chrome/Chromium is only needed
> when `fetch.headless: true` is set in `config.yaml`. All other features
> (notes, web-link capture, AI, calendar, email) work without either.
> See [§7](#7-optional-youtube-ingest) and [§7b](#7b-optional-full-web-link-capture-headless) for installation details.

---

## 2. Install Ollama and pull models

Install Ollama from https://ollama.com/download, then pull the default models:

```bash
ollama pull llama3.1            # title / summary / tags / ask-your-notes
ollama pull nomic-embed-text    # embeddings (RAG)
ollama pull deepseek-ocr        # image OCR + image analysis
```

You can substitute other models and select them later on the **Admin** page
(a different model can be chosen per task: title, summary, tags, chat, OCR,
image analysis, embeddings). Make sure Ollama is running:

```bash
ollama serve        # usually started automatically by the installer
curl http://localhost:11434/api/tags   # should return JSON
```

---

## 3. Get the code

```bash
git clone <your-repo-url> note-aura
cd note-aura
```

(Or copy the project folder to the target machine.)

---

## 4. Configure

Copy the example config and edit it:

**Windows (PowerShell):**
```powershell
Copy-Item config.example.yaml config.yaml
notepad config.yaml
```

**Linux / macOS:**
```bash
cp config.example.yaml config.yaml
nano config.yaml
```

Key settings:

```yaml
listen_addr: ":8090"              # host:port to listen on
db_path: "data/note-aura.db"      # created automatically
uploads_dir: "uploads"            # image attachments
base_url: "http://localhost:8090" # used in emailed links; use https:// with TLS
tls:                              # serve HTTPS — see section 5b (leave blank for HTTP)
  cert_file: ""
  key_file: ""
session:
  secure: false                   # set true when served over HTTPS
ai:
  ollama_url: "http://localhost:11434"
  chat_model: "llama3.1"
  embed_model: "nomic-embed-text"
  ocr_model: "deepseek-ocr"    # text extraction (OCR) from images
  image_model: "deepseek-ocr"  # image analysis / description
initial_user:                     # optional: auto-create the first admin
  email: "you@example.com"
  password: "change-me"
```

> The models above are fallback defaults. An admin can override the host,
> per-task models, and prompts at runtime on the **/admin** page.

---

## 5. Build & run

### Windows

```powershell
# from the project folder, in PowerShell
go build -o note-aura.exe .
.\note-aura.exe
```

To use a specific config file:
```powershell
.\note-aura.exe -config C:\path\to\config.yaml
```

### Linux

```bash
go build -o note-aura .
./note-aura
```

### macOS

```bash
# Apple Silicon or Intel — same command
go build -o note-aura .
./note-aura
```

> Cross-compiling is trivial because there is no CGO. Examples:
> ```bash
> GOOS=linux  GOARCH=amd64 go build -o note-aura-linux .
> GOOS=windows GOARCH=amd64 go build -o note-aura.exe .
> GOOS=darwin GOARCH=arm64 go build -o note-aura-macos .
> ```

When it starts you'll see:
```
note-aura listening on :8090 (http)
```
Open **http://localhost:8090**.

---

## 5b. Serving over HTTPS (TLS)

Note-Aura can terminate TLS itself. Point it at your certificate and private key
(PEM) — set **both** paths and it serves HTTPS; leave either blank for plain HTTP.

```yaml
base_url: "https://your-host:8090"   # emailed links use this
tls:
  cert_file: "certs/note-aura.crt"   # server cert — use fullchain if you have intermediates
  key_file:  "certs/note-aura.key"   # private key (PEM, NOT password-protected)
session:
  secure: true                       # cookies marked Secure (HTTPS only)
```

On startup the log then shows `note-aura listening on :8090 (https)`.

> **Admin toggle:** an admin can also enable/disable HTTPS and set the cert/key
> paths at runtime on the **Admin → HTTPS (TLS)** page (it validates the pair on
> save). That setting overrides `config.yaml` and takes effect after a **restart**.
> Still set `base_url` to `https://…` and `session.secure: true` in `config.yaml`.

Notes:
- The key must be **unencrypted** (no passphrase) — `ListenTLS` won't prompt for
  one. Decrypt first if needed: `openssl rsa -in enc.key -out note-aura.key`.
- If your cert has intermediates, use the **fullchain** (leaf + intermediates)
  for `cert_file`, or some clients report an incomplete chain.
- Binding port **443** may need elevated privileges (Linux: `setcap`, or run
  behind a proxy). Testing is easiest on a high port like 8090.
- **Self-signed** certs trigger a browser warning (expected). For public sites use
  a real cert (e.g. Let's Encrypt).
- Prefer a reverse proxy? Leave `tls:` blank (serve HTTP) and let nginx/Caddy/IIS
  terminate TLS — see section 8.

---

## 6. First login & admin setup

1. Open http://localhost:8090 and click **Create one** to register.
2. **The first account to register automatically becomes the admin**
   (or the `initial_user` from `config.yaml`, if set).
3. As admin, open the **Admin** link in the top nav to choose models per task
   and edit the AI prompts.
4. Other people can register their own accounts; each gets private notes and can
   share individual notes with another user.

Individual users who prefer a cloud model can open **Settings** and enter an
OpenAI-compatible Base URL + API key (works with OpenAI, Google Gemini's
compatible endpoint, OpenRouter, Groq, etc.).

**Languages need no configuration.** The UI ships in English / 繁體中文 / 简体中文 /
日本語 and follows the browser by default; each user can switch in the header and
their choice is saved to their account. Transactional emails (verification,
password reset, invitations) are sent in the recipient's UI language — for
invitations, in the language the inviter selects on the invite form.

---

## 6b. Optional: calendar reminder emails

Notes can have an event date/time; to send **email reminders** before an event,
configure SMTP in `config.yaml`:

```yaml
smtp:
  host: "smtp.gmail.com"   # leave blank to disable reminders
  port: 587                # 587 = STARTTLS, 465 = implicit TLS
  username: "you@gmail.com"
  password: "app-password" # Gmail: create an App Password
  from: "you@gmail.com"
```

A scheduler checks every minute and emails the note owner when an event's
reminder time arrives. With `host` blank, reminders are simply disabled.

## 7. Optional: YouTube ingest

YouTube capture requires `yt-dlp` on the `PATH`. Everything else — web links,
images, typed notes, email capture — works without it.

### What yt-dlp provides

| Feature | Without yt-dlp | With yt-dlp |
|---|---|---|
| YouTube URL → note | ❌ Not available | ✅ Title + description + full transcript |
| Facebook video/reel transcript | ❌ Open-Graph preview only | ✅ Caption track (with cookies) |
| All other features | ✅ Full functionality | ✅ Full functionality |

### Install yt-dlp

**Linux (recommended — always gets the latest version):**
```bash
# Method 1: direct binary (recommended — no Python or pip needed)
sudo wget https://github.com/yt-dlp/yt-dlp/releases/latest/download/yt-dlp \
     -O /usr/local/bin/yt-dlp
sudo chmod +x /usr/local/bin/yt-dlp

# Method 2: pip / pipx
pip install yt-dlp          # system Python
# or
pipx install yt-dlp         # isolated (requires pipx)

# Method 3: apt (version may lag behind upstream)
sudo apt install yt-dlp
```

**Windows:**
```powershell
winget install yt-dlp
# or download yt-dlp.exe from https://github.com/yt-dlp/yt-dlp/releases
# and place it in a folder on your PATH (e.g. C:\Windows\System32)
```

**macOS:**
```bash
brew install yt-dlp
```

### Verify

```bash
yt-dlp --version
```

### Keep yt-dlp up to date

YouTube changes its API frequently; an outdated yt-dlp is the most common cause
of YouTube capture failures. Update it regularly:

```bash
yt-dlp -U          # self-update (works for direct binary installs)
# or
pip install -U yt-dlp
```

### How transcripts are fetched

Note-Aura calls yt-dlp with the **iOS/web player client** first
(`--extractor-args youtube:player_client=ios,web`), which bypasses the
age-restriction gate that blocks the default web client for some videos.
If no caption language is found in the metadata, it retries with
`--sub-langs all` to download any available subtitle track before falling back
to the video description only.

---

## 7b. Optional: full web-link capture (headless)

> **Chrome / Chromium is NOT required to run Note-Aura.**
> The headless-browser fallback is a purely optional enhancement for a small
> subset of web pages. The table below shows what works with and without it.

### What works without Chrome (the default)

| Feature | Without Chrome | With Chrome (`headless: true`) |
|---|---|---|
| Typed notes | ✅ | ✅ |
| YouTube transcripts | ✅ (via yt-dlp) | ✅ |
| Standard web pages (server-rendered HTML) | ✅ Full text | ✅ Full text |
| Pages with JSON-LD structured data (most news/article sites) | ✅ Full text | ✅ Full text |
| **JavaScript-rendered pages** (React/Vue SPA shells) | ⚠️ Short description only | ✅ Full rendered text |
| Image OCR | ✅ | ✅ |
| Email → note | ✅ | ✅ |
| AI / Ask / Calendar / Groups | ✅ | ✅ |

Most pages work fine without headless. Enable it only if you notice a specific
site saving only a sentence or two.

### Enable headless

```yaml
fetch:
  headless: true            # render JS pages with headless Chrome
  headless_wait_ms: 2500    # how long to let a page's JS run before snapshotting
```

It only activates when the plain HTTP fetch returns fewer than ~600 characters,
and falls back gracefully if Chrome is not installed.
On startup: `web-link capture: headless-browser fallback enabled`.

### Install Chrome / Chromium on Linux

```bash
# Option A — Chromium (open-source, smaller, usually sufficient)
sudo apt install chromium-browser       # Ubuntu / Debian
sudo dnf install chromium               # Fedora / RHEL

# Option B — Google Chrome (full Chrome)
wget https://dl.google.com/linux/direct/google-chrome-stable_current_amd64.deb
sudo apt install ./google-chrome-stable_current_amd64.deb
```

Note-Aura finds Chrome/Chromium automatically via the system PATH.
No path configuration is needed.

### Install Chrome on Windows / macOS

Download and install **Google Chrome** or **Chromium** normally.
Note-Aura discovers it automatically via the registry / standard install paths.

---

## 7c. Optional: Facebook capture with cookies

Pasting a Facebook URL always works without extra setup — Note-Aura falls back to
the public **Open-Graph preview** (title + description) for any public post or page.

For richer extraction (full post text, video transcript, private/logged-in content),
you can supply an authenticated **Netscape cookies.txt** from a browser session
where you are logged in to Facebook:

1. In your browser (logged in to Facebook), install a cookies-export extension such
   as **Get cookies.txt LOCALLY** (Chrome/Firefox).
2. Open `facebook.com`, export cookies — make sure to select **Netscape format**
   (the `.txt` file starts with `# Netscape HTTP Cookie File`).
3. Copy the entire contents of that file.
4. In Note-Aura, sign in as an admin and open **Admin → AI settings**.
5. Paste the cookies text into the **Facebook cookies (Netscape cookies.txt)**
   textarea and save.

With cookies configured, Note-Aura tries two richer paths before the public fallback:

1. **yt-dlp + cookies** — downloads video/reel metadata and any caption track
   (requires `yt-dlp` on `PATH`; see [section 7](#7-optional-youtube-ingest)).
2. **Authenticated headless Chrome** — renders the page while logged in to capture
   text posts (requires headless enabled; see [section 7b](#7b-optional-full-web-link-capture-headless)).

**The cookies setting is entirely optional.** Public Facebook links (posts, pages,
videos with public captions) still capture at least the Open-Graph title and
description without any cookies.

> **Cookie expiry:** Facebook session cookies expire periodically. If authenticated
> capture stops working, re-export and paste fresh cookies.

---

## 7d. Optional: Email → note (IMAP)

Turn inbound email into notes. Configure one mailbox in `config.yaml`:

```yaml
imap:
  host: "imap.example.com"   # leave blank to disable
  port: 993                  # 993 = implicit TLS; 143 = STARTTLS or plain
  username: "notes@example.com"
  password: "app-password"   # Gmail/Outlook: use an app password, not your login password
  # TLS mode — pick the combination that matches your server:
  #   tls: true                      → implicit TLS on connect  (port 993, default)
  #   tls: false                     → plain connect + STARTTLS (port 143)
  #   tls: false  starttls: false    → fully plain, no encryption (port 143)
  tls: true
  starttls: true              # only used when tls: false; false = no TLS upgrade
  mailbox: "INBOX"
  address: "notes@example.com"  # public base address; defaults to username
  poll_seconds: 60
  delete_processed: false    # false = mark \Seen and keep; true = delete after import
  insecure_skip_verify: false  # set true only for self-signed / AV-intercepted certs
```

How it works: each user opens **Settings → Email → note** and generates a private
**plus-address** (e.g. `notes+ab12cd34@example.com`). Mail sent to that address is
turned into a note for that user. The secret `+token` routes the mail, so the
`From` header can't be spoofed to inject notes. The mailbox must **accept
plus-addressing** (Gmail, Fastmail, most servers do). A **link-only** email
captures the linked page's content.

> **Authentication failed?** For Gmail and Outlook you need an **app-specific
> password** — your regular login password won't work over IMAP. Generate one in
> your account's security settings (Google: *Security → App passwords*; Microsoft:
> *Security → Advanced security options → App passwords*). Also make sure IMAP
> access is enabled in the account settings.

> **TLS handshake error** (`first record does not look like a TLS handshake`) —
> you're connecting to a STARTTLS port with implicit TLS. Set `tls: false`.

> **Certificate error** (`certificate has expired or is not yet valid`) — a
> TLS-inspecting antivirus (e.g. Avast Mail Shield) is re-signing the cert. Either
> disable its mail SSL scanning or set `imap.insecure_skip_verify: true`.

---

## 8. Running as a background service

### Linux (systemd)

Create `/etc/systemd/system/note-aura.service`:

```ini
[Unit]
Description=Note-Aura
After=network.target

[Service]
WorkingDirectory=/opt/note-aura
ExecStart=/opt/note-aura/note-aura -config /opt/note-aura/config.yaml
Restart=on-failure
User=note-aura

[Install]
WantedBy=multi-user.target
```

```bash
sudo systemctl daemon-reload
sudo systemctl enable --now note-aura
```

### macOS (launchd)

Create `~/Library/LaunchAgents/com.note-aura.plist` pointing
`ProgramArguments` at the binary and `-config` path, then
`launchctl load ~/Library/LaunchAgents/com.note-aura.plist`.

### Windows (service)

Use [NSSM](https://nssm.cc/): `nssm install Note-Aura C:\note-aura\note-aura.exe -config C:\note-aura\config.yaml`,
set the working directory to the project folder, then start the service.

> For public/HTTPS deployments, either let Note-Aura terminate TLS directly
> (section 5b) **or** run behind a reverse proxy (nginx, Caddy, IIS) terminating
> TLS. Either way, set `session.secure: true` and a `https://` `base_url`.

---

## 9. Backup, upgrade & reset

**Backup** = copy two things while the app is stopped (or use SQLite online
backup): the database file (`data/note-aura.db`) and the `uploads/` folder.

**Upgrade:** pull new code, rebuild (`go build`), restart. The schema
self-migrates on startup; existing data is preserved.

### Helper scripts

Two helpers ship with the project for the common operations:

**`update.ps1` — update the program, keep all data** (Windows/PowerShell). Stops
the running `note-aura.exe`, rebuilds from the current source, and restarts it.
Your `note-aura.db`, `uploads/`, and `config.yaml` are never touched; if the build
fails, the old binary and data are left intact.

```powershell
.\update.ps1            # stop → rebuild → restart
.\update.ps1 -NoStart   # stop → rebuild only (don't relaunch)
```

**`update.sh` — the Linux/macOS equivalent.** Run it **on the server**, from the
source directory (`chmod +x update.sh` once). It stops the server, rebuilds, and
restarts — auto-detecting systemd (restarts the `note-aura` unit; override with
`NOTE_AURA_SERVICE=…`) or, failing that, relaunching in the background. Data is
preserved the same way; needs Go + the current source on the server.

```bash
chmod +x update.sh        # first time only
./update.sh               # stop → rebuild → restart
./update.sh --no-start    # stop → rebuild only
NOTE_AURA_SERVICE=my-unit ./update.sh   # custom systemd unit name
```

> **No Go on the server?** Build the binary elsewhere and copy it in instead:
> `GOOS=linux GOARCH=amd64 go build -o note-aura .` on a build machine, `scp` it to
> the server, then `sudo systemctl restart note-aura` (the binary is self-contained —
> templates are embedded).

**`reset.exe` — wipe everything back to a brand-new system.** A small companion
program (source in `cmd/reset/`). It reads the same `config.yaml` to find the
database and uploads directory, then **permanently deletes** them — every user,
note, category, tag, **and admin setting** (settings live inside the database).
The next launch recreates an empty schema and re-seeds the `initial_user` admin
from `config.yaml`, exactly like a fresh install.

```powershell
# Build it once (rebuild after pulling new code):
go build -o reset.exe ./cmd/reset/

# Stop the server first (it locks the database), then:
.\reset.exe         # prompts: type RESET to confirm
.\reset.exe -y      # skip the confirmation prompt
.\reset.exe -config C:\path\to\config.yaml
```

> ⚠️ This is irreversible. **Back up first** (copy `data/note-aura.db` + `uploads/`)
> if there's any chance you'll want the data back. `reset.exe` refuses with a clear
> error if the server is still running (the database file is locked) — stop it and
> retry.
>
> On Linux/macOS, build it the same way (`go build -o reset ./cmd/reset/`) and run
> `./reset`.

---

## 10. Troubleshooting

| Symptom | Cause / Fix |
|---|---|
| Notes stay **failed** with a "connection refused" error | Ollama isn't running or `ollama_url` is wrong. Start Ollama, then click **Retry AI processing** on the note. |
| `model 'x' not found` on a note | Pull it: `ollama pull x`, or pick an installed model on the Admin page. |
| Port already in use | Another process holds `listen_addr`. Change the port in `config.yaml`. |
| YouTube capture fails | Install `yt-dlp` (section 7) and confirm `yt-dlp --version`. Keep it up-to-date (`yt-dlp -U`) — YouTube changes its API frequently. |
| YouTube capture works for some videos but not others | Some videos are age-restricted or use a restricted player. Note-Aura automatically tries the iOS/web player client as a fallback. If it still fails, update `yt-dlp` to the latest version. |
| Can't reach the Admin page (403) | Only the admin account can. The first registered (or `initial_user`) account is the admin. |
| A web link saved with **only a sentence or two** | The page is JavaScript-rendered. Enable the headless fallback (section 7b) and **re-capture** the link. |
| Note has content but **no summary/tags** | The AI step failed (e.g. slow/large content over a big model). Increase `ai.timeout_seconds`, ensure the AI host is reachable, then **Retry**. |
| **OCR fails** with `ocr: ollama request: … context deadline exceeded` | The vision model (default `deepseek-ocr`, ~6.7 GB) was too slow to load+infer within `ai.timeout_seconds` — common on CPU or a remote Ollama, especially the first (cold) call. Raise `ai.timeout_seconds` (e.g. `900`) and **Retry**; the first call warms the model so later ones are faster. |
| **OCR returns nothing / garbage** (no error) | `deepseek-ocr` is prompt- and newline-sensitive. Leave the OCR/image prompts at their defaults (`Free OCR.` / `Describe this image in detail.`); if you set a custom one under **Admin → AI settings**, clear it. The model needs **Ollama ≥ v0.13.0** — check `ollama --version` and `ollama list`. |
| **Email → note** does nothing | Confirm `imap.host` is set and the startup log shows `email→note: polling …`. Send **to** your `notes+<token>@…` address (not the plain mailbox, not Bcc). A skipped mail logs the recipients/tokens it saw. |
| Email→note: `Authentication failed` | Wrong username or password. For Gmail/Outlook, use an **app-specific password** (your login password is blocked over IMAP). Also ensure IMAP access is enabled in the account settings. |
| Email→note: `first record does not look like a TLS handshake` | Port/TLS mismatch — you're using implicit TLS mode on a STARTTLS port. Set `imap.tls: false` (for port 143 + STARTTLS), or `imap.tls: false` + `imap.starttls: false` (plain, no encryption). |
| Email→note TLS error (`certificate … expired or not yet valid`) | A TLS-inspecting AV (Avast Mail Shield) is re-signing the cert. Disable its mail SSL scan, or set `imap.insecure_skip_verify: true`. |
| Build error about CGO | Not applicable — this project needs no C compiler. Ensure Go 1.25+. |
| `go build` fails with **`error obtaining VCS status: exit status 128`** (suggests `-buildvcs=false`) | Go tries to stamp the binary with Git revision info but the `git` call failed — usually a *dubious ownership* check on a **network share / mapped drive** checkout (e.g. building from `T:\…` or a `\\server\share` path), or a shell whose `git` global config (`safe.directory`) isn't loaded. The build itself is fine. **Fix:** build with `go build -buildvcs=false …` (the bundled `update.ps1` / `update.sh` already do this), or set it for all your Go builds with `go env -w GOFLAGS=-buildvcs=false`, or trust the path: `git config --global --add safe.directory '<the path git names in the error>'`. |
