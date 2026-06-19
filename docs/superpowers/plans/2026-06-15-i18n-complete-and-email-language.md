# Complete UI i18n + Per-Invitation Email Language — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make every Note-Aura page and every transactional email fully multilingual, let the inviter choose (and persist) an invitation email's language, and remember each user's UI language on their account.

**Architecture:** Reuse the existing `internal/i18n` registry (`i18n.T(lang, key)` + `i18n.Languages`). Emails are sent in the language of whoever triggered the action (`currentLang(c)`); invitations carry an inviter-chosen `lang` stored on the `user_invitations` row. A user's chosen UI language is persisted to `user_settings["lang"]` and consulted by `detectLang`. Templates already receive `.Lang`; the work is adding keys (×4 languages) and wrapping hardcoded English in `{{t .Lang "key"}}`.

**Tech Stack:** Go 1.26, Fiber v2, Go `html/template`, SQLite (`modernc.org/sqlite`), Tailwind-in-template CSS.

**Working tree:** `C:\Project\Note-Aura` — **not a git repo.** "Commit" steps below are written as `git` commands for convention, but in this tree they are **no-ops**: instead, after each task run `go build ./...` and the task's tests, and treat green as the checkpoint. Do not attempt to `git init`.

---

## Conventions used throughout

- **Run build:** `cd /c/Project/Note-Aura && go build ./...`
- **Run all tests:** `go test ./...`
- **Run one package's tests:** e.g. `go test ./internal/i18n/ -run TestX -v`
- **Key namespaces** (dotted, matching existing `nav.*`, `auth.*`, …): `email.*`, plus
  page namespaces `settings.*`, `admin.*`, `dashboard.*`, `users.*`, `logs.*`,
  `groups.*`, `group.*`, `note.*`, `calendar.*`, `ask.*`, `action.*`, `reset.*`,
  `forgot.*`, `invite.*`, `block.*`, `holidays.*`.
- **English value = the current hardcoded string, verbatim.** When wrapping a template
  string, the `en` map value is exactly the text being replaced.
- **Translation authoring:** author `en` (verbatim) and `zh-Hant` for every new key.
  `zh-Hans` and `ja` are authored best-effort; the parity test (Task 2) lists any key
  missing from any language so nothing is silently skipped. Missing keys fall back to
  English at runtime, so the app never breaks mid-translation.

---

## File Structure

**Modify (Go):**
- `internal/db/db.go` — add `lang` to the `user_invitations` schema + migration.
- `internal/db/invitations.go` — `Invitation.Lang`, `CreateInvitation` signature, SELECTs.
- `internal/server/invitations.go` — capture/validate/store invite language; resend reuse.
- `internal/server/auth_handlers.go` — email fn signatures (`lang`), call sites.
- `internal/server/middleware.go` — `setLang` persistence; `detectLang` precedence.
- `internal/i18n/translations.go` — `email.*` keys + all new UI keys (×4 languages).

**Create (Go tests):**
- `internal/i18n/i18n_test.go` — key parity + `email.*` Sprintf arity + `T` fallback.
- `internal/db/invitations_lang_test.go` — invitation `lang` round-trip.
- `internal/server/lang_test.go` — `detectLang` precedence + `setLang` persistence.

**Modify (templates):** all under `web/templates/` — `settings.html` (incl. invite
dropdown), `admin.html`, `dashboard.html`, `users.html`, `logs.html`, `groups.html`,
`group.html`, `note_view.html`, `reset.html`, `forgot.html`, `ask.html`,
`calendar.html`, and finish partial ones (`layout`, `login`, `register`, `notes_list`,
`note_edit`).

**Modify (docs):** `README.md`, `INSTALL.md`, `USER_GUIDE.md`, `USER_GUIDE.zh-Hant.md`.

---

## Task 1: Add `lang` column to invitations (schema + migration + model)

**Files:**
- Modify: `internal/db/db.go` (schema const `user_invitations`, migration block ~line 373-409)
- Modify: `internal/db/invitations.go`
- Test: `internal/db/invitations_lang_test.go` (create)

- [ ] **Step 1: Write the failing test**

Create `internal/db/invitations_lang_test.go`:

