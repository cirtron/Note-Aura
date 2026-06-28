# Changelog

All notable changes to Note-Aura are recorded here, newest first.

## 2026-06-27

### Added
- **Admin: IP address blocking.** Admins can now block IP addresses from the new **Admin → Blocked IPs** page. Blocked IPs are rejected with HTTP 403 at the middleware level, before any session is resolved. The block list supports optional reason notes and shows the time each IP was blocked. Unblocking is instant.
- **Admin: configurable login lockout.** After a configurable number of consecutive failed login attempts, the account is locked for a configurable number of minutes. Both thresholds are set in **Admin → Login Lockout** (`login_max_attempts`, default 5; `login_lockout_minutes`, default 15). On successful login, the counter is reset. Admins can clear any user's lockout from the Users page.
- **Admin: force-logout and timed suspension.** The Users page now has a **Force logout** button that immediately invalidates all active sessions for a user, and the Suspend form accepts an optional number of hours for a timed suspension (blank = permanent). Timed suspensions lift automatically at the chosen time.
- **Site announcement banner.** Admins can write a site-wide announcement in **Admin → Announcement** and toggle its visibility. When enabled, the message appears at the top of every page (below the nav header) in an amber banner.
- **Admin: send email.** Admins can send a plain-text email to a specific user address, all users in a role, or all members of a group via **Admin → Send Email**. Requires SMTP to be configured; the page shows a notice when email is disabled.
- **Login lockout error messages.** The login page now shows specific, translated error messages for: invalid credentials, account locked (with remaining minutes), account suspended, and unverified email — instead of a single generic "Invalid email or password." All four messages are translated into English, 繁體中文, 简体中文, and 日本語.

## 2026-06-26

### Added
- **Attachments shown in note view.** After uploading an image or file, the note page now displays a dedicated **Attachments** section at the bottom of the note body:
  - **Every file** (image, `.txt`, `.docx`, `.pdf`, video, etc.) shows a `📎 filename (size)` download link.
  - **Images** additionally render as an inline preview below their download link.
  - The section is backed by the `attachments` database table, so it works even when the note body contains only OCR-extracted text and the image URL is not embedded in the Markdown.
- **One-time submit buttons on the note form.** All submit buttons on the New note / Edit note page (URL capture, image upload, file upload, and the manual note form) are disabled immediately after the first click. This prevents a double-tap or a slow network from creating duplicate notes before the server redirect arrives.

