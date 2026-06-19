# Design: Complete UI i18n + per-invitation email language

- **Date:** 2026-06-15
- **Status:** Approved (brainstorming)
- **Working tree:** `C:\Project\Note-Aura` (not a git repo — cannot `git commit`; the
  git-tracked mirror lives at `t:\update\Note-Aura`)

## Summary

Note-Aura already has an i18n foundation (`internal/i18n`, 4 languages:
`en`, `zh-Hant`, `zh-Hans`, `ja`; templates call `{{t .Lang "key"}}`), but coverage
is partial and all outbound emails are hardcoded English. This work:

1. Completes UI translation across **all** templates (incl. admin pages).
2. Makes **all transactional emails** (invitation, verification, password reset)
   multilingual.
3. Lets the **inviter pick the invitation email's language** (stored per invitation).
4. Persists each user's chosen UI language to their account (`user_settings`).
5. Updates the docs (`INSTALL.md`, `README.md`, `USER_GUIDE.md`, `USER_GUIDE.zh-Hant.md`).

## Goals

- Every user-facing page renders fully in any of the 4 supported languages.
- Emails are sent in the language of whoever triggered them; invitations honour an
  inviter-chosen language that survives resends.
- A user's language choice follows their account across devices/browsers.
- Adding a new language later is a localized, low-friction change (one language entry +
  one translation map), with a test that flags incomplete coverage.

## Non-goals

- Shipping additional languages *in this work* (we make adding them easy, but author
  only the existing 4: `en`, `zh-Hant`, `zh-Hans`, `ja`).
- Translating admin-only free-text data (user-entered category names, note bodies, etc.).
- Localising dates/numbers beyond the existing `date` template helper.
- RTL layout support.

## Decisions (locked during brainstorming)

- **Email text mechanism:** reuse `i18n.T(lang, key)` + `fmt.Sprintf` placeholders
  (approach "A"). No separate template files or per-language Go builders.
- **Email language = "who's driving" rule:**
  - Verification / resend-verification / password-reset → `currentLang(c)` (the
    recipient is the person at the form).
  - Invitation → inviter picks from a dropdown (default = inviter's current UI
    language); the chosen code is **stored on the invitation** so resends (user and
    admin) reuse it.
- **Scope:** all pages including admin/dashboard/users/logs; all three emails.
- **Per-user language:** persisted to `user_settings` (key `lang`).

## Architecture

### 1. Per-user UI language persistence

- `setLang` (in `internal/server/middleware.go`): when a user is logged in, also write
  the chosen code to `user_settings` (`SetUserSetting(uid, "lang", code)`) in addition
  to the existing `lang` cookie.
- `detectLang` resolution order becomes:
  1. `lang` cookie (if supported) — honours an explicit just-clicked switch.
  2. logged-in user's stored `user_settings["lang"]` (if supported).
  3. `Accept-Language` header (`i18n.Match`).
  4. `i18n.Default` (`en`).
- `loadSession` already runs before `detectLang` in middleware order, so the user is
  available when `detectLang` reads stored settings. No ordering change needed.

### 2. Email translation mechanism (approach A)

- New translation keys under an `email.*` namespace, added to **all 4 languages** in
  `internal/i18n/translations.go`. Bodies use `%s` placeholders; the call site supplies
  values via `fmt.Sprintf(i18n.T(lang, key), args...)`. Keys:
  - `email.invite.subject` — args: inviter
  - `email.invite.body` — args: inviter, link
  - `email.verify.subject`
  - `email.verify.body` — args: link
  - `email.reset.subject`
  - `email.reset.body` — args: link
- Placeholder order is kept identical across languages (verified per language when the
  keys are authored). A short comment in `translations.go` documents the `%s` order for
  each email key.

### 3. Email plumbing (`internal/server/auth_handlers.go`, `invitations.go`)

- `sendInviteEmail(to, inviter, token, lang string)` — new `lang` param.
- `sendVerifyEmail(to, token, lang string)`, `sendResetEmail(to, token, lang string)` —
  new `lang` param.
- Call sites:
  - `inviteUser` → validate form `lang` (`i18n.Supported`, else `currentLang(c)`), store
    on the invitation, pass to `sendInviteEmail`.
  - `resendInvitation` / `adminResendInvitation` → read the invitation's stored `lang`.
  - `postRegister` (verify), `postResendVerify` → `currentLang(c)`.
  - `postForgot` (reset) → `currentLang(c)`.

### 4. Data model change

- `user_invitations` gains `lang TEXT NOT NULL DEFAULT ''`:
  - Add the column to the `schema` const in `internal/db/db.go`.
  - Add a lightweight migration line `ALTER TABLE user_invitations ADD COLUMN lang TEXT
    NOT NULL DEFAULT ''` next to the existing `sqlDB.Exec(ALTER …)` block (harmless
    "duplicate column" on already-migrated DBs).
