package ai

import "testing"

func TestBuildProviderUserPrompts(t *testing.T) {
	global := GlobalConfig{
		OllamaURL: "http://x",
		Models:    Models{Chat: "m", Embed: "e", OCR: "v"},
		Prompts:   Prompts{Title: "GLOBAL-TITLE", Summary: "GLOBAL-SUM"},
	}
	cloud := map[string]string{
		KeyBaseURL:     "https://api.example/v1",
		KeyAPIKey:      "k",
		KeyPromptTitle: "MY-TITLE",
	}

	// Cloud + allowed → user prompt overrides; unset prompts fall back to global.
	p := BuildProvider(global, cloud, true).(*Cloud)
	if p.Prompts.Title != "MY-TITLE" {
		t.Errorf("allowed cloud title = %q, want MY-TITLE", p.Prompts.Title)
	}
	if p.Prompts.Summary != "GLOBAL-SUM" {
		t.Errorf("unset prompt should fall back to global, got %q", p.Prompts.Summary)
	}

	// Cloud + NOT allowed → global prompts only.
	p2 := BuildProvider(global, cloud, false).(*Cloud)
	if p2.Prompts.Title != "GLOBAL-TITLE" {
		t.Errorf("disallowed cloud title = %q, want GLOBAL-TITLE", p2.Prompts.Title)
	}

	// Ollama (no cloud key) ignores user prompts even when allowed.
	o := BuildProvider(global, map[string]string{KeyPromptTitle: "MY-TITLE"}, true).(*Ollama)
	if o.Prompts.Title != "GLOBAL-TITLE" {
		t.Errorf("ollama title = %q, want GLOBAL-TITLE", o.Prompts.Title)
	}
}