### Fixed
- **Captcha cookie replay prevention.** The image-captcha verification cookie is now expired immediately after every check (pass or fail), so the same challenge token cannot be reused. A fresh cookie and image are issued on the next page render as before.
- **Resend-verification form lacked captcha check.** The "Resend verification email" link on the sign-in page sent a new verification email without verifying the captcha, unlike the sign-in / register / forgot-password forms. It now verifies the captcha first, consistent with the rest of the auth flow.
- **OCR and image analysis now output in the user's selected language.** Text extracted from images (OCR) and image-description output are now produced in the user's chosen **summary language** (e.g. 繁體中文 / 简体中文). This mirrors the existing behaviour of title, summary, and category generation. Both Ollama and cloud (OpenAI-compatible) providers are updated; the language instruction is appended to the prompt sent with the image.
- **OCR produced duplicate content (same text twice, separated by `[Image] ---`).** The image-description (`Describe`) call is now skipped when OCR already returned ≥ 50 runes of text. For text-heavy document images, OCR and Describe used the same vision model, which re-read the text instead of adding visual context, creating a verbatim duplicate with `[Image]` prefix. Describe is still run for photos and diagrams where OCR returns nothing or very little.
- **`update.ps1`: server output was silently discarded.** `Start-Process` launched the executable with no stdout/stderr redirect, so startup errors were invisible. The script now launches via `cmd /c ... >> note-aura.log 2>&1` (matching `update.sh`'s `nohup >>` behaviour), appending all output to `note-aura.log` in the project folder.

## 2026-06-21

### Added
- **Sign-in / sign-up image captcha.** The login, registration, and forgot-password forms now show a distorted **image verification code** (5 alphanumeric characters); you type the characters shown. It deters automated sign-up and password-guessing, is built in — no external service (Cloudflare/Google) and no API keys required, so it works on offline/intranet deployments. The image is generated on the server and the code is carried only in a short-lived, HMAC-signed cookie (never sent to the browser in clear text); a wrong attempt always gets a fresh image. Entry is case-insensitive. Labels are translated into English, 繁體中文, 简体中文, and 日本語.
- **Mobile filters.** On phones, the notes list now has a collapsible **🔍 Filters** panel directly under the menu, exposing the category ("project") and tag filters that were previously only visible on larger screens. It reuses the existing filter data and highlights the active category/tag; the desktop sidebar is unchanged.

## 2026-06-17

### Added
- **Facebook link capture.** Paste a Facebook link (post, video, reel, or watch page) and Note-Aura captures its content like a regular web or YouTube link — title, text, and any available transcript are fetched, then AI generates a title, category, tags, and summary. The source badge shows `facebook`. Three fetch paths are tried in order: yt-dlp with cookies (richest — video metadata + transcript), authenticated headless Chrome (text posts), and public Open-Graph preview (fallback, no cookies required). Admins can configure the cookies path via **Admin → AI settings → Facebook cookies (Netscape cookies.txt)**; leaving it blank still gets the public preview for most links.
- **Stop button for AI processing.** While a note is in the `processing` state, owners and editors see a **Stop** button next to the status pill. Clicking it cancels any in-flight AI job, deletes queued jobs, and transitions the note to a neutral "Stopped" state. The note view shows a neutral box ("AI processing was stopped.") with an indigo **Retry** button instead of the red error box. Retrying clears the stopped flag and re-queues AI. All UI strings are translated into English, 繁體中文, 简体中文, and 日本語.

### Added
- **Worker lifecycle logging.** The worker now logs `processing note N job M (source=…)` when it starts a job and `completed note N job M` on success (previously it logged only on failure). This makes it visible whether a slow OCR is actually running vs. not being picked up at all.
- **Startup log of effective AI settings.** On launch Note-Aura now logs `AI config: ollama_url=… vision_model=… per-call timeout=…`, so OCR/timeout problems can be diagnosed from the log — e.g. confirming whether a raised `ai.timeout_seconds` actually took effect after a redeploy.
- **`update.sh` — Linux/macOS update helper.** The counterpart to `update.ps1`: run it on the server from the source dir to stop → rebuild → restart while preserving all data. Auto-detects systemd (restarts the `note-aura` unit, override with `NOTE_AURA_SERVICE`) or relaunches in the background. Documented in README and INSTALL.md §9.

### Fixed
- **OCR timed out ("context deadline exceeded") and the timeout setting was ignored.** Both AI providers hard-coded a 180-second HTTP-client timeout, so raising `ai.timeout_seconds` in config.yaml had no effect on Ollama/cloud calls. Because generation is non-streaming, that single timeout must cover the *entire* response — and a cold `deepseek-ocr` load + inference on a CPU or remote host routinely exceeds 180s, failing every OCR. The provider HTTP timeout is now driven by `ai.timeout_seconds`, the default was raised to **600s**, and the config comment explains it. Raise it further (e.g. 900) for very slow vision hosts.
- **YouTube transcripts often missing (only the description was captured).** The transcript fetch used `yt-dlp … -o -`, but yt-dlp does **not** pipe subtitles to stdout — it wrote a file literally named `-.<lang>.vtt`, so the primary capture always came back empty (and littered the working directory), leaving every video to the rate-limit-prone fallback. yt-dlp now downloads the caption track into a throwaway temp directory and reads the `.vtt` file, with `--retries`/`--retry-sleep` to soften YouTube's HTTP 429 rate-limiting. Result: transcripts are captured reliably for videos that have captions; the temp dir is auto-removed (no more stray `-.en.vtt`).
- **OCR / image analysis produced nothing with deepseek-ocr.** The default vision model `deepseek-ocr` is a specialist, newline-sensitive model trained on specific prompts; Note-Aura sent generic instructions ("Extract all text from this image…") that it isn't trained on, yielding empty/garbage output. The default OCR and image-analysis prompts now use the model's documented forms (`Free OCR.` / `Describe this image in detail.`, with the required leading newline), matching the working OmniScribe reference. Image-analysis failures, previously swallowed silently, are now logged to the admin syslog. **Note:** if you set a custom OCR/image prompt under Admin → AI settings, clear it (to pick up the new default) or set it to `Free OCR.`; deepseek-ocr also requires Ollama ≥ v0.13.0.
- **YouTube anti-bot page captured as transcript.** When YouTube/Google rate-limited the server's IP, the caption endpoint returned the "We're sorry … your computer or network may be sending automated queries" HTML page, and nothing checked it was a real subtitle — so that text was cleaned and stored as the video's transcript. Caption payloads are now accepted only if they carry the `WEBVTT` signature (new `parseCaption` guard, applied to both the yt-dlp stdout path and the metadata caption-URL path; the latter now fetches the track raw instead of through the bot-tolerant HTML extractor). A blocked fetch falls back to the video description instead of poisoning the note.
- **Stopped-note resurrection race (worker).** If a `provider.*` AI call happened to complete in the narrow window between `Cancel()` and the next DB write, the write would overwrite the Stop handler's `status=failed, stopped=1` state and leave the note in the inconsistent `status=ready, stopped=1` state (shown as a normal ready note). A cancellation guard (`ctx.Err()` check) is now applied immediately before each persistence write in `run()` that could overwrite stopped/failed state: the `ApplyAIResult` calls (no-AI path, early-save, and post-summary), `SetNoteTags`, `SetNoteCategory`, and `ReplaceChunks`. A cancelled context now returns the error before any of those writes, letting `process` take its existing no-op path and preserving the Stopped state.
- **Full multilingual UI.** Every page now renders in English / 繁體中文 / 简体中文 /
  日本語, including the previously English-only password-reset, forgot-password,
  Ask-your-notes, and calendar pages. A `TestLanguageKeyParity` test enforces that
  every translation key exists in all four languages.
- **Per-account language memory.** A user's chosen UI language is saved to their
  account and follows them across devices (cookie before login).
- **Localized transactional emails.** Verification and password-reset emails are
  sent in the recipient's language; invitation emails are sent in a language the
  inviter picks from a dropdown on the invite form (reused on Resend).

### Changed
- `user_invitations` gained a `lang` column (idempotent migration) storing the
  inviter-selected email language.

### Docs
- README, INSTALL, and both user guides (EN + 繁體中文) document the language
  selector, per-account persistence, and email language behavior.

### Verification
- `go build ./...`, `go vet ./...`, and `go test ./...` all pass; end-to-end smoke
  test confirmed all four languages render with no untranslated placeholders, plus
  language persistence and invitation-language storage.
