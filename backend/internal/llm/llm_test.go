package llm

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestExtractJSON(t *testing.T) {
	tests := map[string]struct {
		in, want string
		err      bool
	}{
		"plain":        {`{"a":1}`, `{"a":1}`, false},
		"fenced":       {"```json\n{\"a\":1}\n```", `{"a":1}`, false},
		"chatty":       {"Sure! Here you go: {\"a\":{\"b\":2}} Enjoy.", `{"a":{"b":2}}`, false},
		"array":        {"[1,2]", "[1,2]", false},
		"none":         {"no json here", "", true},
		"unterminated": {"{\"a\":", "", true},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			got, err := ExtractJSON(tt.in)
			if (err != nil) != tt.err || got != tt.want {
				t.Errorf("ExtractJSON(%q) = %q, %v", tt.in, got, err)
			}
		})
	}
}

func TestMasking(t *testing.T) {
	c := Config{APIKey: "sk-secret-1234"}
	m := c.Masked()
	if m.APIKey != "••••1234" || !IsMaskedKey(m.APIKey) {
		t.Errorf("masked = %q", m.APIKey)
	}
	if c.APIKey != "sk-secret-1234" {
		t.Error("Masked must not modify the original")
	}
	if (Config{}).Masked().APIKey != "" {
		t.Error("empty key should stay empty")
	}
}

func TestNewValidates(t *testing.T) {
	if _, err := New(Config{Provider: "openai"}); err == nil {
		t.Error("openai without model should error")
	}
	if _, err := New(Config{Provider: "anthropic", Model: "m"}); err == nil {
		t.Error("anthropic without key should error")
	}
	if _, err := New(Config{Provider: "nope"}); err == nil {
		t.Error("unknown provider should error")
	}
	if c, err := New(Config{}); err != nil || c == nil {
		t.Error("empty provider should default to mock")
	}
}

// capture spins up a fake provider and records the request it receives.
func capture(t *testing.T, reply string, status int) (*httptest.Server, *http.Request, *map[string]any) {
	t.Helper()
	var req http.Request
	body := map[string]any{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		req = *r
		_ = json.NewDecoder(r.Body).Decode(&body)
		w.WriteHeader(status)
		_, _ = w.Write([]byte(reply))
	}))
	t.Cleanup(srv.Close)
	return srv, &req, &body
}

func TestOpenAIProvider(t *testing.T) {
	srv, req, body := capture(t, `{"choices":[{"message":{"content":"hello"}}]}`, 200)
	c, _ := New(Config{Provider: "openai", BaseURL: srv.URL + "/v1/", Model: "llama3.1", APIKey: "k", JSONMode: true, MaxTokens: 100})
	out, err := c.Complete(context.Background(), "sys", "usr")
	if err != nil || out != "hello" {
		t.Fatalf("got %q, %v", out, err)
	}
	if req.URL.Path != "/v1/chat/completions" {
		t.Errorf("path = %s", req.URL.Path)
	}
	if req.Header.Get("Authorization") != "Bearer k" {
		t.Errorf("auth header = %q", req.Header.Get("Authorization"))
	}
	if (*body)["model"] != "llama3.1" || (*body)["response_format"] == nil {
		t.Errorf("body = %v", *body)
	}
}

func TestOpenAIProviderNoKeyForLocal(t *testing.T) {
	srv, req, _ := capture(t, `{"choices":[{"message":{"content":"x"}}]}`, 200)
	c, _ := New(Config{Provider: "openai", BaseURL: srv.URL, Model: "m"})
	if _, err := c.Complete(context.Background(), "s", "u"); err != nil {
		t.Fatal(err)
	}
	if req.Header.Get("Authorization") != "" {
		t.Error("should not send auth header without a key (Ollama etc.)")
	}
}

func TestAnthropicProvider(t *testing.T) {
	srv, req, body := capture(t, `{"content":[{"type":"text","text":"a"},{"type":"text","text":"b"}]}`, 200)
	c, _ := New(Config{Provider: "anthropic", BaseURL: srv.URL, Model: "m", APIKey: "k"})
	out, err := c.Complete(context.Background(), "sys", "usr")
	if err != nil || out != "ab" {
		t.Fatalf("got %q, %v", out, err)
	}
	if req.URL.Path != "/v1/messages" || req.Header.Get("x-api-key") != "k" || req.Header.Get("anthropic-version") == "" {
		t.Errorf("bad request: %s %v", req.URL.Path, req.Header)
	}
	if (*body)["system"] != "sys" {
		t.Errorf("system prompt not sent: %v", *body)
	}
}

func TestProviderErrorIncludesBody(t *testing.T) {
	srv, _, _ := capture(t, `{"error":"model not found"}`, 404)
	c, _ := New(Config{Provider: "openai", BaseURL: srv.URL, Model: "m"})
	_, err := c.Complete(context.Background(), "s", "u")
	if err == nil || !strings.Contains(err.Error(), "model not found") || !strings.Contains(err.Error(), "404") {
		t.Errorf("err = %v", err)
	}
}

func TestMockEstimate(t *testing.T) {
	var out map[string]any
	if err := CompleteJSON(context.Background(), Mock{}, "", "TASK: estimate_macros\nMeal: toast", &out); err != nil {
		t.Fatal(err)
	}
	if out["macros"] == nil {
		t.Errorf("mock estimate missing macros: %v", out)
	}
}

func TestVisionRequests(t *testing.T) {
	img := []Image{{MediaType: "image/png", Data: []byte("PNG")}} // base64 "UE5H"

	t.Run("openai", func(t *testing.T) {
		srv, _, body := capture(t, `{"choices":[{"message":{"content":"ok"}}]}`, 200)
		c, _ := New(Config{Provider: "openai", BaseURL: srv.URL, Model: "m"})
		if _, err := c.(VisionClient).CompleteWithImages(context.Background(), "s", "what is this", img); err != nil {
			t.Fatal(err)
		}
		b, _ := json.Marshal(*body)
		for _, want := range []string{`"type":"image_url"`, `data:image/png;base64,UE5H`, `"text":"what is this"`} {
			if !strings.Contains(string(b), want) {
				t.Errorf("openai body missing %s: %s", want, b)
			}
		}
	})

	t.Run("anthropic", func(t *testing.T) {
		srv, _, body := capture(t, `{"content":[{"type":"text","text":"ok"}]}`, 200)
		c, _ := New(Config{Provider: "anthropic", BaseURL: srv.URL, Model: "m", APIKey: "k"})
		if _, err := c.(VisionClient).CompleteWithImages(context.Background(), "s", "what is this", img); err != nil {
			t.Fatal(err)
		}
		b, _ := json.Marshal(*body)
		for _, want := range []string{`"type":"image"`, `"media_type":"image/png"`, `"data":"UE5H"`, `"type":"base64"`} {
			if !strings.Contains(string(b), want) {
				t.Errorf("anthropic body missing %s: %s", want, b)
			}
		}
	})
}

func TestForVision(t *testing.T) {
	if (Config{Model: "text"}).ForVision().Model != "text" {
		t.Error("no vision model: keep the main model")
	}
	if (Config{Model: "text", VisionModel: "eyes"}).ForVision().Model != "eyes" {
		t.Error("vision model should be used for photos")
	}
}
