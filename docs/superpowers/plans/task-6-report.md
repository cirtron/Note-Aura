# Task 6 Report — Admin `facebook.cookies` Setting

## Files Changed

### `internal/server/admin.go`
- **`getAdmin`** (~line 153, after `m["ModelChat"]`): added `m["FacebookCookies"] = app["facebook.cookies"]`
- **`postAdmin`** (~line 281, after the YT prompt `set(...)` calls): added `set("facebook.cookies", strings.TrimSpace(c.FormValue("facebook_cookies")))`

### `web/templates/admin.html`
- Added a new `<section>` block with the Facebook cookies textarea immediately before the `<button class="bg-indigo-600 ...">{{t .Lang "admin.ai.savesettings"}}</button>` save button (line 226 in the original, now ~line 232). The textarea has `name="facebook_cookies"` and renders `{{.FacebookCookies}}`. Inserted near the end of the YouTube prompts section (after line 224 of the original file, which closed the YouTube prompts `</section>`).

## Build Result

`go build ./...` — **CLEAN** (no output, exit 0)

## Grep Confirmation

```
web/templates/admin.html:230:    <textarea name="facebook_cookies" rows="4" class="w-full border rounded px-2 py-1 text-xs font-mono">{{.FacebookCookies}}</textarea>
```

## Concerns

None. `strings` was already imported in `admin.go`; no new imports required. The textarea is inside the existing `<form method="post" action="/admin">` so it is saved by `postAdmin` on submit. Task 7 can read the value via `app["facebook.cookies"]` from `GetAppSettings()`.
