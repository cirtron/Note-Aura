package ingest

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"golang.org/x/net/html"
)

// thinTextThreshold is the rune count below which an HTTP-extracted page is
// treated as "thin" (likely a JavaScript shell), triggering the headless render.
const thinTextThreshold = 600

// Fetched is the result of pulling a web page.
type Fetched struct {
	Title string
	Text  string
}

const chromeUA = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/124.0 Safari/537.36"

// requestProfiles are tried in order. Some sites' bot filters reject one header
// profile (400/403) but accept another, so we fall back rather than give up.
var requestProfiles = []map[string]string{
	{
		"User-Agent":      chromeUA,
		"Accept":          "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8",
		"Accept-Language": "en,zh;q=0.8",
	},
	{ // A plain, identifiable bot UA — some WAFs allow this where they block a fake browser.
		"User-Agent": "Note-Aura/1.0 (+https://note-aura.local)",
		"Accept":     "*/*",
	},
}

// FetchURL downloads a web page and extracts its title and readable text. It
// tolerates non-200 responses (many sites return 400/403 to automated fetches
// yet still include usable HTML) and JavaScript-rendered shells (falling back to
// the meta description / Open Graph tags).
func FetchURL(ctx context.Context, rawURL string) (*Fetched, error) {
	var lastStatus int
	var lastErr error
	for _, headers := range requestProfiles {
		raw, status, err := httpGet(ctx, rawURL, headers)
		if err != nil {
			lastErr = err
			continue
		}
		lastStatus = status
		if f := buildFetched(raw); f != nil {
			// Thin result (likely JS-rendered) → render with a headless browser
			// and keep whichever yields more text. No-op when headless is off or
			// Chrome isn't available.
			if EnableHeadless && len([]rune(f.Text)) < thinTextThreshold {
				if hf := fetchHeadless(ctx, rawURL); hf != nil && len([]rune(hf.Text)) > len([]rune(f.Text)) {
					return hf, nil
				}
			}
			return f, nil
		}
	}
	// Nothing extractable from the raw HTML at all — try a headless render.
	if EnableHeadless {
		if hf := fetchHeadless(ctx, rawURL); hf != nil {
			return hf, nil
		}
	}
	if lastStatus == 0 && lastErr != nil {
		return nil, fmt.Errorf("fetch url: %w", lastErr)
	}
	return nil, fmt.Errorf("fetch url: server returned status %d with no extractable text "+
		"(the site may block automated requests or require JavaScript)", lastStatus)
}

// httpGet performs a GET and returns the body bytes regardless of status code.
func httpGet(ctx context.Context, rawURL string, headers map[string]string) ([]byte, int, error) {
	client := &http.Client{Timeout: 30 * time.Second}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, 0, err
	}
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, 0, err
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(resp.Body, 20<<20)) // 20 MB cap
	if err != nil {
		return nil, resp.StatusCode, err
	}
	return raw, resp.StatusCode, nil
}

// buildFetched extracts content from HTML bytes, returning nil if nothing usable
// was found.
func buildFetched(raw []byte) *Fetched {
	htmlStr := string(raw)
	title, description := extractMeta(htmlStr)
	body := HTMLToText(htmlStr)

	// Many JS-rendered article/news pages ship the full text in JSON-LD
	// structured data even when the rendered HTML body is thin — prefer whichever
	// is richer (longer).
	jsonBody, jsonTitle := extractJSONLD(htmlStr)
	if len([]rune(jsonBody)) > len([]rune(body)) {
		body = jsonBody
	}
	if title == "" {
		title = jsonTitle
	}

	// Prefer the full body; when it's thin (JS shell, paywall teaser, error
	// page) lead with the meta description so we still capture a few sentences.
	text := strings.TrimSpace(body)
	if len(text) < 160 && description != "" {
		text = strings.TrimSpace(description + "\n\n" + body)
	}
	if text == "" {
		text = strings.TrimSpace(title + "\n" + description)
	}
	if text == "" {
		return nil
	}
	return &Fetched{Title: title, Text: text}
}

// extractJSONLD scans <script type="application/ld+json"> blocks for an article
// body and headline (handling single objects, arrays, and @graph nesting).
func extractJSONLD(htmlStr string) (articleBody, headline string) {
	doc, err := html.Parse(strings.NewReader(htmlStr))
	if err != nil {
		return "", ""
	}
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if n.Type == html.ElementNode && n.Data == "script" &&
			strings.EqualFold(attr(n, "type"), "application/ld+json") &&
			n.FirstChild != nil {
			ab, hl := scanJSONLD(n.FirstChild.Data)
			if len([]rune(ab)) > len([]rune(articleBody)) {
				articleBody = ab
			}
			if headline == "" {
				headline = hl
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(doc)
	return strings.TrimSpace(articleBody), strings.TrimSpace(headline)
}

// scanJSONLD parses one ld+json payload and returns the longest articleBody and
// first headline found anywhere within it.
func scanJSONLD(raw string) (articleBody, headline string) {
	var v any
	if err := json.Unmarshal([]byte(strings.TrimSpace(raw)), &v); err != nil {
		return "", ""
	}
	var rec func(any)
	rec = func(x any) {
		switch t := x.(type) {
		case map[string]any:
			if s, ok := t["articleBody"].(string); ok && len([]rune(s)) > len([]rune(articleBody)) {
				articleBody = s
			}
			if headline == "" {
				if s, ok := t["headline"].(string); ok {
					headline = s
				}
			}
			for _, val := range t {
				rec(val)
			}
		case []any:
			for _, val := range t {
				rec(val)
			}
		}
	}
	rec(v)
	return articleBody, headline
}

// attr returns an element's attribute value (empty when absent).
func attr(n *html.Node, key string) string {
	for _, a := range n.Attr {
		if strings.EqualFold(a.Key, key) {
			return a.Val
		}
	}
	return ""
}