```go
package db

import "testing"

func TestInvitationLangRoundTrip(t *testing.T) {
	d, err := Open(":memory:")
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer d.Close()

	uid, err := d.CreateUser("inviter@example.com", "hash", true, true, "")
	if err != nil {
		t.Fatalf("create user: %v", err)
	}
	if err := d.CreateInvitation(uid, "new@example.com", "tok-1", "zh-Hant"); err != nil {
		t.Fatalf("create invitation: %v", err)
	}

	byTok, err := d.GetInvitationByToken("tok-1")
	if err != nil {
		t.Fatalf("get by token: %v", err)
	}
	if byTok.Lang != "zh-Hant" {
		t.Errorf("GetInvitationByToken lang = %q, want zh-Hant", byTok.Lang)
	}

	byID, err := d.GetInvitation(byTok.ID)
	if err != nil {
		t.Fatalf("get by id: %v", err)
	}
	if byID.Lang != "zh-Hant" {
		t.Errorf("GetInvitation lang = %q, want zh-Hant", byID.Lang)
	}

	// Empty lang is allowed (defaults to "").
	if err := d.CreateInvitation(uid, "two@example.com", "tok-2", ""); err != nil {
		t.Fatalf("create invitation empty lang: %v", err)
	}
	inv2, _ := d.GetInvitationByToken("tok-2")
	if inv2.Lang != "" {
		t.Errorf("empty lang = %q, want \"\"", inv2.Lang)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/db/ -run TestInvitationLangRoundTrip -v`
Expected: FAIL — compile error (`CreateInvitation` wants 3 args; `Invitation` has no `Lang`).

- [ ] **Step 3: Add the schema column**

In `internal/db/db.go`, in the `user_invitations` block of the `schema` const, add the
`lang` column after `token`:

```sql
CREATE TABLE IF NOT EXISTS user_invitations (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    inviter_id  INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    email       TEXT    NOT NULL,
    token       TEXT    NOT NULL UNIQUE,
    lang        TEXT    NOT NULL DEFAULT '',
    accepted_at TIMESTAMP,
    created_at  TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);
```

- [ ] **Step 4: Add the idempotent migration**

In `internal/db/db.go`, in the `sqlDB.Exec(\`ALTER TABLE ...\`)` migration block (next to
the existing `ALTER TABLE jobs ADD COLUMN params ...` line), add:

```go
	sqlDB.Exec(`ALTER TABLE user_invitations ADD COLUMN lang TEXT NOT NULL DEFAULT ''`)
```

- [ ] **Step 5: Add `Lang` to the model and queries**

In `internal/db/invitations.go`:

Add the field to the struct:

```go
type Invitation struct {
	ID         int64
	InviterID  int64
	Email      string
	Token      string
	Lang       string
	Accepted   bool
	AcceptedAt sql.NullTime
	CreatedAt  time.Time
}
```

Change `CreateInvitation` to accept and store `lang`:

```go
// CreateInvitation records a new-user invitation in the inviter-chosen language.
func (d *DB) CreateInvitation(inviterID int64, email, token, lang string) error {
	_, err := d.SQL.Exec(
		`INSERT INTO user_invitations (inviter_id, email, token, lang) VALUES (?, ?, ?, ?)`,
		inviterID, email, token, lang)
	return err
}
```

Update `GetInvitationByToken` to select and scan `lang`:

```go
	err := d.SQL.QueryRow(
		`SELECT id, inviter_id, email, token, lang, accepted_at, created_at FROM user_invitations WHERE token=?`, token).
		Scan(&inv.ID, &inv.InviterID, &inv.Email, &inv.Token, &inv.Lang, &inv.AcceptedAt, &inv.CreatedAt)
```

Update `GetInvitation` likewise:

```go
	err := d.SQL.QueryRow(
		`SELECT id, inviter_id, email, token, lang, accepted_at, created_at FROM user_invitations WHERE id=?`, id).
		Scan(&inv.ID, &inv.InviterID, &inv.Email, &inv.Token, &inv.Lang, &inv.AcceptedAt, &inv.CreatedAt)
```

Update `ListAllInvitations` SELECT + Scan:

```go
	rows, err := d.SQL.Query(
		`SELECT i.id, i.inviter_id, i.email, i.token, i.lang, i.accepted_at, i.created_at, u.email
		 FROM user_invitations i JOIN users u ON u.id = i.inviter_id
		 ORDER BY i.created_at DESC`)
	...
		if err := rows.Scan(&v.ID, &v.InviterID, &v.Email, &v.Token, &v.Lang, &v.AcceptedAt, &v.CreatedAt, &v.InviterEmail); err != nil {
```

Update `ListInvitationsBy` SELECT + Scan:

```go
	rows, err := d.SQL.Query(
		`SELECT id, inviter_id, email, token, lang, accepted_at, created_at
		 FROM user_invitations WHERE inviter_id=? ORDER BY created_at DESC`, inviterID)
	...
		if err := rows.Scan(&inv.ID, &inv.InviterID, &inv.Email, &inv.Token, &inv.Lang, &inv.AcceptedAt, &inv.CreatedAt); err != nil {
```

