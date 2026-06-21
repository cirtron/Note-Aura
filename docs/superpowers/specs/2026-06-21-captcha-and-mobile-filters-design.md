# Captcha + Mobile Filters — Design

**Date:** 2026-06-21
**Status:** Approved (amended 2026-06-21 — captcha switched from math text to image)

Two independent, small enhancements:

1. A built-in captcha on the login, registration, and forgot-password forms.
2. A Categories + Tags filter that is reachable on mobile (today it is hidden on phones).

---

## Amendment (2026-06-21): image captcha instead of math text

The captcha was first shipped as a "what is X + Y?" math text question. It is now
switched to a **distorted-image captcha** at the user's request. The stateless,
HMAC-signed-cookie architecture is unchanged — only the challenge representation and
the answer change:

- The challenge is a random **5-character alphanumeric code** drawn from a safe
  alphabet `ABCDEFGHJKMNPQRSTUVWXYZ23456789` (no O/0/I/1/L). The user types the
  characters shown; comparison is case-insensitive (trim + uppercase).
- `captcha.New()` renders the code into a distorted PNG (random background, per-glyph
  scaling + vertical jitter, speckle noise, random lines) and returns it as a
  `data:image/png;base64,…` URI in `Challenge.Image`, alongside the signed `Token`
  (the HMAC now covers a string code rather than an int sum). The answer is still
  never sent in clear text — only the rendered image and the HMAC.
- Rendering uses `golang.org/x/image/font/basicfont` (built-in bitmap font, no font
  file) + `golang.org/x/image/draw` for scaling + stdlib `image/png`.
- Server: `issueCaptcha` returns the data URI; `renderAuth` passes it as a
  `template.URL` (so `html/template` permits the data URI in `<img src>`).
  `checkCaptcha` is unchanged.
- Templates show `<img src="{{.CaptchaImage}}">` above the same `name="captcha"`
  input (now `autocapitalize="characters"`, no numeric inputmode). New i18n key
  `auth.captcha_hint` ("Type the characters shown") in all four languages;
  `auth.captcha` / `auth.captcha_error` are retained.

**Constraint change:** this overrides the original "standard-library only" rule for
the captcha — it adds the official, lightweight `golang.org/x/image` module. No
external captcha service is introduced.

## Feature 1 — Built-in math captcha

### Goal

Add a lightweight challenge to the three public auth forms (`/login`, `/register`,
`/forgot`) that blocks scripted/spam submissions, without depending on any external
service (Cloudflare/Google) or requiring config keys. Suits a self-hosted / intranet
deployment.

### Approach: stateless, HMAC-signed cookie

No database table and no config keys. State lives in a short-lived signed cookie.

New package `internal/captcha`:

- `Challenge` carries the human prompt (e.g. `"7 + 4"`) and a signed `Token`.
- `New() (Challenge, error)` — picks two small random integers (each 1–9), forms an
  addition prompt, computes the answer, and returns a signed token.
- Token format: `exp.mac` where
  - `exp` = unix expiry (now + 10 minutes), decimal.
  - `mac` = hex `HMAC-SHA256(secret, exp + "|" + answer)`.
  - The answer is **never** placed in the cookie in clear text, so a script cannot read
    it back — it must actually compute the sum.
- `Verify(token, submitted string) bool` — splits the token, rejects if expired,
  recomputes the HMAC over `exp + "|" + normalize(submitted)`, and compares with
  `hmac.Equal`. `normalize` trims whitespace.
- `secret` — 32 bytes from `crypto/rand`, generated once when the package/server
  initializes and held in memory. Captcha lifetime is minutes, so a per-process secret
  is sufficient (a server restart simply invalidates any in-flight challenge, which is
  harmless — the user just sees a fresh one).

### Server wiring (`internal/server`)

A cookie name constant `na_captcha` (short-lived, `HTTPOnly`, `SameSite=Lax`,
`Secure` following the existing session-cookie `Secure` setting, ~10 min expiry).

