// Package ai defines the pluggable inference Provider used by the worker and the
// "ask your notes" feature. The default implementation talks to a local Ollama
// host; a per-user OpenAI-compatible override (OpenAI, Gemini's compat endpoint,
// OpenRouter, etc.) can be configured in Settings.
//
// Models and prompts are configured globally by an admin (stored in
// app_settings) with separate models per capability — title, summary, tags, OCR,
// image analysis, embeddings, and chat.
package ai

import (
	"context"
	"strings"
	"time"
)

// Message is one turn in a chat conversation.
type Message struct {
	Role    string // system | user | assistant
	Content string
}

// Provider is the capability surface the rest of the app depends on. lang is an
// English language name (e.g. "Traditional Chinese") for the response, or "" to
// let the model match the content.
type Provider interface {
	Title(ctx context.Context, text, lang string) (string, error)
	Summarize(ctx context.Context, text, lang string) (string, error)
	Tags(ctx context.Context, text string) ([]string, error)
	// Category picks a single category for the note, preferring one of the
	// existing categories when one fits, else proposing a short new one.
	Category(ctx context.Context, text string, existing []string, lang string) (string, error)
	OCR(ctx context.Context, image []byte, mime string) (string, error)
	Describe(ctx context.Context, image []byte, mime string) (string, error)
	Embed(ctx context.Context, texts []string) ([][]float32, error)
	Chat(ctx context.Context, system string, msgs []Message) (string, error)
}

// Models holds the model name used for each capability. Empty fields fall back
// to a sensible base (Chat for text tasks, OCR for image tasks).
type Models struct {
	Title   string
	Summary string
	Tags    string
	OCR     string
	Image   string
	Embed   string
	Chat    string
}

func (m Models) title() string   { return firstNonEmpty(m.Title, m.Chat) }
func (m Models) summary() string { return firstNonEmpty(m.Summary, m.Chat) }
func (m Models) tags() string    { return firstNonEmpty(m.Tags, m.Chat) }
func (m Models) ocr() string     { return firstNonEmpty(m.OCR, m.Image) }
func (m Models) image() string   { return firstNonEmpty(m.Image, m.OCR) }
func (m Models) embed() string   { return m.Embed }
func (m Models) chat() string    { return m.Chat }

// Prompts holds the admin-editable instruction text for each capability. The
// note content is appended after the prompt.
type Prompts struct {
	Title    string
	Summary  string
	Tags     string
	Category string
	OCR      string
	Image    string
}

// DefaultPrompts returns the built-in prompts.
func DefaultPrompts() Prompts {
	return Prompts{
		Title:    "Write a concise, specific title (max 8 words) for the following note. Reply with ONLY the title, no quotes, no trailing punctuation.\n\n",
		Summary:  "Summarize the following note in 2-3 sentences. Reply with ONLY the summary.\n\n",
		Tags:     "Suggest 3 to 6 short lowercase topical tags for the following note. Reply with ONLY a comma-separated list, no other text.\n\n",
		Category: "Choose the single best category for the following note. Prefer one of the existing categories when one fits; otherwise propose a short new category (1-3 words). Reply with ONLY the category name, no quotes or punctuation.\n\n",
		// deepseek-ocr (the default vision model) is a specialist model trained on
		// specific prompts and is newline-sensitive — generic instructions like
		// "Extract all text…" produce empty/garbage output. These mirror the model's
		// documented examples (https://ollama.com/library/deepseek-ocr) and the
		// working OmniScribe presets; the leading newline is intentional.
		OCR:   "\nFree OCR.",
		Image: "\nDescribe this image in detail.",
	}
}

// GlobalConfig is the admin-controlled AI configuration plus the Ollama host.
type GlobalConfig struct {
	OllamaURL string
	Models    Models
	Prompts   Prompts
	// Timeout bounds each Ollama HTTP call (from config.yaml ai.timeout_seconds).
	// Vision OCR on CPU/remote hosts needs a generous value; 0 falls back to 180s.
	Timeout time.Duration
}