- [ ] **Step 6: Run the test to verify it passes**

Run: `go test ./internal/db/ -run TestInvitationLangRoundTrip -v`
Expected: PASS

- [ ] **Step 7: Build everything (other call sites still break — expected)**

Run: `go build ./... 2>&1 | head`
Expected: build error only in `internal/server` (callers of `CreateInvitation` with the
old 3-arg signature). That is fixed in Task 5. Proceed.

- [ ] **Step 8: Checkpoint commit**

```bash
git add internal/db/db.go internal/db/invitations.go internal/db/invitations_lang_test.go
git commit -m "feat(db): store per-invitation email language"
```

---

## Task 2: i18n email keys + parity/Sprintf tests

**Files:**
- Modify: `internal/i18n/translations.go`
- Test: `internal/i18n/i18n_test.go` (create)

- [ ] **Step 1: Write the failing tests**

Create `internal/i18n/i18n_test.go`:

```go
package i18n

import (
	"fmt"
	"strings"
	"testing"
)

// Every language listed in Languages must have a translations map whose key set
// exactly matches the English map. Guards both missing translations and a newly
// added language with no/partial map.
func TestLanguageKeyParity(t *testing.T) {
	en := translations["en"]
	if len(en) == 0 {
		t.Fatal("english translations missing")
	}
	for _, lang := range Languages {
		m, ok := translations[lang.Code]
		if !ok {
			t.Errorf("language %q in Languages has no translations map", lang.Code)
			continue
		}
		for k := range en {
			if _, ok := m[k]; !ok {
				t.Errorf("language %q missing key %q", lang.Code, k)
			}
		}
		for k := range m {
			if _, ok := en[k]; !ok {
				t.Errorf("language %q has extra key %q not in en", lang.Code, k)
			}
		}
	}
}

// Email body/subject keys use %s placeholders; verify each language formats with
// the expected arg count and leaves no stray %!s(MISSING) verbs.
func TestEmailKeysSprintf(t *testing.T) {
	cases := []struct {
		key  string
		args []any
	}{
		{"email.invite.subject", []any{"alice@example.com"}},
		{"email.invite.body", []any{"alice@example.com", "https://x/register?invite=t"}},
		{"email.verify.subject", nil},
		{"email.verify.body", []any{"https://x/verify?token=t"}},
		{"email.reset.subject", nil},
		{"email.reset.body", []any{"https://x/reset?token=t"}},
	}
	for _, lang := range Languages {
		for _, c := range cases {
			out := fmt.Sprintf(T(lang.Code, c.key), c.args...)
			if strings.Contains(out, "%!") {
				t.Errorf("lang %q key %q bad format: %q", lang.Code, c.key, out)
			}
		}
	}
}

func TestTFallbackToEnglish(t *testing.T) {
	// A key present in en but (pretend) absent elsewhere falls back to english.
	if got := T("zh-Hant", "nav.notes"); got == "" {
		t.Error("expected a value for nav.notes")
	}
	if got := T("zh-Hant", "totally.unknown.key"); got != "totally.unknown.key" {
		t.Errorf("unknown key should return itself, got %q", got)
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/i18n/ -v`
Expected: FAIL — `TestEmailKeysSprintf` (keys don't exist → `T` returns the key, which
has no `%s`, so `Sprintf` emits `%!(EXTRA ...)` → contains `%!`). `TestLanguageKeyParity`
passes for now (existing keys are balanced).

- [ ] **Step 3: Add the `email.*` keys to all four languages**

In `internal/i18n/translations.go`, add these keys inside each language map. Placeholder
order is identical across languages:
`email.invite.subject` → `%s` = inviter; `email.invite.body` → `%s` inviter, `%s` link;
`email.verify.body` → `%s` link; `email.reset.body` → `%s` link.

Add a comment above the `en` block's email keys:

```go
		// Email bodies use %s placeholders (order fixed across all languages):
		//   email.invite.subject(inviter)  email.invite.body(inviter, link)
		//   email.verify.body(link)        email.reset.body(link)
```

**en:**

```go
		"email.invite.subject": "%s invited you to Note-Aura",
		"email.invite.body":    "%s invited you to join Note-Aura.\n\nCreate your account here:\n%s\n",
		"email.verify.subject": "Verify your Note-Aura email",
		"email.verify.body":    "Welcome to Note-Aura!\n\nVerify your email address by opening this link:\n%s\n",
		"email.reset.subject":  "Reset your Note-Aura password",
		"email.reset.body":     "Someone requested a password reset for your Note-Aura account.\n\nOpen this link to choose a new password (valid for 1 hour):\n%s\n\nIf you didn't request this, you can safely ignore this email.\n",
```

