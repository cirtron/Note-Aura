package ingest

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
)

// IsYouTube reports whether a URL looks like a YouTube watch/short link.
func IsYouTube(u string) bool {
	u = strings.ToLower(u)
	return strings.Contains(u, "youtube.com/watch") ||
		strings.Contains(u, "youtu.be/") ||
		strings.Contains(u, "youtube.com/shorts/")
}

var ytSubLineRe = regexp.MustCompile(`^\d{2}:\d{2}:\d{2}\.\d{3} -->`)

// FetchYouTubeTranscript fetches a video's transcript using yt-dlp (which must
// be on PATH, mirroring the previous build's approach). It downloads auto/manual
// subtitles as a VTT file to stdout and strips the cue formatting.
//
// Captions in any language are accepted (English preferred). When the video has
// no captions at all, it falls back to the video description so the note still
// captures something. The returned text is a composed note (metadata header +
// description + transcript) so the captured note is substantive rather than a
// bare transcript. Returns the note text and the title.
func FetchYouTubeTranscript(ctx context.Context, url string) (text, title string, err error) {
	if _, lookErr := exec.LookPath("yt-dlp"); lookErr != nil {
		return "", "", fmt.Errorf("yt-dlp not found on PATH (required for YouTube ingest)")
	}

	// One metadata dump gives us title, description, channel/duration, and the
	// available caption languages — so we can pick a language that actually exists.
	meta := dumpMeta(ctx, url)
	title = strings.TrimSpace(meta.Title)

	transcript := fetchTranscript(ctx, url, meta)
	body := composeYouTubeNote(meta, transcript)
	if strings.TrimSpace(body) == "" {
		return "", title, fmt.Errorf("no transcript or description available for this video")
	}
	return body, title, nil
}

// fetchTranscript returns the cleaned transcript text, or "" when the video has
// no usable captions.
func fetchTranscript(ctx context.Context, url string, meta ytMeta) string {
	lang := pickCaptionLang(meta)
	if lang == "" {
		return ""
	}
	// Primary: have yt-dlp download the chosen caption track into a temp dir, then
	// read the .vtt file. NOTE: "-o -" does NOT pipe subtitles to stdout — yt-dlp
	// writes a file literally named "-.<lang>.vtt", so the old stdout capture always
	// came back empty (every video fell through to the metadata path) and littered
	// the working dir. A temp dir fixes both.
	if t := ytdlpSubsToTemp(ctx, url, lang); strings.TrimSpace(t) != "" {
		return t
	}
	// Secondary: pull the caption URL straight from the metadata and fetch it.
	if t, e := dumpJSONCaptions(ctx, url); e == nil && strings.TrimSpace(t) != "" {
		return t
	}
	return ""
}

// ytdlpSubsToTemp downloads the caption track for lang into a throwaway temp dir
// and returns the cleaned transcript (or "" on failure — e.g. an HTTP 429 from
// YouTube's caption endpoint, or no captions). Retries with a short sleep soften
// rate-limiting; the temp dir is removed so nothing litters the working dir.
func ytdlpSubsToTemp(ctx context.Context, url, lang string) string {
	dir, err := os.MkdirTemp("", "na-subs-")
	if err != nil {
		return ""
	}
	defer os.RemoveAll(dir)
	args := []string{
		"--no-warnings", "--skip-download",
		"--write-auto-subs", "--write-subs",
		"--sub-langs", lang,
		"--sub-format", "vtt",
		"--retries", "5", "--retry-sleep", "3",
		"-o", filepath.Join(dir, "%(id)s.%(ext)s"), url,
	}
	if err := exec.CommandContext(ctx, "yt-dlp", args...).Run(); err != nil {
		return "" // rate-limited or no captions; caller falls back
	}
	files, _ := filepath.Glob(filepath.Join(dir, "*.vtt"))
	for _, f := range files {
		if raw, rerr := os.ReadFile(f); rerr == nil {
			if t, ok := parseCaption(string(raw)); ok && strings.TrimSpace(t) != "" {
				return t
			}
		}
	}
	return ""
}

// composeYouTubeNote builds a rich Markdown note body from the video metadata
// and (optional) transcript, so a captured video is more than a bare transcript.
func composeYouTubeNote(m ytMeta, transcript string) string {
	var b strings.Builder

	var bits []string
	if u := strings.TrimSpace(m.Uploader); u != "" {
		bits = append(bits, "**Channel:** "+u)
	}
	if d := formatDuration(m.Duration); d != "" {
		bits = append(bits, "**Duration:** "+d)
	}
	if up := formatUploadDate(m.UploadDate); up != "" {
		bits = append(bits, "**Published:** "+up)
	}
	if len(bits) > 0 {
		b.WriteString(strings.Join(bits, " · "))
		b.WriteString("\n\n")
	}

	if desc := strings.TrimSpace(m.Description); desc != "" {
		b.WriteString("## Description\n\n")
		b.WriteString(desc)
		b.WriteString("\n\n")
	}

	if t := strings.TrimSpace(transcript); t != "" {
		b.WriteString("## Transcript\n\n")
		b.WriteString(t)
	}

	return strings.TrimSpace(b.String())
}

