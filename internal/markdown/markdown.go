// Package markdown renders note bodies (stored as Markdown) into sanitized HTML
// for display. goldmark does the Markdown→HTML conversion; bluemonday strips any
// unsafe markup, closing the stored-XSS surface for shared notes.
package markdown

import (
	"bytes"
	"html/template"
	"sync"

	"github.com/microcosm-cc/bluemonday"
	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/extension"
	ghtml "github.com/yuin/goldmark/renderer/html"
)

var (
	md = goldmark.New(
		goldmark.WithExtensions(extension.GFM), // tables, strikethrough, autolinks, task lists
		goldmark.WithRendererOptions(ghtml.WithHardWraps()),
	)
	policy     *bluemonday.Policy
	policyOnce sync.Once
)

func sanitizer() *bluemonday.Policy {
	policyOnce.Do(func() {
		p := bluemonday.UGCPolicy()
		p.AllowAttrs("class").OnElements("code", "span", "div", "pre", "li")
		p.AllowAttrs("type", "checked", "disabled").OnElements("input") // GFM task lists
		policy = p
	})
	return policy
}

// Render converts Markdown source to sanitized, display-ready HTML.
func Render(src string) template.HTML {
	var buf bytes.Buffer
	if err := md.Convert([]byte(src), &buf); err != nil {
		// Fall back to escaped plain text on the (unlikely) conversion error.
		return template.HTML(template.HTMLEscapeString(src))
	}
	safe := sanitizer().SanitizeBytes(buf.Bytes())
	return template.HTML(safe)
}