// app_settings keys for the global admin AI config.
const (
	KeyOllamaURL      = "ai_ollama_url"
	KeyModelTitle     = "model_title"
	KeyModelSummary   = "model_summary"
	KeyModelTags      = "model_tags"
	KeyModelOCR       = "model_ocr"
	KeyModelImage     = "model_image"
	KeyModelEmbed     = "model_embed"
	KeyModelChat      = "model_chat"
	KeyPromptTitle    = "prompt_title"
	KeyPromptSummary  = "prompt_summary"
	KeyPromptTags     = "prompt_tags"
	KeyPromptCategory = "prompt_category"
	KeyPromptOCR      = "prompt_ocr"
	KeyPromptImage    = "prompt_image"

	// Source-specific prompts. When set, they override the generic title/summary/
	// tags prompts for web-link ("url") and YouTube notes respectively. Blank
	// means "use the generic prompt".
	KeyPromptWebTitle   = "prompt_web_title"
	KeyPromptWebSummary = "prompt_web_summary"
	KeyPromptWebTags    = "prompt_web_tags"
	KeyPromptYTTitle    = "prompt_yt_title"
	KeyPromptYTSummary  = "prompt_yt_summary"
	KeyPromptYTTags     = "prompt_yt_tags"
)

// ApplySourcePrompts overlays the source-specific admin prompts (web/youtube)
// onto a GlobalConfig's generic title/summary/tags prompts, based on the note's
// source_type. Empty overrides are ignored so the generic prompt remains. Call
// this after LoadGlobal and before BuildProvider.
func ApplySourcePrompts(g GlobalConfig, app map[string]string, sourceType string) GlobalConfig {
	var titleKey, summaryKey, tagsKey string
	switch sourceType {
	case "url":
		titleKey, summaryKey, tagsKey = KeyPromptWebTitle, KeyPromptWebSummary, KeyPromptWebTags
	case "youtube":
		titleKey, summaryKey, tagsKey = KeyPromptYTTitle, KeyPromptYTSummary, KeyPromptYTTags
	default:
		return g
	}
	g.Prompts.Title = firstNonEmpty(app[titleKey], g.Prompts.Title)
	g.Prompts.Summary = firstNonEmpty(app[summaryKey], g.Prompts.Summary)
	g.Prompts.Tags = firstNonEmpty(app[tagsKey], g.Prompts.Tags)
	return g
}

// LoadGlobal overlays admin app_settings onto a fallback (from config.yaml),
// producing the effective global config.
func LoadGlobal(app map[string]string, fallback GlobalConfig) GlobalConfig {
	g := fallback
	g.OllamaURL = firstNonEmpty(app[KeyOllamaURL], g.OllamaURL)
	g.Models.Title = firstNonEmpty(app[KeyModelTitle], g.Models.Title)
	g.Models.Summary = firstNonEmpty(app[KeyModelSummary], g.Models.Summary)
	g.Models.Tags = firstNonEmpty(app[KeyModelTags], g.Models.Tags)
	g.Models.OCR = firstNonEmpty(app[KeyModelOCR], g.Models.OCR)
	g.Models.Image = firstNonEmpty(app[KeyModelImage], g.Models.Image)
	g.Models.Embed = firstNonEmpty(app[KeyModelEmbed], g.Models.Embed)
	g.Models.Chat = firstNonEmpty(app[KeyModelChat], g.Models.Chat)
	g.Prompts.Title = firstNonEmpty(app[KeyPromptTitle], g.Prompts.Title)
	g.Prompts.Summary = firstNonEmpty(app[KeyPromptSummary], g.Prompts.Summary)
	g.Prompts.Tags = firstNonEmpty(app[KeyPromptTags], g.Prompts.Tags)
	g.Prompts.Category = firstNonEmpty(app[KeyPromptCategory], g.Prompts.Category)
	g.Prompts.OCR = firstNonEmpty(app[KeyPromptOCR], g.Prompts.OCR)
	g.Prompts.Image = firstNonEmpty(app[KeyPromptImage], g.Prompts.Image)
	return g
}

