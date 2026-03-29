// Package ai provides pluggable LLM providers for knet analysis.
package ai

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// Provider is the common interface all AI backends implement.
type Provider interface {
	Name() string
	Complete(ctx context.Context, prompt string) (string, error)
}

// Config holds the settings needed to build a Provider.
type Config struct {
	Provider string
	APIKey   string
	Model    string
	BaseURL  string // optional override (used by OpenRouter / custom endpoints)
}

// New returns the correct Provider implementation for the given Config.
func New(cfg Config) (Provider, error) {
	switch cfg.Provider {
	case "openai":
		m := cfg.Model
		if m == "" {
			m = "gpt-4o"
		}
		base := cfg.BaseURL
		if base == "" {
			base = "https://api.openai.com/v1"
		}
		return &openAIProvider{key: cfg.APIKey, model: m, baseURL: base}, nil
	case "anthropic":
		m := cfg.Model
		if m == "" {
			m = "claude-3-5-sonnet-20241022"
		}
		return &anthropicProvider{key: cfg.APIKey, model: m}, nil
	case "openrouter":
		m := cfg.Model
		if m == "" {
			m = "openai/gpt-4o"
		}
		return &openAIProvider{key: cfg.APIKey, model: m, baseURL: "https://openrouter.ai/api/v1"}, nil
	default:
		return nil, fmt.Errorf("unknown AI provider %q — supported: openai, anthropic, openrouter", cfg.Provider)
	}
}

// ─── OpenAI (and OpenRouter) ─────────────────────────────────────────────────

type openAIProvider struct {
	key     string
	model   string
	baseURL string
}

func (p *openAIProvider) Name() string { return "openai (" + p.model + ")" }

func (p *openAIProvider) Complete(ctx context.Context, prompt string) (string, error) {
	type message struct {
		Role    string `json:"role"`
		Content string `json:"content"`
	}
	body := map[string]interface{}{
		"model": p.model,
		"messages": []message{
			{Role: "system", Content: "You are an expert Kubernetes network security analyst."},
			{Role: "user", Content: prompt},
		},
		"max_tokens": 2048,
	}
	b, _ := json.Marshal(body)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, p.baseURL+"/chat/completions", bytes.NewReader(b))
	if err != nil {
		return "", err
	}
	req.Header.Set("Authorization", "Bearer "+p.key)
	req.Header.Set("Content-Type", "application/json")

	resp, err := httpClient().Do(req)
	if err != nil {
		return "", fmt.Errorf("AI request failed: %w", err)
	}
	defer resp.Body.Close()

	raw, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("AI API returned %d: %s", resp.StatusCode, string(raw))
	}

	var out struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
		Error *struct {
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.Unmarshal(raw, &out); err != nil {
		return "", fmt.Errorf("parsing AI response: %w", err)
	}
	if out.Error != nil {
		return "", fmt.Errorf("AI error: %s", out.Error.Message)
	}
	if len(out.Choices) == 0 {
		return "", fmt.Errorf("AI returned empty choices")
	}
	return out.Choices[0].Message.Content, nil
}

// ─── Anthropic ───────────────────────────────────────────────────────────────

type anthropicProvider struct {
	key   string
	model string
}

func (p *anthropicProvider) Name() string { return "anthropic (" + p.model + ")" }

func (p *anthropicProvider) Complete(ctx context.Context, prompt string) (string, error) {
	body := map[string]interface{}{
		"model": p.model,
		"messages": []map[string]string{
			{"role": "user", "content": prompt},
		},
		"system":     "You are an expert Kubernetes network security analyst.",
		"max_tokens": 2048,
	}
	b, _ := json.Marshal(body)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, "https://api.anthropic.com/v1/messages", bytes.NewReader(b))
	if err != nil {
		return "", err
	}
	req.Header.Set("x-api-key", p.key)
	req.Header.Set("anthropic-version", "2023-06-01")
	req.Header.Set("Content-Type", "application/json")

	resp, err := httpClient().Do(req)
	if err != nil {
		return "", fmt.Errorf("AI request failed: %w", err)
	}
	defer resp.Body.Close()

	raw, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("Anthropic API returned %d: %s", resp.StatusCode, string(raw))
	}

	var out struct {
		Content []struct {
			Type string `json:"type"`
			Text string `json:"text"`
		} `json:"content"`
		Error *struct {
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.Unmarshal(raw, &out); err != nil {
		return "", fmt.Errorf("parsing Anthropic response: %w", err)
	}
	if out.Error != nil {
		return "", fmt.Errorf("Anthropic error: %s", out.Error.Message)
	}
	for _, c := range out.Content {
		if c.Type == "text" {
			return c.Text, nil
		}
	}
	return "", fmt.Errorf("Anthropic returned no text content")
}

// ─── shared HTTP client ───────────────────────────────────────────────────────

func httpClient() *http.Client {
	return &http.Client{Timeout: 90 * time.Second}
}
