// Package ingest turns external sources (rich-text HTML, web URLs, YouTube
// links, images) into plain text for the AI pipeline.
package ingest

import (
	"strings"

	"golang.org/x/net/html"
)

// skipTags are elements that never carry readable content. Everything else —
// including header / footer / nav / aside / main / article — is kept, so a note
// captured from a website preserves ALL of the page's text.
var skipTags = map[string]bool{
	"script": true, "style": true, "noscript": true, "svg": true,
	"template": true, "iframe": true, "head": true, "meta": true,
	"link": true, "object": true, "embed": true, "canvas": true,
}

// blockTags trigger a newline so paragraphs/sections don't run together.
var blockTags = map[string]bool{
	"p": true, "div": true, "br": true, "li": true, "tr": true, "td": true, "th": true,
	"h1": true, "h2": true, "h3": true, "h4": true, "h5": true, "h6": true,
	"section": true, "article": true, "blockquote": true, "pre": true, "hr": true,
	"main": true, "header": true, "footer": true, "nav": true, "aside": true,
	"ul": true, "ol": true, "table": true, "figure": true, "figcaption": true,
	"dt": true, "dd": true,
}

// HTMLToText extracts the full plain text from an HTML fragment or document,
// dropping only non-content tags. Returns the original string
// (whitespace-collapsed) if it doesn't parse.
func HTMLToText(htmlStr string) string {
	doc, err := html.Parse(strings.NewReader(htmlStr))
	if err != nil {
		return collapse(htmlStr)
	}
	var sb strings.Builder
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if n.Type == html.ElementNode && skipTags[n.Data] {
			return
		}
		if n.Type == html.TextNode {
			sb.WriteString(n.Data)
			// Separate adjacent inline text so words don't merge (e.g. links).
			sb.WriteByte(' ')
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
		if n.Type == html.ElementNode && blockTags[n.Data] {
			sb.WriteString("\n")
		}
	}
	walk(doc)
	return collapse(sb.String())
}

// extractMeta pulls the page title and a description from <title>, and the
// meta description / Open Graph tags — used as a fallback when a page's body has
// little extractable text.
func extractMeta(htmlStr string) (title, description string) {
	doc, err := html.Parse(strings.NewReader(htmlStr))
	if err != nil {
		return "", ""
	}
	var titleTag, ogTitle, metaDesc, ogDesc string
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if n.Type == html.ElementNode {
			switch n.Data {
			case "title":
				if titleTag == "" && n.FirstChild != nil && n.FirstChild.Type == html.TextNode {
					titleTag = strings.TrimSpace(n.FirstChild.Data)
				}
			case "meta":
				var name, prop, content string
				for _, a := range n.Attr {
					switch strings.ToLower(a.Key) {
					case "name":
						name = strings.ToLower(a.Val)
					case "property":
						prop = strings.ToLower(a.Val)
					case "content":
						content = strings.TrimSpace(a.Val)
					}
				}
				switch {
				case name == "description" && metaDesc == "":
					metaDesc = content
				case (prop == "og:description" || name == "og:description") && ogDesc == "":
					ogDesc = content
				case (prop == "og:title" || name == "og:title") && ogTitle == "":
					ogTitle = content
				}
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(doc)
	return firstNonEmpty(titleTag, ogTitle), firstNonEmpty(metaDesc, ogDesc)
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}

// collapse trims and squeezes runs of blank lines / spaces.
func collapse(s string) string {
	lines := strings.Split(s, "\n")
	var out []string
	for _, ln := range lines {
		ln = strings.Join(strings.Fields(ln), " ")
		if ln != "" {
			out = append(out, ln)
		}
	}
	return strings.TrimSpace(strings.Join(out, "\n"))
}