**zh-Hant:**

```go
		"email.invite.subject": "%s 邀請你加入 Note-Aura",
		"email.invite.body":    "%s 邀請你加入 Note-Aura。\n\n在此建立你的帳號：\n%s\n",
		"email.verify.subject": "驗證你的 Note-Aura 電郵",
		"email.verify.body":    "歡迎使用 Note-Aura！\n\n開啟以下連結以驗證你的電郵地址：\n%s\n",
		"email.reset.subject":  "重設你的 Note-Aura 密碼",
		"email.reset.body":     "有人為你的 Note-Aura 帳號要求重設密碼。\n\n開啟以下連結設定新密碼（1 小時內有效）：\n%s\n\n若不是你本人要求，可忽略此郵件。\n",
```

**zh-Hans:**

```go
		"email.invite.subject": "%s 邀请你加入 Note-Aura",
		"email.invite.body":    "%s 邀请你加入 Note-Aura。\n\n在此创建你的账号：\n%s\n",
		"email.verify.subject": "验证你的 Note-Aura 邮箱",
		"email.verify.body":    "欢迎使用 Note-Aura！\n\n打开以下链接以验证你的邮箱地址：\n%s\n",
		"email.reset.subject":  "重置你的 Note-Aura 密码",
		"email.reset.body":     "有人为你的 Note-Aura 账号请求重置密码。\n\n打开以下链接设置新密码（1 小时内有效）：\n%s\n\n若不是你本人请求，可忽略此邮件。\n",
```

**ja:**

```go
		"email.invite.subject": "%s さんが Note-Aura に招待しました",
		"email.invite.body":    "%s さんが Note-Aura への参加に招待しました。\n\nこちらからアカウントを作成してください：\n%s\n",
		"email.verify.subject": "Note-Aura のメールアドレスを確認",
		"email.verify.body":    "Note-Aura へようこそ！\n\n次のリンクを開いてメールアドレスを確認してください：\n%s\n",
		"email.reset.subject":  "Note-Aura のパスワードを再設定",
		"email.reset.body":     "Note-Aura アカウントのパスワード再設定が要求されました。\n\n次のリンクを開いて新しいパスワードを設定してください（1 時間有効）：\n%s\n\n心当たりがない場合は、このメールを無視してください。\n",
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/i18n/ -v`
Expected: PASS (all three tests).

- [ ] **Step 5: Checkpoint commit**

```bash
git add internal/i18n/translations.go internal/i18n/i18n_test.go
git commit -m "feat(i18n): add translated email keys + parity/sprintf tests"
```

---

## Task 3: Translate emails (auth_handlers.go)

**Files:**
- Modify: `internal/server/auth_handlers.go`

- [ ] **Step 1: Import i18n and rewrite the three email senders**

In `internal/server/auth_handlers.go`, add `"fmt"` and `"note-aura/internal/i18n"` to the
import block. Replace the three `send*Email` functions:

```go
// sendResetEmail emails a password-reset link in lang (no-op without SMTP).
func (s *Server) sendResetEmail(to, token, lang string) {
	if !s.mailer.Enabled() {
		return
	}
	link := s.LinkBase() + "/reset?token=" + token
	subject := i18n.T(lang, "email.reset.subject")
	body := fmt.Sprintf(i18n.T(lang, "email.reset.body"), link)
	go func() { _ = s.mailer.Send(to, subject, body) }()
}

// sendVerifyEmail emails a verification link in lang (no-op without SMTP).
func (s *Server) sendVerifyEmail(to, token, lang string) {
	if !s.mailer.Enabled() {
		return
	}
	link := s.LinkBase() + "/verify?token=" + token
	subject := i18n.T(lang, "email.verify.subject")
	body := fmt.Sprintf(i18n.T(lang, "email.verify.body"), link)
	go func() { _ = s.mailer.Send(to, subject, body) }()
}

// sendInviteEmail emails a new-user invitation link in lang (no-op without SMTP).
func (s *Server) sendInviteEmail(to, inviter, token, lang string) {
	if !s.mailer.Enabled() {
		return
	}
	link := s.LinkBase() + "/register?invite=" + token
	subject := fmt.Sprintf(i18n.T(lang, "email.invite.subject"), inviter)
	body := fmt.Sprintf(i18n.T(lang, "email.invite.body"), inviter, link)
	go func() { _ = s.mailer.Send(to, subject, body) }()
}
```

- [ ] **Step 2: Update verify/reset call sites to pass `currentLang(c)`**

In the same file:
- `postRegister` — the `sendVerifyEmail(email, verifyToken)` call becomes
  `s.sendVerifyEmail(email, verifyToken, currentLang(c))`.