// Per-user override settings keys (cloud backend).
const (
	KeyBaseURL     = "ai_base_url"
	KeyAPIKey      = "ai_api_key"
	KeyChatModel   = "ai_chat_model"
	KeyEmbedModel  = "ai_embed_model"
	KeyVisionModel = "ai_vision_model"
)

// BuildProvider returns the provider for a user: the cloud override when both a
// base URL and API key are present in their settings, otherwise the default
// Ollama provider. Cloud model names come from the user's settings. Prompts come
// from the global (admin) config, but a cloud user whose role permits it
// (allowUserPrompts) may override individual prompts in their settings. Ollama
// users always use the admin prompts.
func BuildProvider(global GlobalConfig, userSettings map[string]string, allowUserPrompts bool) Provider {
	base := strings.TrimSpace(userSettings[KeyBaseURL])
	key := strings.TrimSpace(userSettings[KeyAPIKey])
	if base != "" && key != "" {
		chat := firstNonEmpty(userSettings[KeyChatModel], "gpt-4o-mini")
		vision := firstNonEmpty(userSettings[KeyVisionModel], chat)
		models := Models{
			Title: chat, Summary: chat, Tags: chat, Chat: chat,
			OCR: vision, Image: vision,
			Embed: firstNonEmpty(userSettings[KeyEmbedModel], "text-embedding-3-small"),
		}
		prompts := global.Prompts
		if allowUserPrompts {
			prompts = overlayPrompts(prompts, userSettings)
		}
		return NewCloud(base, key, models, prompts, global.Timeout)
	}
	return NewOllama(global.OllamaURL, global.Models, global.Prompts, global.Timeout)
}

// overlayPrompts applies a user's non-empty prompt overrides onto the base
// prompts.
func overlayPrompts(p Prompts, s map[string]string) Prompts {
	if v := strings.TrimSpace(s[KeyPromptTitle]); v != "" {
		p.Title = v
	}
	if v := strings.TrimSpace(s[KeyPromptSummary]); v != "" {
		p.Summary = v
	}
	if v := strings.TrimSpace(s[KeyPromptTags]); v != "" {
		p.Tags = v
	}
	if v := strings.TrimSpace(s[KeyPromptCategory]); v != "" {
		p.Category = v
	}
	if v := strings.TrimSpace(s[KeyPromptOCR]); v != "" {
		p.OCR = v
	}
	if v := strings.TrimSpace(s[KeyPromptImage]); v != "" {
		p.Image = v
	}
	return p
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}

// ----- shared response cleaners -----

func cleanTitle(s string) string {
	s = strings.TrimSpace(s)
	s = strings.Trim(s, "\"'`")
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		s = s[:i]
	}
	return strings.TrimSpace(s)
}

// existingClause lists the user's current categories for the model to prefer.
func existingClause(existing []string) string {
	if len(existing) == 0 {
		return ""
	}
	return "Existing categories: " + strings.Join(existing, ", ") + ".\n"
}

// cleanCategory normalizes a model's category reply to a single short label.
func cleanCategory(s string) string {
	s = cleanTitle(s) // first line only
	s = strings.Trim(s, " \t\"'`.,:;-#")
	if len([]rune(s)) > 40 {
		s = string([]rune(s)[:40])
	}
	return s
}

func parseTags(s string) []string {
	s = strings.ReplaceAll(s, "\n", ",")
	var out []string
	for _, part := range strings.Split(s, ",") {
		t := strings.ToLower(strings.TrimSpace(part))
		t = strings.Trim(t, "#.\"'`")
		t = strings.TrimSpace(t)
		if t != "" && len(t) <= 40 {
			out = append(out, t)
		}
	}
	if len(out) > 6 {
		out = out[:6]
	}
	return out
}

func clip(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max]
}

// langClause returns an instruction to respond in a given language, prepended
// before the content. Empty lang lets the model match the content.
func langClause(lang string) string {
	if strings.TrimSpace(lang) == "" {
		return ""
	}
	return "Write your answer in " + lang + ".\n\n"
}
