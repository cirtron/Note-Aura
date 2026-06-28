package ai

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// Cloud is an OpenAI-compatible Provider. It works with the OpenAI API, Google
// Gemini's OpenAI-compatible endpoint, OpenRouter, Groq, and similar services.
// Models are supplied per capability; prompts come from the global admin config.
type Cloud struct {
	BaseURL string // e.g. https://api.openai.com/v1
	APIKey  string
	Models  Models
	Prompts Prompts
	HTTP    *http.Client
}

// NewCloud builds an OpenAI-compatible provider. timeout bounds each HTTP call
// (from config.yaml ai.timeout_seconds); a non-positive value falls back to 180s.
func NewCloud(baseURL, apiKey string, models Models, prompts Prompts, timeout time.Duration) *Cloud {
	if timeout <= 0 {
		timeout = 180 * time.Second
	}
	return &Cloud{
		BaseURL: strings.TrimRight(baseURL, "/"),
		APIKey:  apiKey,
		Models:  models,
		Prompts: prompts,
		HTTP:    &http.Client{Timeout: timeout},
	}
}

func (c *Cloud) post(ctx context.Context, path string, payload any, out any) error {
	body, _ := json.Marshal(payload)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.BaseURL+path, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.APIKey)
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return fmt.Errorf("cloud request: %w", err)
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("cloud %d: %s", resp.StatusCode, truncate(string(raw), 300))
	}
	return json.Unmarshal(raw, out)
}

type chatCompletionResp struct {
	Choices []struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
	} `json:"choices"`
}

func (c *Cloud) chat(ctx context.Context, model string, messages []map[string]any) (string, error) {
	if model == "" {
		return "", fmt.Errorf("no model configured for this capability")
	}
	var out chatCompletionResp
	err := c.post(ctx, "/chat/completions", map[string]any{
		"model":    model,
		"messages": messages,
	}, &out)
	if err != nil {
		return "", err
	}
	if len(out.Choices) == 0 {
		return "", fmt.Errorf("cloud: empty completion")
	}
	return strings.TrimSpace(out.Choices[0].Message.Content), nil
}

func textMsg(role, content string) map[string]any {
	return map[string]any{"role": role, "content": content}
}

func (c *Cloud) visionMsg(prompt, mime string, image []byte) map[string]any {
	if mime == "" {
		mime = "image/png"
	}
	dataURI := "data:" + mime + ";base64," + base64.StdEncoding.EncodeToString(image)
	return map[string]any{
		"role": "user",
		"content": []any{
			map[string]any{"type": "text", "text": prompt},
			map[string]any{"type": "image_url", "image_url": map[string]any{"url": dataURI}},
		},
	}
}

func (c *Cloud) Title(ctx context.Context, text, lang string) (string, error) {
	out, err := c.chat(ctx, c.Models.title(), []map[string]any{textMsg("user", c.Prompts.Title+langClause(lang)+clip(text, 12000))})
	if err != nil {
		return "", err
	}
	return cleanTitle(out), nil
}

func (c *Cloud) Summarize(ctx context.Context, text, lang string) (string, error) {
	return c.chat(ctx, c.Models.summary(), []map[string]any{textMsg("user", c.Prompts.Summary+langClause(lang)+clip(text, 16000))})
}

func (c *Cloud) Tags(ctx context.Context, text string) ([]string, error) {
	out, err := c.chat(ctx, c.Models.tags(), []map[string]any{textMsg("user", c.Prompts.Tags+clip(text, 12000))})
	if err != nil {
		return nil, err
	}
	return parseTags(out), nil
}

func (c *Cloud) Category(ctx context.Context, text string, existing []string, lang string) (string, error) {
	out, err := c.chat(ctx, c.Models.tags(), []map[string]any{textMsg("user", c.Prompts.Category+existingClause(existing)+langClause(lang)+clip(text, 12000))})
	if err != nil {
		return "", err
	}
	return cleanCategory(out), nil
}

func (c *Cloud) OCR(ctx context.Context, image []byte, mime string, lang string) (string, error) {
	return c.chat(ctx, c.Models.ocr(), []map[string]any{c.visionMsg(c.Prompts.OCR+langClause(lang), mime, image)})
}

func (c *Cloud) Describe(ctx context.Context, image []byte, mime string, lang string) (string, error) {
	return c.chat(ctx, c.Models.image(), []map[string]any{c.visionMsg(c.Prompts.Image+langClause(lang), mime, image)})
}

type embeddingsResp struct {
	Data []struct {
		Embedding []float32 `json:"embedding"`
	} `json:"data"`
}

func (c *Cloud) Embed(ctx context.Context, texts []string) ([][]float32, error) {
	if c.Models.embed() == "" {
		return nil, fmt.Errorf("no embedding model configured")
	}
	clipped := make([]string, len(texts))
	for i, t := range texts {
		clipped[i] = clip(t, 8000)
	}
	var out embeddingsResp
	err := c.post(ctx, "/embeddings", map[string]any{
		"model": c.Models.embed(),
		"input": clipped,
	}, &out)
	if err != nil {
		return nil, err
	}
	res := make([][]float32, len(out.Data))
	for i, d := range out.Data {
		res[i] = d.Embedding
	}
	return res, nil
}

func (c *Cloud) Chat(ctx context.Context, system string, msgs []Message) (string, error) {
	messages := make([]map[string]any, 0, len(msgs)+1)
	if system != "" {
		messages = append(messages, textMsg("system", system))
	}
	for _, m := range msgs {
		messages = append(messages, textMsg(m.Role, m.Content))
	}
	return c.chat(ctx, c.Models.chat(), messages)
}
