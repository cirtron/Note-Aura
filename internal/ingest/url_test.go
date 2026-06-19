package ingest

import (
	"strings"
	"testing"
)

func TestExtractJSONLD_object(t *testing.T) {
	html := `<html><head>
		<script type="application/ld+json">
		{"@context":"https://schema.org","@type":"NewsArticle",
		 "headline":"Big News Today","articleBody":"This is the full article body text that is quite long."}
		</script></head><body></body></html>`
	body, headline := extractJSONLD(html)
	if !strings.Contains(body, "full article body text") {
		t.Errorf("articleBody = %q", body)
	}
	if headline != "Big News Today" {
		t.Errorf("headline = %q", headline)
	}
}

func TestExtractJSONLD_graph(t *testing.T) {
	html := `<script type="application/ld+json">
		{"@graph":[{"@type":"WebPage"},{"@type":"Article","articleBody":"Nested graph article body."}]}
		</script>`
	body, _ := extractJSONLD(html)
	if !strings.Contains(body, "Nested graph article body") {
		t.Errorf("articleBody = %q", body)
	}
}

func TestBuildFetched_prefersJSONLDWhenBodyThin(t *testing.T) {
	long := strings.Repeat("Real article sentence. ", 60) // ~1380 chars
	html := `<html><head>
		<meta name="description" content="short teaser">
		<title>Page Title</title>
		<script type="application/ld+json">{"@type":"Article","articleBody":"` + long + `"}</script>
		</head><body><div id="root"></div></body></html>`
	f := buildFetched([]byte(html))
	if f == nil {
		t.Fatal("buildFetched returned nil")
	}
	if !strings.Contains(f.Text, "Real article sentence") || len([]rune(f.Text)) < 1000 {
		t.Errorf("expected full JSON-LD body, got %d chars: %.80q", len([]rune(f.Text)), f.Text)
	}
}
