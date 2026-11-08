package llm

import (
	"context"
	"encoding/json"
	"errors"
	"net/url"
	"strings"
)

type ChatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}
type Citation struct {
	URL   string `json:"url"`
	Title string `json:"title"`
	Start int    `json:"start"`
	End   int    `json:"end"`
}
type ChatReply struct {
	Text      string     `json:"text"`
	Citations []Citation `json:"citations"`
	Searched  bool       `json:"searched"`
}
type ChatClient interface {
	Chat(context.Context, string, []ChatMessage, bool) (ChatReply, error)
}

// Converse keeps conversation history separate from server instructions. Providers
// without a search-capable interface can still discuss food, but never fake search.
func Converse(ctx context.Context, c Client, system string, messages []ChatMessage, search bool) (ChatReply, error) {
	if chat, ok := c.(ChatClient); ok {
		return chat.Chat(ctx, system, messages, search)
	}
	if search {
		return ChatReply{}, errors.New("web search needs an OpenAI Responses-compatible connection; switch off web search to chat with this provider")
	}
	transcript, _ := json.Marshal(messages)
	text, err := c.Complete(ctx, system+"\nThe user input is the ordered conversation as JSON. Continue it with an assistant reply.", string(transcript))
	return ChatReply{Text: text, Citations: []Citation{}}, err
}

func (o *OpenAI) Chat(ctx context.Context, system string, messages []ChatMessage, search bool) (ChatReply, error) {
	headers := map[string]string{}
	if o.cfg.APIKey != "" {
		headers["Authorization"] = "Bearer " + o.cfg.APIKey
	}
	if !search {
		input := []ChatMessage{{Role: "system", Content: system}}
		input = append(input, messages...)
		body := map[string]any{"model": o.cfg.Model, "messages": input}
		if o.cfg.MaxTokens > 0 {
			body["max_completion_tokens"] = o.cfg.MaxTokens
		}
		var response struct {
			Choices []struct {
				Message ChatMessage `json:"message"`
			} `json:"choices"`
		}
		if err := postJSON(ctx, o.http, trimSlash(o.cfg.BaseURL)+"/chat/completions", headers, body, &response); err != nil {
			return ChatReply{}, err
		}
		if len(response.Choices) == 0 || strings.TrimSpace(response.Choices[0].Message.Content) == "" {
			return ChatReply{}, errors.New("the model returned an empty reply")
		}
		return ChatReply{Text: response.Choices[0].Message.Content, Citations: []Citation{}}, nil
	}
	// Do not use JSON mode here: retain native search output and citation annotations.
	body := map[string]any{"model": o.cfg.Model, "instructions": system, "input": messages, "store": false, "tools": []map[string]string{{"type": "web_search"}}, "tool_choice": "auto", "include": []string{"web_search_call.action.sources"}}
	if o.cfg.MaxTokens > 0 {
		body["max_output_tokens"] = o.cfg.MaxTokens
	}
	var response struct {
		Status string `json:"status"`
		Output []struct {
			Type    string `json:"type"`
			Status  string `json:"status"`
			Content []struct {
				Type        string `json:"type"`
				Text        string `json:"text"`
				Refusal     string `json:"refusal"`
				Annotations []struct {
					Type  string `json:"type"`
					URL   string `json:"url"`
					Title string `json:"title"`
					Start int    `json:"start_index"`
					End   int    `json:"end_index"`
				} `json:"annotations"`
			} `json:"content"`
		} `json:"output"`
	}
	if err := postJSON(ctx, o.http, trimSlash(o.cfg.BaseURL)+"/responses", headers, body, &response); err != nil {
		return ChatReply{}, err
	}
	if response.Status != "completed" {
		return ChatReply{}, errors.New("the web-enabled response did not finish; try again or shorten the conversation")
	}
	result := ChatReply{Citations: []Citation{}}
	for _, item := range response.Output {
		if item.Type == "web_search_call" && item.Status == "completed" {
			result.Searched = true
		}
		if item.Type != "message" {
			continue
		}
		for _, part := range item.Content {
			if part.Type == "refusal" {
				return ChatReply{}, errors.New(part.Refusal)
			}
			if part.Type != "output_text" {
				continue
			}
			if result.Text != "" {
				result.Text += "\n\n"
			}
			offset := len([]rune(result.Text))
			result.Text += part.Text
			for _, a := range part.Annotations {
				u, err := url.Parse(a.URL)
				if a.Type != "url_citation" || err != nil || (u.Scheme != "https" && u.Scheme != "http") || u.Host == "" || u.User != nil {
					continue
				}
				if a.Start < 0 || a.End < a.Start || a.End > len([]rune(part.Text)) {
					continue
				}
				result.Citations = append(result.Citations, Citation{URL: a.URL, Title: a.Title, Start: offset + a.Start, End: offset + a.End})
			}
		}
	}
	if strings.TrimSpace(result.Text) == "" {
		return ChatReply{}, errors.New("the model returned no answer")
	}
	return result, nil
}