- `postResendVerify` — `s.sendVerifyEmail(email, token)` becomes
  `s.sendVerifyEmail(email, token, currentLang(c))`.
- `postForgot` — `s.sendResetEmail(email, token)` becomes
  `s.sendResetEmail(email, token, currentLang(c))`.

(The two `sendInviteEmail` call sites live in `invitations.go`, handled in Task 5.)

- [ ] **Step 3: Build (invitations.go callers still break — expected)**

Run: `go build ./... 2>&1 | head`
Expected: remaining errors only in `internal/server/invitations.go` (old
`sendInviteEmail`/`CreateInvitation` arities). Fixed in Task 5.

- [ ] **Step 4: Checkpoint commit**

```bash
git add internal/server/auth_handlers.go
git commit -m "feat(email): send verify/reset emails in the request's language"
```

---

## Task 4: Per-user UI language persistence (middleware.go)

**Files:**
- Modify: `internal/server/middleware.go`
- Test: `internal/server/lang_test.go` (create)

- [ ] **Step 1: Write failing tests for resolution + persistence**

Create `internal/server/lang_test.go` (complete file):

```go
package server

import (
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v2"

	"note-aura/internal/db"
)

func bodyOf(t *testing.T, resp *http.Response) string {
	t.Helper()
	b, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read body: %v", err)
	}
	return string(b)
}

// detectLang precedence: cookie > stored user setting > Accept-Language > default.
func TestDetectLangPrecedence(t *testing.T) {
	database, err := db.Open(":memory:")
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { database.Close() })
	s := &Server{db: database}

	uid, _ := s.db.CreateUser("u@example.com", "h", false, true, "")
	_ = s.db.SetUserSetting(uid, "lang", "ja")
	user, _ := s.db.GetUser(uid)

	app := fiber.New()
	app.Use(func(c *fiber.Ctx) error {
		c.Locals(userLocalKey, user) // simulate loadSession having run
		return c.Next()
	})
	app.Use(s.detectLang)
	app.Get("/", func(c *fiber.Ctx) error { return c.SendString(currentLang(c)) })

	// Stored ja used when no cookie and no Accept-Language.
	req := httptest.NewRequest("GET", "/", nil)
	resp, _ := app.Test(req)
	if got := bodyOf(t, resp); got != "ja" {
		t.Errorf("stored-setting lang = %q, want ja", got)
	}

	// Cookie overrides stored setting.
	req = httptest.NewRequest("GET", "/", nil)
	req.Header.Set("Cookie", "lang=zh-Hant")
	resp, _ = app.Test(req)
	if got := bodyOf(t, resp); got != "zh-Hant" {
		t.Errorf("cookie lang = %q, want zh-Hant", got)
	}
}
```

Note: the `Server` struct fields are unexported but this test lives in `package server`,
so `&Server{db: database}` is valid. `CreateUser`'s signature is
`CreateUser(email, hash string, isAdmin, verified bool, verifyToken string)`.

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/server/ -run TestDetectLangPrecedence -v`
Expected: FAIL — current `detectLang` ignores the stored user setting (returns default
`en`, not `ja`).

- [ ] **Step 3: Update `detectLang` to consult the stored user setting**

In `internal/server/middleware.go`, replace `detectLang`:

```go
// detectLang resolves the UI language into c.Locals for templates. Precedence:
// explicit lang cookie → logged-in user's saved preference → Accept-Language → default.
func (s *Server) detectLang(c *fiber.Ctx) error {
	lang := ""
	if ck := c.Cookies("lang"); i18n.Supported(ck) {
		lang = ck
	}
	if lang == "" {
		if u := currentUser(c); u != nil {
			if settings, err := s.db.GetUserSettings(u.ID); err == nil && i18n.Supported(settings["lang"]) {
				lang = settings["lang"]
			}
		}
	}
	if lang == "" {
		lang = i18n.Match(c.Get("Accept-Language"))
	}
	c.Locals("lang", lang)
	return c.Next()
}
```

- [ ] **Step 4: Persist the choice in `setLang` when authenticated**

In `internal/server/middleware.go`, update `setLang` to also write the user setting:

```go
// setLang stores a language cookie (and the user's saved preference when logged
// in) and returns to the previous page.
func (s *Server) setLang(c *fiber.Ctx) error {
	if code := c.Params("code"); i18n.Supported(code) {
		c.Cookie(&fiber.Cookie{
			Name: "lang", Value: code, Path: "/",
			Expires: time.Now().AddDate(1, 0, 0), SameSite: "Lax",
		})
		if u := currentUser(c); u != nil {
			_ = s.db.SetUserSetting(u.ID, "lang", code)
		}
	}
	ref := c.Get("Referer")
	if ref == "" {
		ref = "/"
	}
	return c.Redirect(ref, fiber.StatusFound)
}
```

- [ ] **Step 5: Run test to verify it passes**

Run: `go test ./internal/server/ -run TestDetectLangPrecedence -v`
Expected: PASS

- [ ] **Step 6: Checkpoint commit**

```bash
git add internal/server/middleware.go internal/server/lang_test.go
git commit -m "feat(i18n): persist and prefer each user's saved UI language"
```

---

## Task 5: Capture & store invitation language (invitations.go + invite form)

**Files:**
- Modify: `internal/server/invitations.go`
- Modify: `web/templates/settings.html` (invite form, ~lines 144-146)

- [ ] **Step 1: Add the language dropdown to the invite form**

In `web/templates/settings.html`, replace the invite form (currently lines 144-146):

```html
  <form method="post" action="/invite" class="flex flex-wrap items-center gap-2">
    <input name="email" type="email" placeholder="{{t .Lang "invite.placeholder"}}" required class="border rounded px-3 py-1.5 text-sm flex-1 min-w-[200px]">
    <select name="lang" title="{{t .Lang "invite.lang"}}" class="border rounded px-2 py-1.5 text-sm bg-white text-neutral-700">
      {{range langs}}<option value="{{.Code}}" {{if eq .Code $.Lang}}selected{{end}}>{{.Native}}</option>{{end}}
    </select>
    <button class="bg-indigo-600 text-white rounded px-3 py-1.5 text-sm">{{t .Lang "invite.send"}}</button>
  </form>
