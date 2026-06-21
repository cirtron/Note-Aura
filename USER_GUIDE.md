# Note-Aura — User Guide

Note-Aura is your **AI knowledge inbox**: throw anything in — typed notes, web
links, YouTube videos, or images — and AI titles, summarizes, tags, and indexes
it. Find things later by keyword search or by **asking your notes** a question.

> **At a glance:** Markdown notes with inline images · capture from web/YouTube/
> image (OCR) / **files** / **email** · AI **title + summary + tags + category**
> (only fills empty fields) · keyword search,
> **sort & pagination**, "ask your notes" (RAG) · **sub-categories**, tags &
> calendar with email reminders · public holidays · **bulk delete** ·
> **import/export** notes · share with people or **groups** (with co-admins &
> read/write control) · block users · invite new users · email verification ·
> multi-language UI (follows your browser) · bring your own AI key (and your own
> prompts). Admins get a **dashboard** and manage users (suspend/delete), roles &
> quotas, allowed upload types, branding, AI models/prompts, holidays, email,
> **HTTPS**, and whether sign-ups are open.

- [1. Getting started](#1-getting-started)
- [2. The interface](#2-the-interface)
- [3. Adding notes](#3-adding-notes)
- [4. How AI processing works](#4-how-ai-processing-works)
- [5. Organizing & browsing](#5-organizing--browsing)
- [6. Calendar & reminders](#6-calendar--reminders)
- [7. Ask your notes](#7-ask-your-notes)
- [8. Sharing & groups](#8-sharing--groups)
- [9. Blocking users](#9-blocking-users)
- [10. Inviting new users](#10-inviting-new-users)
- [11. Email → note](#11-email--note)
- [12. Settings](#12-settings)
- [13. Admin guide](#13-admin-guide)
- [14. FAQ](#14-faq)

---

## 1. Getting started

### Create an account
1. Open Note-Aura and click **Create one** on the sign-in page.
2. Enter your email and a password (at least 6 characters).
3. Type the characters shown in the **verification image** (captcha) on the
   form — it's there to block automated sign-ups. Entry isn't case-sensitive.
4. If the site has email enabled, you'll receive a **verification link** — open
   it to verify your address, then sign in. (Didn't get it? On the sign-in page
   use **Resend verification email**.)
5. If you were **invited** by another user, the registration page is pre-filled
   with your email and you're verified automatically — just set a password.

> The very first account created on a new deployment becomes the **admin**.

### Sign in
Enter your email and password, and type the characters from the **verification
image** (captcha). The same captcha also guards the **forgot-password** form.
If you get it wrong, a fresh image appears. Your session stays active for
~30 days.

---

## 2. The interface

The top bar has two rows:

- **Top row:** the logo, the main menu (**Notes, Calendar, Ask, Shared, Groups,
  Settings**, plus **Admin** if you're an admin), and a **+ Add note** button.
- **Second row (right):** the **language selector**, your email, and **Sign out**.

### Language
Pick a language from the selector (English / 繁體中文 / 简体中文 / 日本語). If you
never choose one, Note-Aura follows your **browser's language** automatically.
Your choice is **remembered on your account**, so it follows you to any device you
sign in on. (Before you log in, it's remembered on the device via a cookie.)
Emails Note-Aura sends you — verification and password reset — use this language.

---

## 3. Adding notes

Click **+ Add note**. You have three ways to capture:

### Write a note (Markdown)
Type in the Markdown editor (bold, headings, lists, links, code). You can also:
- **Insert images** — use the image button, or **drag-and-drop**/**paste** an
  image straight into the editor.
- Add an optional **title**, **category**, **tags**, a **schedule** (see
  Calendar), and a **summary language**.

If you leave the title blank, AI writes one for you.

### Add from a link (web, YouTube, or Facebook)
Paste a URL. Note-Aura fetches the page text (or the video's transcript) and
AI summarizes it into a note. Pick a **summary language** if you want.

**Facebook** posts, videos, and reels are supported the same way — paste the URL
and Note-Aura captures whatever content is accessible. The source badge shows
`facebook`. If your admin has configured Facebook cookies (Admin → AI settings),
richer text and transcripts are extracted; without cookies, Note-Aura uses the
public Open-Graph preview (title + description), which works for most public posts.

### Add an image (OCR)
Upload a photo or screenshot. AI extracts the text (OCR) and summarizes it.

### Upload a file
If your role allows it, an **Upload file** option appears (with the allowed file
types listed). How each kind is handled:
- **Images** → OCR, like above.
- **Text files** (`.txt`, `.md`, `.csv`, …) → their content becomes the note body.
- **Other files** (video, audio, PDF, Office, …) → stored as a **downloadable
  attachment**, linked in the note.

> Your admin sets which file types each role may upload.

### By email
You can also send things in by **email** — see [Email → note](#11-email--note).

> **Capture is instant.** The note is saved immediately and shows as
> *processing…* while AI works in the background. It becomes *ready* once done.
> The captured content is kept even if AI is briefly unavailable.

---

## 4. How AI processing works

When you capture something, AI does several things automatically — but **only for
the fields you left empty** (it won't overwrite a title, tags, or category you set):
- **Title** — a short title (only if you didn't type one).
- **Summary** — a 2–3 sentence summary.
- **Tags** — a few topical tags.
- **Category** — picks a fitting existing category, or proposes a new one.
- **Index** — embeds the content so you can search it and *ask your notes*.

### Summary language
On any capture form, choose **Summary language**:
- **Auto (match content)** — the summary follows the content's language.
- Or pick a specific language — the title and summary are written in it.

### Stopping AI processing
While a note is *processing*, a **■ Stop** button appears next to the status pill.
Clicking it cancels the in-flight AI work immediately — the captured content is
kept, and the note lands in a neutral **Stopped** state with a message "AI
processing was stopped." and an indigo **↻ Retry AI processing** button. Click
Retry whenever you are ready to re-run AI.

### If a note shows *failed*
This usually means the AI service was unreachable. The captured **content is still
saved** — open the note and click **↻ Retry AI processing** to fill in the
summary/tags once AI is back.

### Re-running AI after an edit
When you edit a note, you can tick which parts to **re-generate** —
**Title**, **Summary**, **Tags**, and/or **Category**. Boxes are pre-ticked only for
fields that are currently empty. Only the ticked parts change; untick everything to
just save your edits without using AI.

> AI re-runs on the note's **current content** — i.e. your edits. For notes
> captured from a link/video/file, editing then re-running uses **your edited
> text** (it does not re-fetch the original or overwrite your changes).

---

## 5. Organizing & browsing

- **Categories** — give a note one category (typed on the note form; AI can pick
  one if you leave it blank). Make a **sub-category** by typing a path like
  `Work/Project A`. The **Notes** sidebar shows categories as an indented tree;
  selecting a parent also shows its sub-categories.
- **Tags** — AI suggests tags; you can also add your own (comma-separated). Tags
  appear in the sidebar and as clickable chips on each note.
- **On phones** the category/tag sidebar is replaced by a collapsible
  **🔍 Filters** panel just under the menu — tap it to open the same category
  tree and tag chips and filter your notes.
- **Search** — the search box does fast keyword search over titles and content.
  Category and tag chips are clickable shortcuts to filter.
- **Sort** — order the list by **Modified date** (default), **Created date**,
  **Title**, **Category**, **Tag**, or **Added method** (how it was captured).
- **Per page / pagination** — the list shows **10** notes per page by default;
  change it (10 / 25 / 50 / 100) with the **Per page** selector, and use
  **Prev / Next** to move between pages. Your sort and page-size choices are
  remembered.
- **Bulk delete** — tick the checkboxes on notes (or **Select all**) and click
  **🗑 Delete selected** to remove several at once.
- Each note shows its **Created** and **Modified** time on the note page.

---

## 6. Calendar & reminders

Give a note a **schedule** (on the note form): an **event date**, optional
**start/end time** or **all-day**, and an optional **email reminder** (at the
time, or 5 min to 1 day before).

- The **Calendar** page shows a month grid with your scheduled notes; click a day
  for that day's agenda. Use ←/Today/→ to change months.
- **Public holidays** for countries you choose appear on the calendar (see
  Settings → Calendar holidays).
- **Reminders** are emailed to you before the event (requires the site to have
  email configured).

---

## 7. Ask your notes

The **Ask** page lets you ask a question in natural language. Note-Aura finds the
most relevant pieces of your notes (and notes shared with you) and answers with
**citations** linking back to the source notes.

> Ask is available when your role permits AI (or you've set your own API key in
> Settings). It may be hidden if AI is turned off for your role.

---

## 8. Sharing & groups

### Share a note with one person
Open a note you own → **Sharing** → enter their email, optionally tick **can
edit**, and **Share**. They'll see it under **Shared**.

### Groups
The **Groups** page lets you create groups and share notes with everyone in them.

- **Create a group** (subject to your role's limit). You become its **owner /
  group admin**.
- **Invite members** by email. They get a pending **invitation** and must
  **Accept** (or **Reject**) it from their Groups page — people are never added
  without consent.
- **Permissions** — each member is **read-only** or **read/write**. A group admin
  can toggle this any time (**Allow write** / **Set read-only**).
- **Co-admins** — a group admin can promote a member with **Make admin**.
  Co-admins can invite/remove members and manage permissions. Only the **owner**
  can delete the group.
- **Leave** — any member can leave a group (the owner deletes it instead).

### Share a note to a group
Open a note you own → **Share to a group** → pick a group. All members can read
it; those with read/write can edit it.

---

## 9. Blocking users

In **Settings → Blocked users**, block someone by email. While blocked, neither
of you can share notes with, or invite, the other. You **cannot block an admin**.
Unblock them any time from the same place.

---

## 10. Inviting new users

If your role allows it, **Settings → Invite new users** lets you invite people by
email. They receive a link to register, and joining via that link skips email
verification (the invite already proves their address). Next to the email field is
a **language dropdown** — pick the language the invitation email is written in (it
defaults to your current UI language). The choice is stored with the invitation, so
**Resend** reuses the same language.

You have a limited number of invitations (shown on the page); the limit is set by
your role and can be adjusted per user by an admin.

---

## 11. Email → note

If your admin has enabled it, **Settings → Email → note** gives you a private
email address. Anything you **send or forward** to it becomes a note.

1. Click **Generate my email address** — you get something like
   `notes+ab12cd34@yourdomain.com`.
2. Send or forward an email **to that address** (put it in the **To** field):
   - The **subject** becomes the note title, the **body** becomes the content,
     then AI summarizes/tags it.
   - If the email body is **just a link**, the linked page/video is fetched.
   - Allowed attachments (per your role) are saved with the note.
3. **Keep the address secret** — anyone who has it can add notes to your account.
   You can **Regenerate** it any time (the old one stops working).

> Use the exact address from Settings, send **To** it (not Bcc), and make sure
> it's the `+token` address — not the plain mailbox.

---

## 12. Settings

- **Storage** — your usage and limit (notes + attachments + inline images). If
  you hit the limit, free space or ask an admin to raise it.
- **AI backend** — by default the site's built-in AI is used. You can enter your
  own **OpenAI-compatible** Base URL + API key (OpenAI, Google Gemini's compatible
  endpoint, OpenRouter, …) to use your own account. With your own key you aren't
  subject to the built-in daily AI limit.
- **Your AI prompts** — when you run on **your own external AI server** (Base URL
  + API key above), you can customize the title/summary/tags/OCR prompts sent to
  it. (Built-in-AI users use the admin's prompts.)
- **Email → note** — generate/regenerate your private inbound address (above).
- **Import / export notes** — **Export** all your notes to a JSON file, or
  **Import** notes from one. Title, body, summary, tags, category, and schedule are
  preserved; imported notes are added as-is (no AI re-processing).
- **Built-in AI usage** — if a daily limit applies, your usage today is shown.
- **Calendar holidays** — tick the countries whose public holidays you want on
  your calendar.
- **Invite new users** / **Blocked users** — as above (you can delete your own
  pending invitations here).
- **Language** — also switchable from the top bar.

---

## 13. Admin guide

Admins see extra **Dashboard**, **Users**, and **Admin** menus. (The first
registered account is the admin; an admin can promote others.)

### Dashboard
A read-only overview:
- **Totals** — users, notes, groups, failed jobs.
- **Server monitor** — uptime, memory, goroutines, CPUs, Go version, job queue,
  database & uploads size.
- **User usage** — per user: role, note count, storage used vs. limit.
- **Recent notes** across all users.

The user-usage and recent-notes tables are **paginated** (Prev / Next).

### Users
The **Users** tab manages accounts:
- **Promote/demote admins** — tick **admin** on a user's row. (The system won't
  let you remove the last admin.)
- **Assign a role**, and set **per-user overrides** — storage (MB), daily AI
  limit, and invitation limit — overriding the role defaults.
- **Suspend / Unsuspend** — a suspended user can't sign in and is signed out of
  any active session. (You can't suspend yourself or the last admin.)
- **Delete** — removes the user and all their data. (Not yourself / last admin.)
- **Last visited** — each row shows when the user was last active and their IP.
- **Invitations** — every invitation sent by users is listed here; you can
  **delete** any of them. (Users can also delete their own in Settings.)

### Registration
Toggle **Allow new users to sign up**. When off, the public sign-up is closed and
only people you **invite** can register; the *Create one* link is hidden on the
sign-in page.

### Roles & privileges
Create roles and set, per role:
- **Capacity (MB)** — storage limit (0 = unlimited).
- **Max groups** — how many groups a user may **create** (0 = none, -1 = unlimited).
- **Built-in AI (Ollama)** — whether the role may use the bundled AI. *(Users with
  their own API key can use AI regardless.)*
- **Ollama/day** — daily built-in-AI usage cap (0 = unlimited). Doesn't apply to
  users on their own key.
- **Invites** — how many new-user invitations the role may send.
- **Uploads** — which file types the role may upload: tick **image / video /
  audio / document**, and/or list extra extensions (e.g. `epub, csv`).

> Customizing one's own prompts is no longer a role setting — any user on their
> **own external AI server** can edit their prompts in Settings.

### Public holidays
Load holiday data so users can show it on their calendar:
- **Import online** — enter country code(s) (e.g. `US, GB, JP`) to fetch from an
  online source.
- **Upload CSV** — for countries the online source doesn't cover (e.g. HK, TW,
  CN). Format: `date,name` per line, date as `YYYY-MM-DD` (a header row is fine).

### AI settings
- Choose a **separate model per task** (title, summary, tags, chat/ask, OCR,
  image analysis, embeddings) for the built-in backend, plus the Ollama host.
- Edit the global **prompts** for each task — including the **Category** prompt.
- **Web-link** and **YouTube** prompts — optional dedicated title/summary/tags
  prompts that override the general ones for those sources (blank = use the
  general prompt).
- **Facebook cookies (Netscape cookies.txt)** — paste an exported `cookies.txt`
  from a logged-in Facebook browser session to enable authenticated capture (fuller
  post text and video transcripts). Leaving this blank is fine — public links still
  get the Open-Graph preview. See INSTALL.md §7c for export instructions.

### HTTPS (TLS)
Enable or disable **HTTPS** and set the certificate/key file paths on the
**Admin → HTTPS (TLS)** page (the pair is validated when you save). This overrides
`config.yaml` and takes effect after the server is **restarted**.

### Branding
Upload an **organization logo** or set **wording** shown in the header and on the
sign-in page (logo → wording → "Note-Aura").

### Email (SMTP & IMAP)
- **Outbound (`smtp:`)** — email verification, invitations, and calendar reminders
  need SMTP in `config.yaml` (host, port, username, password, from, `starttls`).
  Without SMTP, those emails are skipped (and new accounts are auto-verified).
- **Inbound (`imap:`)** — to enable **Email → note**, configure an IMAP mailbox
  that accepts plus-addressing. See INSTALL.md §7d.

### Web-link capture (headless)
Most pages capture their full text automatically. For JavaScript-heavy sites, set
`fetch.headless: true` in `config.yaml` (needs Chrome installed) so pages are
rendered before extraction. See INSTALL.md §7b.

### Maintenance: update & reset
Two helper tools live alongside the program (run them on the server, from the
project folder):

- **Update the program, keep all data** — `update.ps1` (Windows/PowerShell) stops
  the server, rebuilds it from the latest code, and restarts it. Your notes, users,
  uploads, and settings are untouched.
  ```powershell
  .\update.ps1
  ```
- **Reset to a brand-new system** — `reset.exe` **permanently deletes every user,
  note, category, tag, and admin setting** (and all uploaded images), returning the
  install to factory-fresh. Stop the server first, then run it; type `RESET` to
  confirm. The next start re-seeds the first admin from `config.yaml`.
  ```powershell
  .\reset.exe
  ```
  ⚠️ Irreversible — back up `data/note-aura.db` and `uploads/` first if you might
  want the data back. See INSTALL.md §9.

---

## 14. FAQ

**My note is stuck on "processing".**
The AI backend may be busy or down. If it shows **failed**, open the note and
click **Retry AI processing** — the captured content is kept either way.

**The note has the content but no summary or tags.**
The AI step failed (often a slow/unreachable AI host, or large content over a big
model timing out). Make sure the AI host is reachable, then **Retry**. Admins can
raise `ai.timeout_seconds`.

**The summary is in the wrong language.**
Set **Summary language** on the note (or when capturing) and re-run AI for the
summary.

**I can't see the Ask page.**
Your role may have AI turned off. Set your own API key in Settings, or ask an
admin to enable AI for your role.

**A link saved with only a sentence or two.**
That page is JavaScript-rendered, so only its short description was extractable.
Ask your admin to enable the **headless** web-link fallback, then **re-capture**
the link (re-capturing — not Retry — re-fetches the page).

**My emailed note didn't appear.**
Send **to** the exact `notes+<token>@…` address from **Settings → Email → note**
(in the To field, not Bcc) — not the plain mailbox. Regenerated the address? Use
the new one.

**Can others use this over the internet?**
Yes, if the deployment is exposed publicly (behind HTTPS). Ask your admin.

**How do I change the interface language?**
Use the language selector in the top bar. With no choice set, it follows your
browser.

**New sign-ups are closed — how do I get in?**
An admin has turned off public registration. Ask an existing user to **invite**
you; the invitation link lets you register.

---

*Last updated: 2026-06-17.*
