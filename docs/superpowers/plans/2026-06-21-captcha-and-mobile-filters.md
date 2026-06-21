# Captcha + Mobile Filters Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add a built-in math captcha to the login/register/forgot-password forms, and make the Categories + Tags filter reachable on mobile.

**Architecture:** A new dependency-free `internal/captcha` package produces a math challenge whose answer is carried in an HMAC-signed, short-lived cookie (never in clear text). The server issues a fresh challenge on every render of an auth page via a single `renderAuth` helper, and verifies it before processing login/register/forgot POSTs. The mobile filter is a pure-template change: a `sm:hidden` `<details>` panel reusing the data already passed to `notes_list.html`.

**Tech Stack:** Go 1.x, Fiber v2, `html/template`, `crypto/hmac` + `crypto/sha256` + `crypto/rand`, Tailwind (CDN).

## Global Constraints

- Module path is `note-aura` (e.g. import `note-aura/internal/captcha`).
- No new third-party dependencies — standard library only for the captcha.
- Captcha applies to `/login`, `/register`, `/forgot` only. The token-gated `/reset` form is NOT captcha-protected.
- The captcha cookie is the single source of truth for the expected answer; no clear-text answer is ever sent to the client.
- Every i18n key MUST be added to all four languages in `internal/i18n/translations.go`: `en`, `zh-Hant`, `zh-Hans`, `ja`.
- Per the project CLAUDE.md: append a dated entry to `CHANGELOG.md` (newest on top) and keep `README.md` / `USER_GUIDE.md` / `USER_GUIDE.zh-Hant.md` in sync.
- Cookie security flag follows the existing pattern: `Secure: s.cfg.Session.Secure`, `HTTPOnly: true`, `SameSite: "Lax"`, `Path: "/"`.

---

### Task 1: `internal/captcha` package

**Files:**
- Create: `internal/captcha/captcha.go`
- Test: `internal/captcha/captcha_test.go`

**Interfaces:**
- Consumes: nothing.
- Produces:
  - `type Challenge struct { Prompt string; Token string }`
  - `func New() (Challenge, error)` — `Prompt` like `"7 + 4"`, `Token` like `"<expUnix>.<hexHMAC>"`.
  - `func Verify(token, submitted string) bool` — true iff `submitted` (a base-10 int, surrounding spaces trimmed) is the correct answer for an unexpired, untampered `token`.

- [ ] **Step 1: Write the failing test**

Create `internal/captcha/captcha_test.go`:

```go
package captcha

import (
	"fmt"
	"strconv"
	"strings"
	"testing"
	"time"
)

// solve parses "a + b" and returns the integer answer as a string.
func solve(t *testing.T, prompt string) string {
	t.Helper()
	var a, b int
	if _, err := fmt.Sscanf(prompt, "%d + %d", &a, &b); err != nil {
		t.Fatalf("unparseable prompt %q: %v", prompt, err)
	}
	return strconv.Itoa(a + b)
}

func TestVerifyCorrectAnswer(t *testing.T) {
	ch, err := New()
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if !Verify(ch.Token, solve(t, ch.Prompt)) {
		t.Fatal("correct answer rejected")
	}
}

func TestVerifyAcceptsSurroundingSpaces(t *testing.T) {
	ch, _ := New()
	if !Verify(ch.Token, "  "+solve(t, ch.Prompt)+"  ") {
		t.Fatal("answer with spaces rejected")
	}
}

func TestVerifyWrongAnswer(t *testing.T) {
	ch, _ := New()
	wrong := solve(t, ch.Prompt) + "9" // never equal to the real answer
	if Verify(ch.Token, wrong) {
		t.Fatal("wrong answer accepted")
	}
}

func TestVerifyNonNumeric(t *testing.T) {
	ch, _ := New()
	if Verify(ch.Token, "abc") {
		t.Fatal("non-numeric answer accepted")
	}
}

func TestVerifyExpired(t *testing.T) {
	// Craft a token that expired one second ago.
	token := signToken(time.Now().Add(-time.Second).Unix(), 5)
	if Verify(token, "5") {
		t.Fatal("expired token accepted")
	}
}

func TestVerifyTampered(t *testing.T) {
	ch, _ := New()
	tampered := ch.Token + "00"
	if Verify(tampered, solve(t, ch.Prompt)) {
		t.Fatal("tampered token accepted")
	}
}

func TestVerifyMalformed(t *testing.T) {
	if Verify("not-a-token", "5") {
		t.Fatal("malformed token accepted")
	}
	if Verify("", "5") {
		t.Fatal("empty token accepted")
	}
}

func TestTokenHasNoClearTextAnswer(t *testing.T) {
	// Force a known prompt by signing directly, then ensure the answer digits
	// are not trivially the last path segment (it must be an HMAC, not the answer).
	token := signToken(time.Now().Add(time.Minute).Unix(), 7)
	parts := strings.SplitN(token, ".", 2)
	if len(parts) != 2 {
		t.Fatalf("bad token shape %q", token)
	}
	if parts[1] == "7" {
		t.Fatal("answer leaked in clear text")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/captcha/`
