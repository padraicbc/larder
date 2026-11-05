// Package llm is a tiny provider-agnostic wrapper around chat-completion APIs.
// Add a provider by implementing Client and adding a case to New.
package llm

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"
)

type Client interface {
	Complete(ctx context.Context, system, user string) (string, error)
}

// Image is a photo sent to a vision-capable model.
type Image struct {
	MediaType string // image/jpeg, image/png, image/webp, image/gif
	Data      []byte
}

// VisionClient is implemented by providers that can read images. Whether the
// configured *model* actually supports images is up to the provider; the error
// it returns is passed back to the user.
type VisionClient interface {
	CompleteWithImages(ctx context.Context, system, user string, images []Image) (string, error)
}

// Config is what the settings UI edits.
type Config struct {
	Provider       string  `json:"provider"` // mock | openai | anthropic
	BaseURL        string  `json:"baseUrl"`
	Model          string  `json:"model"`
	APIKey         string  `json:"apiKey"`
	Temperature    float64 `json:"temperature"`
	MaxTokens      int     `json:"maxTokens"`
	TimeoutSeconds int     `json:"timeoutSeconds"`
	JSONMode       bool    `json:"jsonMode"`    // OpenAI-style response_format; turn off if your server rejects it
	VisionModel    string  `json:"visionModel"` // optional model for photos; empty = use Model
}

// ForVision returns the config to use for image requests.
func (c Config) ForVision() Config {
	if c.VisionModel != "" {
		c.Model = c.VisionModel
	}
	return c
}

func DefaultConfig() Config {
	return Config{Provider: "mock", Temperature: 0.7, MaxTokens: 8000, TimeoutSeconds: 120}
}

// Masked returns a copy safe to send to the browser.
func (c Config) Masked() Config {
	if c.APIKey != "" {
		k := c.APIKey
		if len(k) > 4 {
			k = k[len(k)-4:]
		}
		c.APIKey = "••••" + k
	}
	return c
}

// IsMaskedKey reports whether s is a value produced by Masked (so we keep the stored key).
func IsMaskedKey(s string) bool { return strings.HasPrefix(s, "••••") }

func New(c Config) (Client, error) {
	timeout := time.Duration(c.TimeoutSeconds) * time.Second
	if timeout <= 0 {
		timeout = 120 * time.Second
	}
	hc := &http.Client{Timeout: timeout}
	switch c.Provider {
	case "", "mock":
		return Mock{}, nil
	case "openai":
		if c.BaseURL == "" {
			c.BaseURL = "https://api.openai.com/v1"
		}
		if c.Model == "" {
			return nil, errors.New("llm: model is required")
		}
		return &OpenAI{cfg: c, http: hc}, nil
	case "anthropic":
		if c.BaseURL == "" {
			c.BaseURL = "https://api.anthropic.com"
		}
		if c.Model == "" {
			return nil, errors.New("llm: model is required")
		}
		if c.APIKey == "" {
			return nil, errors.New("llm: anthropic needs an API key")
		}
		return &Anthropic{cfg: c, http: hc}, nil
	default:
		return nil, fmt.Errorf("llm: unknown provider %q", c.Provider)
	}
}

// ExtractJSON pulls the first JSON object out of a model reply, tolerating
// code fences and chatter before/after.
func ExtractJSON(s string) (string, error) {
	start := strings.IndexAny(s, "{[")
	if start < 0 {
		return "", errors.New("llm: no JSON in response")
	}
	open := s[start]
	closeCh := byte('}')
	if open == '[' {
		closeCh = ']'
	}
	end := strings.LastIndexByte(s, closeCh)
	if end <= start {
		return "", errors.New("llm: unterminated JSON in response")
	}
	return s[start : end+1], nil
}

// CompleteJSON asks the model and decodes its JSON reply into out.
func CompleteJSON(ctx context.Context, c Client, system, user string, out any) error {
	raw, err := c.Complete(ctx, system, user)
	if err != nil {
		return err
	}
	return decodeReply(raw, out)
}

// CompleteJSONWithImages is CompleteJSON for vision requests.
func CompleteJSONWithImages(ctx context.Context, c VisionClient, system, user string, images []Image, out any) error {
	raw, err := c.CompleteWithImages(ctx, system, user, images)
	if err != nil {
		return err
	}
	return decodeReply(raw, out)
}

func decodeReply(raw string, out any) error {
	js, err := ExtractJSON(raw)
	if err != nil {
		return err
	}
	if err := json.Unmarshal([]byte(js), out); err != nil {
		return fmt.Errorf("llm: decode JSON: %w", err)
	}
	return nil
}

func trimSlash(s string) string { return strings.TrimRight(s, "/") }
