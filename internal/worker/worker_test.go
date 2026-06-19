package worker

import "testing"

func TestParseParts(t *testing.T) {
	// Empty => all fields (first-time processing).
	all := parseParts("")
	if !all["title"] || !all["summary"] || !all["tags"] {
		t.Errorf("empty params should select all, got %v", all)
	}
	// Subset only.
	m := parseParts("title")
	if !m["title"] || m["summary"] || m["tags"] {
		t.Errorf("\"title\" should select only title, got %v", m)
	}
	m = parseParts("summary, tags")
	if m["title"] || !m["summary"] || !m["tags"] {
		t.Errorf("\"summary, tags\" should select summary+tags, got %v", m)
	}
}