- `db.Invitation` struct gains a `Lang string` field.
- `CreateInvitation(inviterID int64, email, token, lang string)` — new `lang` param;
  `GetInvitation` / `GetInvitationByToken` select the column.

### 5. UI translation coverage

Add the missing keys to all 4 language maps and replace hardcoded English with
`{{t .Lang "key"}}`. Current `{{t}}` coverage per template (from audit):

| Template | Current t-calls | Action |
|---|---|---|
| `layout.html` | 13 | finish any remaining strings |
| `login.html` | 6 | finish |
| `register.html` | 6 | finish |
| `notes_list.html` | 8 | finish |
| `note_edit.html` | 9 | finish |
| `ask.html` | 1 | translate fully |
| `calendar.html` | 1 | translate fully |
| `forgot.html` | 1 | translate fully |
| `admin.html` | 0 | translate fully |
| `dashboard.html` | 0 | translate fully |
| `group.html` | 0 | translate fully |
| `groups.html` | 0 | translate fully |
| `logs.html` | 0 | translate fully |
| `note_view.html` | 0 | translate fully |
| `reset.html` | 0 | translate fully |
| `settings.html` | 0 | translate fully + add invite-language dropdown |
| `users.html` | 0 | translate fully |

`.Lang` is already supplied to every page via `baseMap` / `withLang`; no handler needs a
new `.Lang` injection. New keys follow the existing dotted namespace convention
(`nav.*`, `auth.*`, `notes.*`, `settings.*`, `admin.*`, `dashboard.*`, `users.*`,
`logs.*`, `groups.*`, `calendar.*`, `ask.*`, `action.*`, `email.*`).

Missing keys fall back to English (then to the key), so an incomplete language never
breaks a page.

### 6. Invite form (`settings.html`)

- Add a language `<select>` to the invite form, options from `i18n.Languages` (Native
  names), pre-selected to the current UI language (`.Lang`). Field name `lang`.

### 7. Extensibility — adding a language later

`i18n.Languages` is the single source of truth: the UI switcher (`langs` template func),
the invite-language dropdown, and the parity test all iterate over it. Adding a language
must require touching only these well-defined spots:

1. Append one entry to `i18n.Languages` (`{Code, Native, English}`).
2. Add one `"<code>": { … }` map to `translations` (English fallback covers any gaps).
3. *(Optional, for header auto-detect)* add a branch to `i18n.Match` so the
   `Accept-Language` header maps to the new code; without it the language is still fully
   usable via the switcher/cookie/account preference, just not auto-detected.

No template, handler, or email code should need editing to introduce a language — that
is the design constraint. The key-parity test (see Testing) runs over **every** entry in
`i18n.Languages`, so a newly added language with missing keys is flagged immediately.

## Affected files

- `internal/i18n/translations.go` — many new keys × 4 languages (UI + `email.*`).
- `internal/server/middleware.go` — `setLang` persistence, `detectLang` order.
- `internal/server/auth_handlers.go` — email fn signatures + call sites.
- `internal/server/invitations.go` — invite lang capture/store/resend.
- `internal/db/db.go` — schema + migration for `user_invitations.lang`.
- `internal/db/invitations.go` — `Invitation.Lang`, query/insert changes.
- `web/templates/*.html` — wrap strings in `{{t}}`; invite dropdown in `settings.html`.
- Docs: `INSTALL.md`, `README.md`, `USER_GUIDE.md`, `USER_GUIDE.zh-Hant.md`.

## Testing

- **Unit:** extend i18n tests — every language in `i18n.Languages` has a `translations`
  map whose key set matches `en` (catches both missing translations and a newly added
  language); `T` fallback behaviour; `email.*` `Sprintf` produces no stray `%!s`
  (correct arg count per key).
- **DB:** invitation round-trip persists and returns `lang`; migration is idempotent.
- **Server:** `inviteUser` stores the submitted/validated language; `detectLang`
  precedence (cookie > stored > Accept-Language > default); `setLang` writes
  `user_settings` when authed.
- **Manual smoke:** switch UI language, confirm each page renders translated; send an
  invitation in a chosen language and confirm the email body language; trigger
  verify/reset and confirm language follows the form.
- `go build ./...` and `go test ./...` clean.

## Docs updates

- `README.md` — note multilingual UI (4 languages) + per-account language.
- `INSTALL.md` — any config/notes about language (none required to enable; document the
  switcher and that emails follow UI language / inviter choice).
- `USER_GUIDE.md` + `USER_GUIDE.zh-Hant.md` — how to change language, where the choice
  is remembered, and how to pick an invitation's language.

## Risks / mitigations

- **Translation completeness for `zh-Hans` / `ja`:** authored best-effort; English
  fallback prevents breakage. The i18n key-parity test surfaces gaps.
- **`Sprintf` placeholder drift across languages:** mitigated by the arg-count test and
  a documented placeholder-order comment per email key.
- **No git in working tree:** spec and code changes are not version-controlled here;
  sync to `t:\update\Note-Aura` separately if tracking is wanted.
