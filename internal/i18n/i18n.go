// Package i18n provides UI translations and language detection.
package i18n

import "strings"

// Language is a supported UI language.
type Language struct {
	Code    string // BCP-47-ish code used in cookies/URLs
	Native  string // display name in its own language
	English string // English name (used for AI summary-language prompts)
}

// Languages lists the supported languages (first = default).
var Languages = []Language{
	{"en", "English", "English"},
	{"zh-Hant", "繁體中文", "Traditional Chinese"},
	{"zh-Hans", "简体中文", "Simplified Chinese"},
	{"ja", "日本語", "Japanese"},
}

// Default is the fallback language code.
const Default = "en"

// Supported reports whether a code is a supported language.
func Supported(code string) bool {
	for _, l := range Languages {
		if l.Code == code {
			return true
		}
	}
	return false
}

// EnglishName returns the English name for a language code (for AI prompts), or
// "" for unknown/empty codes.
func EnglishName(code string) string {
	for _, l := range Languages {
		if l.Code == code {
			return l.English
		}
	}
	return ""
}

// T translates a key for a language, falling back to English then the key.
func T(lang, key string) string {
	if m, ok := translations[lang]; ok {
		if v, ok := m[key]; ok {
			return v
		}
	}
	if v, ok := translations[Default][key]; ok {
		return v
	}
	return key
}

// Match picks the best supported language from an Accept-Language header,
// returning Default when none matches.
func Match(acceptLanguage string) string {
	for _, part := range strings.Split(acceptLanguage, ",") {
		tag := strings.ToLower(strings.TrimSpace(strings.SplitN(part, ";", 2)[0]))
		switch {
		case tag == "":
			continue
		case strings.HasPrefix(tag, "zh"):
			if strings.Contains(tag, "tw") || strings.Contains(tag, "hk") || strings.Contains(tag, "hant") || strings.Contains(tag, "mo") {
				return "zh-Hant"
			}
			return "zh-Hans"
		case strings.HasPrefix(tag, "ja"):
			return "ja"
		case strings.HasPrefix(tag, "en"):
			return "en"
		}
	}
	return Default
}