```

(The `invite.*` keys are added in Task 6 with the rest of `settings.html`.)

- [ ] **Step 2: Capture/validate/store the language in `inviteUser`**

In `internal/server/invitations.go`, add `"note-aura/internal/i18n"` to imports. In
`inviteUser`, after the `email`/exists checks and before `token, err := auth.NewToken()`,
resolve the language; then store and send with it. Replace the tail of `inviteUser`:

```go
	lang := strings.TrimSpace(c.FormValue("lang"))
	if !i18n.Supported(lang) {
		lang = currentLang(c)
	}
	token, err := auth.NewToken()
	if err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, "token error")
	}
	if err := s.db.CreateInvitation(u.ID, email, token, lang); err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, err.Error())
	}
	s.sendInviteEmail(email, u.Email, token, lang)
	return c.Redirect("/settings?isent=1", fiber.StatusFound)
```

- [ ] **Step 3: Resend uses the stored language**

In `resendInvitation`:

```go
	if inv, err := s.db.GetInvitation(id); err == nil && inv.InviterID == u.ID && !inv.Accepted {
		s.sendInviteEmail(inv.Email, u.Email, inv.Token, inv.Lang)
	}
```

In `adminResendInvitation`:

```go
	if inv, err := s.db.GetInvitation(id); err == nil && !inv.Accepted {
		inviter := ""
		if iu, e := s.db.GetUser(inv.InviterID); e == nil {
			inviter = iu.Email
		}
		s.sendInviteEmail(inv.Email, inviter, inv.Token, inv.Lang)
	}