Expected: FAIL — `undefined: New`, `undefined: signToken`.

- [ ] **Step 3: Write minimal implementation**

Create `internal/captcha/captcha.go`:

```go
// Package captcha provides a dependency-free, stateless math challenge for the
// public auth forms. The answer is carried only inside an HMAC-signed token
// (never in clear text), so a script must actually solve the sum.
package captcha

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strconv"
	"strings"
	"time"
)

// ttl bounds how long a challenge stays valid.
const ttl = 10 * time.Minute

// secret signs tokens. It is random per process: a restart simply invalidates
// in-flight challenges, which only makes a user re-answer a fresh one.
var secret = mustSecret()

func mustSecret() []byte {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		panic("captcha: cannot read random secret: " + err.Error())
	}
	return b
}

// Challenge is a math question to show the user plus its signed token.
type Challenge struct {
	Prompt string // e.g. "7 + 4"
	Token  string // "<expUnix>.<hexHMAC>"
}

// New returns a fresh addition challenge with two operands in [1,9].
func New() (Challenge, error) {
	a, err := randDigit()
	if err != nil {
		return Challenge{}, err
	}
	b, err := randDigit()
	if err != nil {
		return Challenge{}, err
	}
	exp := time.Now().Add(ttl).Unix()
	return Challenge{
		Prompt: fmt.Sprintf("%d + %d", a, b),
		Token:  signToken(exp, a+b),
	}, nil
}

// Verify reports whether submitted is the right answer for an unexpired,
// untampered token. submitted is trimmed and parsed as a base-10 integer.
func Verify(token, submitted string) bool {
	n, err := strconv.Atoi(strings.TrimSpace(submitted))
	if err != nil {
		return false
	}
	exp, ok := tokenExpiry(token)
	if !ok || time.Now().Unix() > exp {
		return false
	}
	expected := signToken(exp, n)
	return hmac.Equal([]byte(expected), []byte(token))
}

// signToken builds "<exp>.<hex HMAC of exp|answer>".
func signToken(exp int64, answer int) string {
	payload := fmt.Sprintf("%d|%d", exp, answer)
	mac := hmac.New(sha256.New, secret)
	mac.Write([]byte(payload))
	return fmt.Sprintf("%d.%s", exp, hex.EncodeToString(mac.Sum(nil)))
}

// tokenExpiry pulls the exp field out of a token without trusting the MAC yet.
func tokenExpiry(token string) (int64, bool) {
	dot := strings.IndexByte(token, '.')
	if dot <= 0 {
		return 0, false
	}
	exp, err := strconv.ParseInt(token[:dot], 10, 64)
	if err != nil {
		return 0, false
	}
	return exp, true
}

// randDigit returns a uniform-ish integer in [1,9].
func randDigit() (int, error) {
	b := make([]byte, 1)
	if _, err := rand.Read(b); err != nil {
		return 0, err
	}
	return int(b[0]%9) + 1, nil
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/captcha/`
Expected: PASS (all tests ok).