Helpers on `*Server`:

- `issueCaptcha(c) Challenge` — calls `captcha.New()`, sets the `na_captcha` cookie to
  the token, returns the challenge so the handler can pass `CaptchaPrompt` (and the
  token as a hidden field, read back from the cookie on POST) to the template.
- `checkCaptcha(c) bool` — reads the `na_captcha` cookie and the submitted `captcha`
  form value, calls `captcha.Verify`.

Handler changes in `auth_handlers.go`:

- `getLogin`, `getRegister`, `getForgot`: issue a challenge and pass `CaptchaPrompt` to
  the render map.
- `postLogin`, `postRegister`, `postForgot`: call `checkCaptcha` **before** any other
  validation or credential check. On failure, re-render the same form with an error
  message and a **fresh** challenge (so a wrong/used cookie cannot be replayed).
- Every existing failure path on these forms that re-renders the template must also
  issue a fresh captcha, otherwise the second attempt would have no challenge.

### Templates

`login.html`, `register.html`, `forgot.html` each gain, inside the form:

- A read-only prompt label showing `What is {{.CaptchaPrompt}}?` (localized).
- A required text input `name="captcha"`.

The token rides in the cookie, so no hidden token field is strictly required; the cookie
is the source of truth. (If a hidden field is desired for clarity it must not be trusted
over the cookie — the cookie is authoritative.)

### i18n

Add to every language in `internal/i18n/translations.go`:

- `auth.captcha` — e.g. "What is %s?" / label for the field.
- `auth.captcha_error` — e.g. "Incorrect answer to the verification question. Try again."

(Use the existing translation mechanism; mirror the wording across all language
variants so they do not drift.)

### Trade-off

A math captcha deters basic spam and scripted brute-force, not a determined targeted
attacker. That is the appropriate weight for a self-hosted notes app and adds zero
runtime dependencies.

---

## Feature 2 — Categories + Tags filter on mobile

### Problem

In `web/templates/notes_list.html` the filter sidebar is
`<aside class="w-44 shrink-0 hidden sm:block">` — completely hidden below the `sm`
breakpoint, so phone users cannot filter by category ("project") or tag from the notes
list.

### Approach: mobile-only collapsible panel (no JS)

- Add a `sm:hidden` block at the **top of the notes content column**, directly under the
  header menu, using a native `<details>`/`<summary>` element:
  - `<summary>` is a "🔍 Filters" toggle (localized), collapsed by default.
  - Expanded content reuses the **same** Categories tree (`.CatTree`) and Tag chips
    (`.TagCounts`) markup already rendered in the desktop sidebar, with the same active
    highlighting based on `.ActiveCategory` / `.ActiveTag`.
- The desktop sidebar (`hidden sm:block`) is unchanged.
- Only shown when `not .Shared` (matching the existing sidebar condition).

### No backend changes

`CatTree` and `TagCounts` are already passed to the template by `listNotes`. Feature 2
is purely presentational — no route, handler, or query changes.

### i18n

Add `notes.filters` (e.g. "Filters") to every language in `translations.go`.

---

## Out of scope

- External captcha providers (Turnstile/reCAPTCHA).
- Image/distorted-text captcha.
- Any change to the notes filtering query semantics.
- Treating "project" as a separate data concept — "project" maps to the existing
  category paths (e.g. `Work/Project A`).

## Testing

- `internal/captcha`: unit tests for `New`/`Verify` — correct answer passes, wrong
  answer fails, expired token fails, tampered token fails.
- Manual: load each of the three forms on a narrow viewport, confirm the captcha prompt
  renders and a wrong answer is rejected with a fresh challenge; confirm the mobile
  "Filters" panel appears only below `sm`, expands, and category/tag links filter the
  list.

## Docs

Update `CHANGELOG.md` (append, newest on top) and the user guides
(`USER_GUIDE.md`, `USER_GUIDE.zh-Hant.md`) to mention the captcha and the mobile filter.