```

- [ ] **Step 4: Build the whole project — must be green now**

Run: `go build ./...`
Expected: clean (Tasks 1, 3, 5 together resolve all signature changes).

- [ ] **Step 5: Run the full test suite**

Run: `go test ./...`
Expected: PASS across all packages.

- [ ] **Step 6: Checkpoint commit**

```bash
git add internal/server/invitations.go web/templates/settings.html
git commit -m "feat(invite): inviter-chosen email language, reused on resend"
```

---

## Task 6: Translate templates — worked procedure + per-template tasks

**The procedure (apply to each template task below):**

1. Open the template. Identify every **user-facing literal string** (visible text,
   `placeholder=`, `title=`, `value=` on buttons, `confirm('…')` messages). Skip code,
   URLs, class names, template variables, and data values.
2. For each string, choose a key in the page's namespace (e.g. `settings.export`,
   `users.suspend`). Reuse an existing key when the exact English text already exists
   (e.g. `action.save`, `action.delete`, `action.cancel`, `nav.*`).
3. In `internal/i18n/translations.go`, add the key to **all four** language maps. `en`
   value = the verbatim original string. Author `zh-Hant`; author `zh-Hans`/`ja`
   best-effort.
4. In the template, replace the string with `{{t .Lang "key"}}`. For attributes:
   `placeholder="{{t .Lang "key"}}"`. For JS confirm dialogs, replace the literal with
   the rendered key (acceptable since templates render server-side):
   `onsubmit="return confirm('{{t .Lang "key"}}')"`.
5. After each template: `go build ./...` then `go test ./internal/i18n/ -run TestLanguageKeyParity -v`
   (must stay green — proves every new key exists in all 4 languages).
6. Commit.

**Reference — keys that already exist (reuse, don't duplicate):** `action.save`,
`action.cancel`, `action.edit`, `action.delete`, `action.share`, `action.back`,
`nav.notes`, `nav.dashboard`, `nav.users`, `nav.logs`, `nav.calendar`, `nav.ask`,
`nav.shared`, `nav.groups`, `nav.settings`, `nav.admin`, `nav.addnote`, `nav.signout`,
`auth.email`, `auth.password`, `auth.signin`, `auth.createaccount`, `settings.title`,
`calendar.today`, `ask.title`, `summary.lang`, `summary.auto`, `lang.label`.

### Task 6a: `settings.html` (largest; includes invite keys)

**Files:** Modify `web/templates/settings.html`, `internal/i18n/translations.go`

- [ ] **Step 1:** Apply the procedure to `web/templates/settings.html`. This includes the
  `invite.*` keys referenced by Task 5's dropdown. At minimum author these keys (en
  verbatim from the template; add to all 4 langs):
  `invite.placeholder` ("Invite by email…"), `invite.lang` ("Invitation language"),
  `invite.send` ("Send invite"), `invite.resend` ("Resend"), plus `settings.*` keys for
  the AI settings, export/import, email-address, block, and holidays sections, and the
  `block.*` keys ("Block user by email…", "Block", "Unblock"). Reuse `action.save` for
  every "Save" button.
- [ ] **Step 2:** `go build ./...` → clean.
- [ ] **Step 3:** `go test ./internal/i18n/ -run TestLanguageKeyParity -v` → PASS.
- [ ] **Step 4:** Manually scan the rendered file: no remaining bare English in visible
  text/placeholders/buttons. Commit:

```bash
git add web/templates/settings.html internal/i18n/translations.go
git commit -m "i18n(settings): translate settings page + invite language picker"
```

### Task 6b: `admin.html` + `users.html` + `logs.html` (admin pages)

**Files:** Modify those 3 templates + `internal/i18n/translations.go`

- [ ] **Step 1:** Apply the procedure to all three, using `admin.*`, `users.*`, `logs.*`
  namespaces (reuse `nav.*`/`action.*` where text matches).
- [ ] **Step 2:** `go build ./...` → clean; `go test ./internal/i18n/ -run TestLanguageKeyParity -v` → PASS.
- [ ] **Step 3:** Commit:

```bash
git add web/templates/admin.html web/templates/users.html web/templates/logs.html internal/i18n/translations.go
git commit -m "i18n(admin): translate admin, users, and logs pages"
```

### Task 6c: `dashboard.html` + `groups.html` + `group.html`

**Files:** Modify those 3 templates + `internal/i18n/translations.go`

- [ ] **Step 1:** Apply the procedure using `dashboard.*`, `groups.*`, `group.*`.
- [ ] **Step 2:** `go build ./...` → clean; `go test ./internal/i18n/ -run TestLanguageKeyParity -v` → PASS.
- [ ] **Step 3:** Commit:

```bash
git add web/templates/dashboard.html web/templates/groups.html web/templates/group.html internal/i18n/translations.go
git commit -m "i18n(groups): translate dashboard and group pages"
```

### Task 6d: `note_view.html` + finish `note_edit.html` + `notes_list.html`

**Files:** Modify those 3 templates + `internal/i18n/translations.go`

- [ ] **Step 1:** Apply the procedure using `note.*` (and existing `notes.*`). Finish the
  partially-translated `note_edit.html` and `notes_list.html`.
- [ ] **Step 2:** `go build ./...` → clean; `go test ./internal/i18n/ -run TestLanguageKeyParity -v` → PASS.
- [ ] **Step 3:** Commit:

```bash
git add web/templates/note_view.html web/templates/note_edit.html web/templates/notes_list.html internal/i18n/translations.go
git commit -m "i18n(notes): translate note view/edit/list"
```

### Task 6e: Auth/misc — `reset.html`, `forgot.html`, `ask.html`, `calendar.html`, finish `login.html`/`register.html`/`layout.html`

**Files:** Modify those templates + `internal/i18n/translations.go`

- [ ] **Step 1:** Apply the procedure using `reset.*`, `forgot.*`, `ask.*`, `calendar.*`
  (reuse `auth.*`). Sweep `login.html`, `register.html`, `layout.html` for any remaining
  bare strings.
- [ ] **Step 2:** `go build ./...` → clean; `go test ./internal/i18n/ -run TestLanguageKeyParity -v` → PASS.
- [ ] **Step 3:** Commit:

```bash
git add web/templates/reset.html web/templates/forgot.html web/templates/ask.html web/templates/calendar.html web/templates/login.html web/templates/register.html web/templates/layout.html internal/i18n/translations.go
git commit -m "i18n(auth): translate reset/forgot/ask/calendar and finish auth pages"
```

---

## Task 7: End-to-end manual smoke test

**Files:** none (verification only). Uses the same throwaway-config pattern; never touch
`data/note-aura.db`.

- [ ] **Step 1: Build and launch on a test config**

```bash
cd /c/Project/Note-Aura
go build -o note-aura-test.exe .
# Create a temp config with a seeded admin, dead Ollama, no SMTP, test port 8099,
# temp db_path/uploads_dir, then:
./note-aura-test.exe -config /tmp/na-smoke.yaml &
```

- [ ] **Step 2: Verify each language renders**

For each `code` in `en`, `zh-Hant`, `zh-Hans`, `ja`: set the `lang` cookie (e.g.
`curl -s --cookie "lang=zh-Hant" http://127.0.0.1:8099/login`) and confirm the page
contains the translated tagline/labels, not English. Spot-check `/login` and (after
login) `/settings`, `/calendar`, `/ask`.
Expected: visible text changes per language; no `{{t ...}}` literals leak into output.

