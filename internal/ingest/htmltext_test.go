package ingest

import "testing"

func TestHTMLToTextKeepsAllRegions(t *testing.T) {
	html := `<!doctype html><html><head><title>T</title>
<style>.x{color:red}</style><script>var a=1;</script></head>
<body>
  <header><h1>Site Header Title</h1><nav><a href="/">Home</a><a href="/about">About</a></nav></header>
  <main>
    <article><p>First paragraph of the <a href="#">article body</a> here.</p>
    <p>Second paragraph with detail.</p></article>
    <aside>Sidebar related note.</aside>
  </main>
  <footer>Footer copyright text.</footer>
  <script>var b=2;</script>
</body></html>`

	got := HTMLToText(html)

	// All content regions must be present.
	for _, want := range []string{
		"Site Header Title", "Home", "About",
		"First paragraph of the article body here.",
		"Second paragraph with detail.",
		"Sidebar related note.", "Footer copyright text.",
	} {
		if !contains(got, want) {
			t.Errorf("expected extracted text to contain %q\n--- got ---\n%s", want, got)
		}
	}
	// Non-content must be dropped.
	for _, bad := range []string{"var a=1", "var b=2", "color:red"} {
		if contains(got, bad) {
			t.Errorf("expected %q to be stripped\n--- got ---\n%s", bad, got)
		}
	}
}

func TestExtractMetaFallback(t *testing.T) {
	// A JavaScript-rendered shell: no body text, only meta tags.
	shell := `<!doctype html><html><head>
<title>My Page Title</title>
<meta name="description" content="A short description with a few sentences about the page.">
<meta property="og:title" content="OG Title">
</head><body><div id="root"></div><script>render()</script></body></html>`

	title, desc := extractMeta(shell)
	if title != "My Page Title" {
		t.Errorf("title = %q, want %q", title, "My Page Title")
	}
	if !contains(desc, "few sentences about the page") {
		t.Errorf("description = %q, want it to contain the meta description", desc)
	}

	// Body extraction is empty for the shell, confirming the fallback matters.
	if body := HTMLToText(shell); contains(body, "few sentences") {
		t.Errorf("shell body unexpectedly contained description: %q", body)
	}
}

func contains(s, sub string) bool {
	return len(sub) == 0 || (len(s) >= len(sub) && indexOf(s, sub) >= 0)
}

func indexOf(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}