// formatDuration renders seconds as h:mm:ss or m:ss; "" for non-positive.
func formatDuration(sec float64) string {
	s := int(sec)
	if s <= 0 {
		return ""
	}
	h, m, ss := s/3600, (s%3600)/60, s%60
	if h > 0 {
		return fmt.Sprintf("%d:%02d:%02d", h, m, ss)
	}
	return fmt.Sprintf("%d:%02d", m, ss)
}

// formatUploadDate turns yt-dlp's "YYYYMMDD" into "YYYY-MM-DD"; passes other
// shapes through unchanged.
func formatUploadDate(d string) string {
	d = strings.TrimSpace(d)
	if len(d) == 8 {
		return d[0:4] + "-" + d[4:6] + "-" + d[6:8]
	}
	return d
}

// ytMeta is the subset of yt-dlp --dump-json we care about.
type ytMeta struct {
	Title             string                  `json:"title"`
	Description       string                  `json:"description"`
	Uploader          string                  `json:"uploader"`
	Duration          float64                 `json:"duration"`
	UploadDate        string                  `json:"upload_date"`
	AutomaticCaptions map[string][]captionTrk `json:"automatic_captions"`
	Subtitles         map[string][]captionTrk `json:"subtitles"`
}

type captionTrk struct {
	URL string `json:"url"`
	Ext string `json:"ext"`
}

// dumpMeta runs a single --dump-json and returns parsed metadata (best-effort:
// a zero value on any error).
func dumpMeta(ctx context.Context, url string) ytMeta {
	var m ytMeta
	out, err := exec.CommandContext(ctx, "yt-dlp", "--no-warnings", "--dump-json", url).Output()
	if err != nil {
		return m
	}
	_ = json.Unmarshal(out, &m)
	return m
}

// pickCaptionLang chooses the best available caption language code: an English
// track if present (manual preferred over auto), otherwise any available
// language (manual preferred). Returns "" when no captions exist.
func pickCaptionLang(m ytMeta) string {
	enOrAny := func(tracks map[string][]captionTrk) (en, any string) {
		for lang := range tracks {
			if len(tracks[lang]) == 0 {
				continue
			}
			if strings.HasPrefix(lang, "en") {
				en = lang
			}
			if any == "" || lang < any {
				any = lang // deterministic: lexicographically smallest
			}
		}
		return
	}
	if en, any := enOrAny(m.Subtitles); en != "" || any != "" {
		if en != "" {
			return en
		}
		return any
	}
	if en, any := enOrAny(m.AutomaticCaptions); en != "" || any != "" {
		if en != "" {
			return en
		}
		return any
	}
	return ""
}

// cleanVTT removes WEBVTT headers, timestamps, and duplicate lines.
func cleanVTT(s string) string {
	var lines []string
	seen := map[string]bool{}
	for _, ln := range strings.Split(s, "\n") {
		ln = strings.TrimSpace(ln)
		if ln == "" || ln == "WEBVTT" || strings.HasPrefix(ln, "Kind:") ||
			strings.HasPrefix(ln, "Language:") || ytSubLineRe.MatchString(ln) ||
			strings.Contains(ln, "-->") {
			continue
		}
		// strip inline tags like <00:00:00.000><c> ... </c>
		ln = stripTags(ln)
		if ln == "" || seen[ln] {
			continue
		}
		seen[ln] = true
		lines = append(lines, ln)
	}
	return strings.Join(lines, "\n")
}

var inlineTagRe = regexp.MustCompile(`<[^>]+>`)

func stripTags(s string) string { return strings.TrimSpace(inlineTagRe.ReplaceAllString(s, "")) }

// dumpJSONCaptions is a secondary path: ask yt-dlp for metadata and pull a
// caption-track URL (English preferred, otherwise any language), then fetch and
// clean it.
func dumpJSONCaptions(ctx context.Context, url string) (string, error) {
	meta := dumpMeta(ctx, url)
	// Prefer a vtt track in the picked language, else any vtt track in that map.
	pickURL := func(m map[string][]captionTrk, lang string) string {
		for _, t := range m[lang] {
			if t.Ext == "vtt" {
				return t.URL
			}
		}
		for _, tracks := range m { // fall back to any vtt track
			for _, t := range tracks {
				if t.Ext == "vtt" {
					return t.URL
				}
			}
		}
		return ""
	}
	lang := pickCaptionLang(meta)
	capURL := pickURL(meta.Subtitles, lang)
	if capURL == "" {
		capURL = pickURL(meta.AutomaticCaptions, lang)
	}
	if capURL == "" {
		return "", fmt.Errorf("no captions")
	}
	// Fetch the caption track raw — NOT through FetchURL, whose bot-tolerant HTML
	// extraction would happily turn Google's "Sorry / automated queries" anti-bot
	// page into "transcript" text. parseCaption requires a real WebVTT signature.
	raw, _, err := httpGet(ctx, capURL, requestProfiles[0])
	if err != nil {
		return "", err
	}
	if t, ok := parseCaption(string(raw)); ok {
		return t, nil
	}
	return "", fmt.Errorf("caption fetch returned non-subtitle content")
}

// parseCaption cleans a raw caption payload into transcript text, returning
// ok=false when the payload is not a real WebVTT track (e.g. an anti-bot HTML
// page served when YouTube/Google rate-limits the caption endpoint). This guard
// keeps junk like the "We're sorry … automated queries" page out of notes.
func parseCaption(raw string) (text string, ok bool) {
	if !validVTT(raw) {
		return "", false
	}
	return cleanVTT(raw), true
}
