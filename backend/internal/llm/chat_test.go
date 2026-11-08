package llm

import (
	"context"
	"strings"
	"testing"
)

func TestOpenAIWebChatProtocol(t *testing.T) {
	srv, req, body := capture(t, `{"status":"completed","output":[{"type":"web_search_call","status":"completed"},{"type":"message","content":[{"type":"output_text","text":"Cheese source","annotations":[{"type":"url_citation","url":"https://maker.example/cheese","title":"Manufacturer","start_index":7,"end_index":13},{"type":"url_citation","url":"javascript:bad","start_index":0,"end_index":6}]}]}]}`, 200)
	c, _ := New(Config{Provider: "openai", BaseURL: srv.URL + "/v1", Model: "configured-model", APIKey: "test-key", JSONMode: true, MaxTokens: 2000})
	messages := []ChatMessage{{Role: "user", Content: "Check cheese"}, {Role: "assistant", Content: "Which variety?"}, {Role: "user", Content: "Vintage, Ireland"}}
	reply, err := Converse(context.Background(), c, "Server instructions", messages, true)
	if err != nil || !reply.Searched || len(reply.Citations) != 1 || reply.Citations[0].Start != 7 {
		t.Fatalf("reply: %+v %v", reply, err)
	}
	if req.URL.Path != "/v1/responses" || req.Header.Get("Authorization") != "Bearer test-key" || (*body)["model"] != "configured-model" || (*body)["store"] != false {
		t.Fatalf("request: %s %+v", req.URL.Path, body)
	}
	if _, ok := (*body)["response_format"]; ok {
		t.Fatal("chat must retain native text and citations")
	}
	tools := (*body)["tools"].([]any)
	if tools[0].(map[string]any)["type"] != "web_search" {
		t.Fatal("missing actual search tool")
	}
	if len((*body)["input"].([]any)) != 3 {
		t.Fatal("follow-up history lost")
	}
}
func TestChatDoesNotFakeSearchOrDropErrors(t *testing.T) {
	if _, err := Converse(context.Background(), Mock{}, "", []ChatMessage{{Role: "user", Content: "Find food"}}, true); err == nil {
		t.Fatal("mock must not pretend to search")
	}
	for _, tc := range []struct {
		status int
		body   string
	}{{400, `{"error":{"message":"model does not support web search"}}`}, {200, `{"status":"incomplete","output":[]}`}, {200, `{"status":"completed","output":[]}`}} {
		srv, _, _ := capture(t, tc.body, tc.status)
		c, _ := New(Config{Provider: "openai", BaseURL: srv.URL, Model: "m"})
		if _, err := Converse(context.Background(), c, "", nil, true); err == nil {
			t.Fatalf("accepted unusable response %s", tc.body)
		}
	}
	srv, _, _ := capture(t, `{"status":"completed","output":[{"type":"message","content":[{"type":"output_text","text":"Which brand?"}]}]}`, 200)
	c, _ := New(Config{Provider: "openai", BaseURL: srv.URL, Model: "m"})
	reply, err := Converse(context.Background(), c, "", nil, true)
	if err != nil || reply.Searched || !strings.Contains(reply.Text, "brand") {
		t.Fatal("tool availability is not proof a search happened")
	}
}
func TestPlainChatUsesHistoryWithoutJSONMode(t *testing.T) {
	srv, req, body := capture(t, `{"choices":[{"message":{"content":"Use 150 g cooked rice."}}]}`, 200)
	c, _ := New(Config{Provider: "openai", BaseURL: srv.URL, Model: "m", JSONMode: true})
	_, err := Converse(context.Background(), c, "system", []ChatMessage{{Role: "user", Content: "How much rice?"}}, false)
	if err != nil || req.URL.Path != "/chat/completions" || len((*body)["messages"].([]any)) != 2 {
		t.Fatalf("plain chat: %v %+v", err, body)
	}
	if _, ok := (*body)["tools"]; ok {
		t.Fatal("search disabled but tools were sent")
	}
	if _, ok := (*body)["response_format"]; ok {
		t.Fatal("food chat should be natural conversation")
	}
}