- [ ] **Step 5: Commit**

```bash
git add internal/captcha/
git commit -m "feat(captcha): add dependency-free HMAC-signed math challenge"
```

---

### Task 2: Server wiring — issue & verify captcha on auth forms

**Files:**
- Create: `internal/server/captcha.go`
- Modify: `internal/server/auth_handlers.go` (renders + POST checks)

**Interfaces:**
- Consumes: `captcha.New`, `captcha.Verify` (Task 1); `s.cfg.Session.Secure`, `withLang`, `currentLang` (existing).
- Produces:
  - `func (s *Server) renderAuth(c *fiber.Ctx, tmpl string, m fiber.Map) error` — sets a fresh captcha cookie, injects `m["CaptchaPrompt"]`, renders `tmpl` with the `layout`.
  - `func (s *Server) checkCaptcha(c *fiber.Ctx) bool` — verifies the submitted answer against the cookie.

- [ ] **Step 1: Create the server captcha helpers**

Create `internal/server/captcha.go`:

```go
package server

import (
	"time"

	"github.com/gofiber/fiber/v2"

	"note-aura/internal/captcha"
)

// captchaCookie holds the signed captcha token between GET (issue) and POST
// (verify) of an auth form.
const captchaCookie = "na_captcha"

// issueCaptcha generates a challenge, stores its token in a short-lived cookie,
// and returns the human-readable prompt (empty string on the rare RNG error).
func (s *Server) issueCaptcha(c *fiber.Ctx) string {
	ch, err := captcha.New()
	if err != nil {
		return ""
	}
	c.Cookie(&fiber.Cookie{
		Name:     captchaCookie,
		Value:    ch.Token,
		Expires:  time.Now().Add(10 * time.Minute),
		HTTPOnly: true,
		SameSite: "Lax",
		Secure:   s.cfg.Session.Secure,
		Path:     "/",
	})
	return ch.Prompt
}

// checkCaptcha verifies the submitted "captcha" form value against the cookie.
func (s *Server) checkCaptcha(c *fiber.Ctx) bool {
	return captcha.Verify(c.Cookies(captchaCookie), c.FormValue("captcha"))
}

// renderAuth renders an auth template (login/register/forgot) with a freshly
// issued captcha, so the page's form always has a working, single-use challenge.
func (s *Server) renderAuth(c *fiber.Ctx, tmpl string, m fiber.Map) error {
	m["CaptchaPrompt"] = s.issueCaptcha(c)
	return c.Render(tmpl, withLang(c, m), "layout")
}
```

- [ ] **Step 2: Route all login/register/forgot renders through `renderAuth`**

In `internal/server/auth_handlers.go`, replace every `c.Render("login", withLang(c, MAP), "layout")` with `s.renderAuth(c, "login", MAP)`, and likewise for `"register"` and `"forgot"`. Pass the bare `fiber.Map{...}` (NOT wrapped in `withLang` — `renderAuth` wraps it). Leave `"reset"` renders calling `c.Render` unchanged.

Concretely, change these call sites:

