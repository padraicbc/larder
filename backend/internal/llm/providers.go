package llm

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
)

// OpenAI speaks the /chat/completions format. That also covers Ollama,
// LM Studio, llama.cpp server, OpenRouter, Groq, Together, vLLM, etc.
type OpenAI struct {
	cfg  Config
	http *http.Client
}

func (o *OpenAI) Complete(ctx context.Context, system, user string) (string, error) {
	return o.send(ctx, system, user)
}

// CompleteWithImages sends images as data: URLs in the user message.
func (o *OpenAI) CompleteWithImages(ctx context.Context, system, user string, images []Image) (string, error) {
	parts := []map[string]any{{"type": "text", "text": user}}
	for _, img := range images {
		parts = append(parts, map[string]any{
			"type":      "image_url",
			"image_url": map[string]string{"url": "data:" + img.MediaType + ";base64," + base64.StdEncoding.EncodeToString(img.Data)},
		})
	}
	return o.send(ctx, system, parts)
}

func (o *OpenAI) send(ctx context.Context, system string, userContent any) (string, error) {
	body := map[string]any{
		"model":       o.cfg.Model,
		"temperature": o.cfg.Temperature,
		"messages": []map[string]any{
			{"role": "system", "content": system},
			{"role": "user", "content": userContent},
		},
	}
	if o.cfg.MaxTokens > 0 {
		body["max_completion_tokens"] = o.cfg.MaxTokens
	}
	if o.cfg.JSONMode {
		body["response_format"] = map[string]string{"type": "json_object"}
	}
	headers := map[string]string{}
	if o.cfg.APIKey != "" {
		headers["Authorization"] = "Bearer " + o.cfg.APIKey
	}
	var resp struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := postJSON(ctx, o.http, trimSlash(o.cfg.BaseURL)+"/chat/completions", headers, body, &resp); err != nil {
		return "", err
	}
	if len(resp.Choices) == 0 {
		return "", errors.New("llm: empty response")
	}
	return resp.Choices[0].Message.Content, nil
}

type Anthropic struct {
	cfg  Config
	http *http.Client
}

func (a *Anthropic) Complete(ctx context.Context, system, user string) (string, error) {
	return a.send(ctx, system, user)
}

// CompleteWithImages puts images before the text, which works best for Claude.
func (a *Anthropic) CompleteWithImages(ctx context.Context, system, user string, images []Image) (string, error) {
	var parts []map[string]any
	for _, img := range images {
		parts = append(parts, map[string]any{
			"type": "image",
			"source": map[string]string{
				"type": "base64", "media_type": img.MediaType, "data": base64.StdEncoding.EncodeToString(img.Data),
			},
		})
	}
	parts = append(parts, map[string]any{"type": "text", "text": user})
	return a.send(ctx, system, parts)
}

func (a *Anthropic) send(ctx context.Context, system string, userContent any) (string, error) {
	maxTokens := a.cfg.MaxTokens
	if maxTokens <= 0 {
		maxTokens = 8000
	}
	body := map[string]any{
		"model":                 a.cfg.Model,
		"max_completion_tokens": maxTokens,
		"temperature":           a.cfg.Temperature,
		"system":                system,
		"messages":              []map[string]any{{"role": "user", "content": userContent}},
	}
	headers := map[string]string{
		"x-api-key":         a.cfg.APIKey,
		"anthropic-version": "2023-06-01",
	}
	var resp struct {
		Content []struct {
			Type string `json:"type"`
			Text string `json:"text"`
		} `json:"content"`
	}
	if err := postJSON(ctx, a.http, trimSlash(a.cfg.BaseURL)+"/v1/messages", headers, body, &resp); err != nil {
		return "", err
	}
	var out bytes.Buffer
	for _, c := range resp.Content {
		if c.Type == "text" {
			out.WriteString(c.Text)
		}
	}
	if out.Len() == 0 {
		return "", errors.New("llm: empty response")
	}
	return out.String(), nil
}

func postJSON(ctx context.Context, hc *http.Client, url string, headers map[string]string, body, out any) error {
	b, err := json.Marshal(body)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(b))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	res, err := hc.Do(req)
	if err != nil {
		return fmt.Errorf("llm: request failed: %w", err)
	}
	defer res.Body.Close()
	data, _ := io.ReadAll(io.LimitReader(res.Body, 10<<20))
	if res.StatusCode >= 300 {
		msg := string(data)
		if len(msg) > 500 {
			msg = msg[:500]
		}
		return fmt.Errorf("llm: %s returned %d: %s", url, res.StatusCode, msg)
	}
	if err := json.Unmarshal(data, out); err != nil {
		return fmt.Errorf("llm: decode provider response: %w", err)
	}
	return nil
}
