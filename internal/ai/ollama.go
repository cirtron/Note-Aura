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

// Ollama is the default local Provider. It uses /api/generate for text tasks and
// OCR/image analysis (vision models accept an images field), /api/embeddings for
// vectors, and /api/chat for conversational answers. Models and prompts are
// supplied per capability.
type Ollama struct {
	BaseURL string
	Models  Models
	Prompts Prompts
	HTTP    *http.Client
}

// NewOllama builds a local provider. timeout bounds each HTTP call; because
// generation runs with stream:false, this single timeout must cover the WHOLE
// response — including a cold vision-model load + inference, which can be slow on
// CPU or a remote host. A non-positive timeout falls back to 180s.
func NewOllama(baseURL string, models Models, prompts Prompts, timeout time.Duration) *Ollama {
	if timeout <= 0 {
		timeout = 180 * time.Second
	}
	return &Ollama{
		BaseURL: strings.TrimRight(baseURL, "/"),
		Models:  models,
		Prompts: prompts,
		HTTP:    &http.Client{Timeout: timeout},
	}
}

type ollamaGenReq struct {
	Model  string   `json:"model"`
	Prompt string   `json:"prompt"`
	Images []string `json:"images,omitempty"`
	Stream bool     `json:"stream"`
}

type ollamaGenResp struct {
	Response string `json:"response"`
	Error    string `json:"error,omitempty"`
}

func (o *Ollama) generate(ctx context.Context, model, prompt string, images []string) (string, error) {
	if model == "" {
		return "", fmt.Errorf("no model configured for this capability")
	}
	body, _ := json.Marshal(ollamaGenReq{Model: model, Prompt: prompt, Images: images, Stream: false})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, o.BaseURL+"/api/generate", bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := o.HTTP.Do(req)
	if err != nil {
		return "", fmt.Errorf("ollama request: %w", err)
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("ollama %d: %s", resp.StatusCode, truncate(string(raw), 300))
	}
	var gr ollamaGenResp
	if err := json.Unmarshal(raw, &gr); err != nil {
		return "", fmt.Errorf("decode ollama response: %w", err)
	}
	if gr.Error != "" {
		return "", fmt.Errorf("ollama error: %s", gr.Error)
	}
	return strings.TrimSpace(gr.Response), nil
}

func (o *Ollama) Title(ctx context.Context, text, lang string) (string, error) {
	out, err := o.generate(ctx, o.Models.title(), o.Prompts.Title+langClause(lang)+clip(text, 8000), nil)
	if err != nil {
		return "", err
	}
	return cleanTitle(out), nil
}

func (o *Ollama) Summarize(ctx context.Context, text, lang string) (string, error) {
	return o.generate(ctx, o.Models.summary(), o.Prompts.Summary+langClause(lang)+clip(text, 12000), nil)
}

func (o *Ollama) Tags(ctx context.Context, text string) ([]string, error) {
	out, err := o.generate(ctx, o.Models.tags(), o.Prompts.Tags+clip(text, 8000), nil)
	if err != nil {
		return nil, err
	}
	return parseTags(out), nil
}

func (o *Ollama) Category(ctx context.Context, text string, existing []string, lang string) (string, error) {
	out, err := o.generate(ctx, o.Models.tags(), o.Prompts.Category+existingClause(existing)+langClause(lang)+clip(text, 6000), nil)
	if err != nil {
		return "", err
	}
	return cleanCategory(out), nil
}

func (o *Ollama) OCR(ctx context.Context, image []byte, mime string) (string, error) {
	b64 := base64.StdEncoding.EncodeToString(image)
	return o.generate(ctx, o.Models.ocr(), o.Prompts.OCR, []string{b64})
}

func (o *Ollama) Describe(ctx context.Context, image []byte, mime string) (string, error) {
	b64 := base64.StdEncoding.EncodeToString(image)
	return o.generate(ctx, o.Models.image(), o.Prompts.Image, []string{b64})
}

type ollamaEmbedReq struct {
	Model  string `json:"model"`
	Prompt string `json:"prompt"`
}
type ollamaEmbedResp struct {
	Embedding []float32 `json:"embedding"`
	Error     string    `json:"error,omitempty"`
}

func (o *Ollama) Embed(ctx context.Context, texts []string) ([][]float32, error) {
	model := o.Models.embed()
	if model == "" {
		return nil, fmt.Errorf("no embedding model configured")
	}
	out := make([][]float32, 0, len(texts))
	for _, t := range texts {
		body, _ := json.Marshal(ollamaEmbedReq{Model: model, Prompt: clip(t, 8000)})
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, o.BaseURL+"/api/embeddings", bytes.NewReader(body))
		if err != nil {
			return nil, err
		}
		req.Header.Set("Content-Type", "application/json")
		resp, err := o.HTTP.Do(req)
		if err != nil {
			return nil, fmt.Errorf("ollama embed request: %w", err)
		}
		raw, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			return nil, fmt.Errorf("ollama embed %d: %s", resp.StatusCode, truncate(string(raw), 300))
		}
		var er ollamaEmbedResp
		if err := json.Unmarshal(raw, &er); err != nil {
			return nil, fmt.Errorf("decode embed response: %w", err)
		}
		if er.Error != "" {
			return nil, fmt.Errorf("ollama embed error: %s", er.Error)
		}
		out = append(out, er.Embedding)
	}
	return out, nil
}

type ollamaChatReq struct {
	Model    string          `json:"model"`
	Messages []ollamaChatMsg `json:"messages"`
	Stream   bool            `json:"stream"`
}
type ollamaChatMsg struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}
type ollamaChatResp struct {
	Message ollamaChatMsg `json:"message"`
	Error   string        `json:"error,omitempty"`
}

func (o *Ollama) Chat(ctx context.Context, system string, msgs []Message) (string, error) {
	model := o.Models.chat()
	if model == "" {
		return "", fmt.Errorf("no chat model configured")
	}
	all := make([]ollamaChatMsg, 0, len(msgs)+1)
	if system != "" {
		all = append(all, ollamaChatMsg{Role: "system", Content: system})
	}
	for _, m := range msgs {
		all = append(all, ollamaChatMsg{Role: m.Role, Content: m.Content})
	}
	body, _ := json.Marshal(ollamaChatReq{Model: model, Messages: all, Stream: false})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, o.BaseURL+"/api/chat", bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := o.HTTP.Do(req)
	if err != nil {
		return "", fmt.Errorf("ollama chat request: %w", err)
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("ollama chat %d: %s", resp.StatusCode, truncate(string(raw), 300))
	}
	var cr ollamaChatResp
	if err := json.Unmarshal(raw, &cr); err != nil {
		return "", fmt.Errorf("decode chat response: %w", err)
	}
	if cr.Error != "" {
		return "", fmt.Errorf("ollama chat error: %s", cr.Error)
	}
	return strings.TrimSpace(cr.Message.Content), nil
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "...(truncated)"
}