- `getLogin` (1): `return s.renderAuth(c, "login", fiber.Map{"Title": "Sign in"})` — keep the `if !s.registrationOpen()` block setting `m["RegClosed"]` before the call (build `m` as a variable, then `return s.renderAuth(c, "login", m)`).
- `postLogin` (3): invalid credentials, suspended, unverified — each becomes `s.renderAuth(c, "login", fiber.Map{...})` keeping the existing status codes and map keys.
- `getRegister` (2): the RegClosed branch → `s.renderAuth(c, "login", fiber.Map{...})`; the success branch → build `m` then `return s.renderAuth(c, "register", m)`.
- `postRegister`: the RegClosed branch → `s.renderAuth(c, "login", ...)`; the `render` closure → `s.renderAuth(c, "register", fiber.Map{...})`; the "verify your email" notice → `s.renderAuth(c, "login", fiber.Map{...})`.
- `getVerify` (1, error): `s.renderAuth(c, "login", fiber.Map{...})`.
- `postResendVerify` (1): `s.renderAuth(c, "login", fiber.Map{...})`.
- `getForgot` (1): `return s.renderAuth(c, "forgot", fiber.Map{"Title": "Reset password"})`.
- `postForgot` (1, notice): `s.renderAuth(c, "forgot", fiber.Map{...})`.
- `getReset` error → renders `"login"`: `s.renderAuth(c, "login", fiber.Map{...})`. The `"reset"` render stays `c.Render`.
- `postReset`: the `"reset"` renders stay `c.Render`; the final success render of `"login"` → `s.renderAuth(c, "login", fiber.Map{...})`.

- [ ] **Step 3: Add captcha verification to the three POST handlers**

In `postLogin`, as the **first** statement of the body:

```go
	if !s.checkCaptcha(c) {
		c.Status(fiber.StatusBadRequest)
		return s.renderAuth(c, "login", fiber.Map{"Title": "Sign in",
			"Error": i18n.T(currentLang(c), "auth.captcha_error"), "Email": c.FormValue("email")})
	}
```

In `postRegister`, immediately after computing `email`, `password`, `inviteToken` and before the `canRegister` check:

```go
	if !s.checkCaptcha(c) {
		c.Status(fiber.StatusBadRequest)
		return s.renderAuth(c, "register", fiber.Map{"Title": "Create account",
			"Error": i18n.T(currentLang(c), "auth.captcha_error"), "Email": email, "InviteToken": inviteToken})
	}
```

In `postForgot`, as the **first** statement:

```go
	if !s.checkCaptcha(c) {
		c.Status(fiber.StatusBadRequest)
		return s.renderAuth(c, "forgot", fiber.Map{"Title": "Reset password",
			"Error": i18n.T(currentLang(c), "auth.captcha_error")})
	}
```

(`i18n` is already imported in `auth_handlers.go`.)

- [ ] **Step 4: Build and vet**

Run: `go build ./... && go vet ./internal/server/`
Expected: no output (success). Existing server tests still compile: `go test ./internal/server/`
Expected: PASS (no behavior they assert is broken; auth tests, if any, do not submit forms).

- [ ] **Step 5: Commit**

```bash
git add internal/server/captcha.go internal/server/auth_handlers.go
git commit -m "feat(auth): issue and verify captcha on login/register/forgot"
```

---

### Task 3: Captcha field in templates + i18n strings

**Files:**
- Modify: `web/templates/login.html`, `web/templates/register.html`, `web/templates/forgot.html`
- Modify: `internal/i18n/translations.go`

**Interfaces:**
- Consumes: `.CaptchaPrompt` (set by `renderAuth`, Task 2); `auth.captcha`, `auth.captcha_error` keys (added here).
- Produces: a `name="captcha"` form field on each auth page.

- [ ] **Step 1: Add the i18n keys to all four languages**

In `internal/i18n/translations.go`, add these two keys inside each language's `auth.*` group (next to `"auth.password"`), for `en`, `zh-Hant`, `zh-Hans`, `ja` respectively:

```go
// en
		"auth.captcha":                    "Verification",
		"auth.captcha_error":              "Incorrect answer to the verification question — please try again.",
// zh-Hant
		"auth.captcha":                    "驗證碼",
		"auth.captcha_error":              "驗證碼答案不正確，請再試一次。",
// zh-Hans
		"auth.captcha":                    "验证码",
		"auth.captcha_error":              "验证码答案不正确，请重试。",
// ja
		"auth.captcha":                    "認証",
		"auth.captcha_error":              "認証の答えが正しくありません。もう一度お試しください。",
```

