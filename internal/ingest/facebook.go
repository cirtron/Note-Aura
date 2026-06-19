package ingest

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"strings"
)

// IsFacebook reports whether a URL points at Facebook content.
func IsFacebook(u string) bool {
	u = strings.ToLower(u)
	for _, h := range []string{"facebook.com/", "m.facebook.com/", "web.facebook.com/", "fb.com/", "fb.watch/"} {
		if strings.Contains(u, h) {
			return true
		}
	}
	return false
}

// validVTT reports whether s is a real WebVTT subtitle payload (starts with the
// WEBVTT signature). Guards against anti-bot HTML pages being stored as captions.
func validVTT(s string) bool {
	return strings.HasPrefix(strings.TrimSpace(s), "WEBVTT")
}

// FetchFacebook captures a Facebook URL's content, richest path first:
//  1. yt-dlp + cookies (videos/reels/watch) -> title, description, transcript
//  2. authenticated headless render (text posts) when headlessOK
//  3. plain Open-Graph fallback (works without cookies)
func FetchFacebook(ctx context.Context, rawURL, cookies string, headlessOK bool) (*Fetched, error) {
	// Path 1: yt-dlp with cookies.
	if f := fetchFacebookYtdlp(ctx, rawURL, cookies); f != nil {
		return f, nil
	}
	// Path 2: authenticated headless.
	if headlessOK && EnableHeadless {
		if f := fetchFacebookHeadless(ctx, rawURL, cookies); f != nil {
			return f, nil
		}
	}
	// Path 3: Open-Graph fallback.
	if f, err := FetchURL(ctx, rawURL); err == nil {
		return f, nil
	}
	return nil, fmt.Errorf("facebook: no content extractable for %s", rawURL)
}

// fetchFacebookYtdlp uses yt-dlp (with optional cookies) to pull metadata and any
// caption track. Returns nil if yt-dlp is unavailable or yields nothing.
func fetchFacebookYtdlp(ctx context.Context, rawURL, cookies string) *Fetched {
	if _, err := exec.LookPath("yt-dlp"); err != nil {
		return nil
	}
	cookieFile, cleanup := writeCookieFile(cookies)
	defer cleanup()

	args := []string{"--no-warnings", "--dump-json"}
	if cookieFile != "" {
		args = append(args, "--cookies", cookieFile)
	}
	args = append(args, rawURL)
	out, err := exec.CommandContext(ctx, "yt-dlp", args...).Output()
	if err != nil || len(out) == 0 {
		return nil
	}
	var m ytMeta
	if json.Unmarshal(out, &m) != nil {
		return nil
	}
	transcript := facebookTranscript(ctx, rawURL, cookieFile, m)
	body := composeYouTubeNote(m, transcript) // same metadata+description+transcript shape
	if strings.TrimSpace(body) == "" {
		return nil
	}
	return &Fetched{Title: strings.TrimSpace(m.Title), Text: body}
}

// facebookTranscript downloads the chosen caption track via yt-dlp and returns the
// cleaned text, only if it is a real VTT (rejects anti-bot HTML).
func facebookTranscript(ctx context.Context, rawURL, cookieFile string, m ytMeta) string {
	lang := pickCaptionLang(m)
	if lang == "" {
		return ""
	}
	args := []string{"--no-warnings", "--skip-download", "--write-auto-subs", "--write-subs",
		"--sub-langs", lang, "--sub-format", "vtt", "-o", "-"}
	if cookieFile != "" {
		args = append(args, "--cookies", cookieFile)
	}
	args = append(args, rawURL)
	out, err := exec.CommandContext(ctx, "yt-dlp", args...).Output()
	if err != nil || !validVTT(string(out)) {
		return ""
	}
	return cleanVTT(string(out))
}

// fetchFacebookHeadless renders the URL in headless Chrome with the supplied
// cookies injected, then extracts text. Returns nil on any failure.
func fetchFacebookHeadless(ctx context.Context, rawURL, cookies string) *Fetched {
	cs := parseNetscapeCookies(cookies)
	return fetchHeadlessWithCookies(ctx, rawURL, cs)
}

// writeCookieFile materializes cookies (Netscape format) to a temp file for
// yt-dlp's --cookies. Returns "" + a no-op cleanup when cookies is empty.
func writeCookieFile(cookies string) (path string, cleanup func()) {
	if strings.TrimSpace(cookies) == "" {
		return "", func() {}
	}
	f, err := os.CreateTemp("", "fbcookies-*.txt")
	if err != nil {
		return "", func() {}
	}
	_, _ = f.WriteString(cookies)
	_ = f.Close()
	return f.Name(), func() { _ = os.Remove(f.Name()) }
}

// parseNetscapeCookies parses a Netscape cookies.txt body into http.Cookies.
func parseNetscapeCookies(s string) []*http.Cookie {
	var out []*http.Cookie
	for _, line := range strings.Split(s, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		f := strings.Split(line, "\t")
		if len(f) < 7 {
			continue
		}
		out = append(out, &http.Cookie{Domain: f[0], Path: f[2], Name: f[5], Value: f[6]})
	}
	return out
}