- [ ] **Step 3: Verify language persistence**

Log in, `GET /lang/ja`, then in a *fresh* request with the session cookie but **no**
`lang` cookie, `GET /settings`; confirm Japanese (proves `user_settings` persistence via
`detectLang`).
Expected: Japanese without a `lang` cookie.

- [ ] **Step 4: Verify invitation language is stored**

`POST /invite` with `email=x@example.com&lang=ja` (SMTP off, so no mail sent), then query
the temp DB: `sqlite3 <tempdb> "SELECT email, lang FROM user_invitations;"`.
Expected: row shows `lang = ja`.

- [ ] **Step 5: Stop the server and clean up**

```bash
# kill the background note-aura-test.exe; remove note-aura-test.exe and temp config/db
```

- [ ] **Step 6: Full suite green**

Run: `go test ./... && go build ./...`
Expected: PASS + clean build.

---

## Task 8: Update documentation

**Files:** `README.md`, `INSTALL.md`, `USER_GUIDE.md`, `USER_GUIDE.zh-Hant.md`

- [ ] **Step 1: README.md** — add a "Languages / 多語" note: UI available in English,
  繁體中文, 简体中文, 日本語; switch via the header selector; the choice is remembered on
  your account; transactional emails follow the relevant person's language.

- [ ] **Step 2: INSTALL.md** — note that no configuration is required to enable
  languages; document that invitation/verification/password-reset emails are sent in the
  user's UI language (invitations: inviter-selected).

- [ ] **Step 3: USER_GUIDE.md** — add a "Changing your language" section (header
  selector, remembered per account) and, in the invitations section, document the
  per-invitation language dropdown.

- [ ] **Step 4: USER_GUIDE.zh-Hant.md** — mirror the Step 3 additions in Traditional
  Chinese.

- [ ] **Step 5: Commit**

```bash
git add README.md INSTALL.md USER_GUIDE.md USER_GUIDE.zh-Hant.md
git commit -m "docs: document multilingual UI and email language"
```

---

## Self-Review (completed by plan author)

- **Spec coverage:** §1 per-user persistence → Task 4. §2 email mechanism → Tasks 2-3.
  §3 email plumbing → Tasks 3, 5. §4 data model → Task 1. §5 UI coverage (all templates)
  → Task 6a-6e. §6 invite dropdown → Task 5 (markup) + 6a (keys). §7 extensibility →
  enforced by `TestLanguageKeyParity` (Task 2) iterating `i18n.Languages`. Testing →
  Tasks 1,2,4 (unit) + Task 7 (manual). Docs → Task 8. No gaps.
- **Signature consistency:** `CreateInvitation(inviterID, email, token, lang)` defined in
  Task 1, called in Task 5. `sendInviteEmail(to, inviter, token, lang)`,
  `sendVerifyEmail(to, token, lang)`, `sendResetEmail(to, token, lang)` defined in Task 3,
  called in Tasks 3 (verify/reset) and 5 (invite). `Invitation.Lang` defined Task 1, read
  Task 5. Consistent.
- **Placeholder scan:** No stubs/TBDs. Task 6 uses a documented repeatable procedure with the parity test as the guardrail
  rather than transcribing ~200 strings × 4 languages — the structural change and key
  conventions are fully specified, and `en` values are defined as the verbatim source
  strings.