- [ ] **Step 2: Add the captcha input to each form**

In `web/templates/login.html`, inside the `<form method="post" action="/login" ...>` (after the password input, before the submit button), add:

```html
    {{if .CaptchaPrompt}}
    <div>
      <label class="block text-sm text-neutral-600 mb-1">{{t .Lang "auth.captcha"}}: {{.CaptchaPrompt}} = ?</label>
      <input name="captcha" type="text" inputmode="numeric" autocomplete="off" required
             class="w-full border rounded px-3 py-2">
    </div>
    {{end}}
```

In `web/templates/register.html`, add the identical block inside `<form method="post" action="/register" ...>` after the password input, before the submit button.

In `web/templates/forgot.html`, add the identical block inside `<form method="post" action="/forgot" ...>` after the email input, before the submit button.

- [ ] **Step 3: Build and manually verify**

Run: `go build ./...`
Expected: success.

Manual check (start the app, e.g. `go run ./cmd/... ` or run `note-aura.exe`, then in a browser):
- `/login`, `/register`, `/forgot` each show "Verification: a + b = ?" with an input.
- Submitting a wrong number re-renders the same form with the error text and a NEW sum (different a/b).
- Submitting the correct number proceeds (login attempt / account creation / reset email path).

- [ ] **Step 4: Commit**

```bash
git add web/templates/login.html web/templates/register.html web/templates/forgot.html internal/i18n/translations.go
git commit -m "feat(auth): show captcha field on login/register/forgot forms"
```

---

### Task 4: Mobile Categories + Tags filter panel

**Files:**
- Modify: `web/templates/notes_list.html`
- Modify: `internal/i18n/translations.go`

**Interfaces:**
- Consumes: `.Shared`, `.CatTree` (`.Path`/`.Leaf`/`.Depth`/`.Count`), `.TagCounts` (`.Name`/`.Count`), `.ActiveCategory`, `.ActiveTag`, the `mul` template func, and the `notes.filters` key — all already available except the new key.
- Produces: a `sm:hidden` collapsible filter panel; no backend change.

- [ ] **Step 1: Add the `notes.filters` i18n key to all four languages**

In `internal/i18n/translations.go`, next to `"notes.tags"` in each language group:

```go
// en
		"notes.filters":                   "Filters",
// zh-Hant
		"notes.filters":                   "篩選",
// zh-Hans
		"notes.filters":                   "筛选",
// ja
		"notes.filters":                   "フィルター",
```

- [ ] **Step 2: Insert the mobile panel**

In `web/templates/notes_list.html`, immediately after the closing `</div>` of the heading block (the `<div class="flex items-center gap-3 mb-4">…</div>` at the top) and before `<div class="flex gap-6 items-start">`, insert:

```html
{{if not .Shared}}
<details class="sm:hidden mb-4 bg-white rounded-xl shadow-sm">
  <summary class="cursor-pointer select-none px-4 py-3 text-sm font-semibold text-neutral-600">🔍 {{t .Lang "notes.filters"}}</summary>
  <div class="px-4 pb-4 space-y-3">
    <div>
      <h3 class="text-xs font-semibold text-neutral-400 uppercase mb-2">{{t .Lang "notes.categories"}}</h3>
      <ul class="space-y-1 text-sm">
        <li><a href="/notes" class="{{if not .ActiveCategory}}font-semibold text-indigo-600{{else}}text-neutral-700{{end}}">{{t .Lang "notes.all"}}</a></li>
        {{range .CatTree}}
        <li>
          <a href="/notes?category={{.Path}}" title="{{.Path}}" style="padding-left:{{mul .Depth 12}}px" class="flex justify-between gap-2 {{if eq $.ActiveCategory .Path}}font-semibold text-indigo-600{{else}}text-neutral-700{{end}}">
            <span class="truncate">{{.Leaf}}</span>{{if .Count}}<span class="text-neutral-400">{{.Count}}</span>{{end}}
          </a>
        </li>
        {{end}}
        {{if not .CatTree}}<li class="text-neutral-400 text-xs">none yet</li>{{end}}
      </ul>
    </div>
    <div>
      <h3 class="text-xs font-semibold text-neutral-400 uppercase mb-2">{{t .Lang "notes.tags"}}</h3>
      <div class="flex flex-wrap gap-1">
        {{range .TagCounts}}
        <a href="/notes?tag={{.Name}}" class="px-1.5 py-0.5 rounded text-xs {{if eq $.ActiveTag .Name}}bg-indigo-600 text-white{{else}}bg-indigo-50 text-indigo-600 hover:bg-indigo-100{{end}}">#{{.Name}} <span class="opacity-60">{{.Count}}</span></a>
        {{end}}
        {{if not .TagCounts}}<span class="text-neutral-400 text-xs">none yet</span>{{end}}
      </div>
    </div>
  </div>
</details>
{{end}}
```

