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
	if got := T("zh-Hant", "nav.notes"); got == "" {
		t.Error("expected a value for nav.notes")
	}
	if got := T("zh-Hant", "totally.unknown.key"); got != "totally.unknown.key" {
		t.Errorf("unknown key should return itself, got %q", got)
	}
}
