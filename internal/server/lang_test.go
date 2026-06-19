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

func TestSafeReferer(t *testing.T) {
	cases := map[string]string{
		"":                               "/",
		"/settings":                      "/settings",
		"/notes?q=x":                     "/notes?q=x",
		"https://evil.com/x":             "/",
		"//evil.com":                     "/",
		"http://localhost:8000/settings": "/",
	}
	for in, want := range cases {
		if got := safeReferer(in); got != want {
			t.Errorf("safeReferer(%q) = %q, want %q", in, got, want)
		}
	}
}