- [ ] **Step 3: Build and manually verify**

Run: `go build ./...`
Expected: success.

Manual check (browser at a narrow/phone viewport, logged in, on `/notes`):
- A "🔍 Filters" toggle appears under the header; the desktop sidebar is hidden.
- Expanding it shows the category tree and tag chips; clicking one filters the list (URL gains `?category=` / `?tag=`) and the active item is highlighted.
- At desktop width (`≥ sm`) the panel is hidden and the original sidebar shows instead.

- [ ] **Step 4: Commit**

```bash
git add web/templates/notes_list.html internal/i18n/translations.go
git commit -m "feat(notes): add mobile collapsible category/tag filter panel"
```

---

### Task 5: Docs — changelog and user guides

**Files:**
- Modify: `CHANGELOG.md`
- Modify: `USER_GUIDE.md`, `USER_GUIDE.zh-Hant.md`
- Modify (if it documents auth/filtering): `README.md`

**Interfaces:** none (documentation only).

- [ ] **Step 1: Prepend a dated CHANGELOG entry**

Add to the top of `CHANGELOG.md` (do not alter existing entries):

```markdown
## 2026-06-21

- **Sign-in / sign-up captcha.** The login, registration, and forgot-password
  forms now include a simple "what is X + Y?" verification question to deter
  automated sign-up and password-guessing. No external service or keys required.
- **Mobile filters.** On phones, the notes list now has a collapsible
  "🔍 Filters" panel (under the menu) exposing the category and tag filters that
  were previously only visible on larger screens.
```

- [ ] **Step 2: Update the user guides**

In `USER_GUIDE.md` and `USER_GUIDE.zh-Hant.md`, in the sign-in/account section, note the new verification question on login/register/forgot; and in the notes/filtering section, note the mobile "Filters" panel. Mirror the wording in the Traditional Chinese guide. If `README.md` describes the auth flow or the notes UI, add a one-line mention there too; otherwise state in the commit that README needed no change.

- [ ] **Step 3: Commit**

```bash
git add CHANGELOG.md USER_GUIDE.md USER_GUIDE.zh-Hant.md README.md
git commit -m "docs: document captcha and mobile filters"
```

---

## Self-Review notes

- **Spec coverage:** captcha package (Task 1) ✓; server issue/verify on login+register+forgot (Task 2) ✓; templates + i18n (Task 3) ✓; mobile collapsible filter reusing existing data (Task 4) ✓; docs (Task 5) ✓. Reset form intentionally excluded per spec.
- **No clear-text answer:** enforced by `TestTokenHasNoClearTextAnswer` and the HMAC design.
- **Replay/used-cookie:** every auth re-render goes through `renderAuth`, which issues a brand-new challenge, so a failed attempt always gets a fresh sum.
- **Type consistency:** `Challenge{Prompt,Token}`, `New`, `Verify`, `signToken` names match across Task 1 code/tests and Task 2 callers; template field `.CaptchaPrompt` matches `renderAuth`.
