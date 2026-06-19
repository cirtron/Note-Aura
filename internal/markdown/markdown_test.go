package markdown

import (
	"strings"
	"testing"
)

func TestRenderMarkdown(t *testing.T) {
	out := string(Render("# Title\n\nSome **bold** and a list:\n\n- one\n- two\n"))
	for _, want := range []string{"<h1", "Title", "<strong>bold</strong>", "<li>one</li>", "<li>two</li>"} {
		if !strings.Contains(out, want) {
			t.Errorf("rendered output missing %q\n--- got ---\n%s", want, out)
		}
	}
}

func TestRenderImage(t *testing.T) {
	out := string(Render("text\n\n![alt text](/uploads/inline/5/abc.png)\n"))
	if !strings.Contains(out, "<img") || !strings.Contains(out, `src="/uploads/inline/5/abc.png"`) {
		t.Errorf("expected an <img> with the uploaded src\n--- got ---\n%s", out)
	}
}

func TestRenderSanitizesXSS(t *testing.T) {
	out := string(Render("Hello <script>alert('xss')</script> world\n\n[click](javascript:alert(1))"))
	if strings.Contains(out, "<script") || strings.Contains(strings.ToLower(out), "javascript:") {
		t.Errorf("unsafe content not sanitized:\n%s", out)
	}
	if !strings.Contains(out, "Hello") || !strings.Contains(out, "world") {
		t.Errorf("safe text was dropped:\n%s", out)
	}
}
