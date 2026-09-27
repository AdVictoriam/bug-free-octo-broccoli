package studio

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"
)

type Providers struct {
	OpenAIKey    string
	AnthropicKey string
	Client       *http.Client
}

func NewProviders(openai, anthropic string) *Providers {
	return &Providers{openai, anthropic, &http.Client{Timeout: 4 * time.Minute, Transport: &http.Transport{MaxIdleConns: 4, MaxConnsPerHost: 2, IdleConnTimeout: 60 * time.Second, TLSHandshakeTimeout: 15 * time.Second}}}
}
func (p *Providers) Ready(r Role) error {
	if strings.TrimSpace(r.Model) == "" {
		return errors.New("set an exact model ID in Model settings")
	}
	switch r.Provider {
	case "openai":
		if p.OpenAIKey == "" {
			return errors.New("OPENAI_API_KEY is not configured on the server")
		}
	case "anthropic":
		if p.AnthropicKey == "" {
			return errors.New("ANTHROPIC_API_KEY is not configured on the server")
		}
	default:
		return errors.New("unsupported model provider")
	}
	return nil
}
func (p *Providers) JSON(ctx context.Context, r Role, system, user string, schema map[string]any, maxTokens int) (string, Usage, error) {
	return p.call(ctx, r, system, user, schema, maxTokens, false)
}
func (p *Providers) Search(ctx context.Context, r Role, system, user string) (SearchResult, Usage, error) {
	raw, u, e := p.call(ctx, r, system, user, nil, 7000, true)
	if e != nil {
		return SearchResult{}, u, e
	}
	var result SearchResult
	e = json.Unmarshal([]byte(raw), &result)
	return result, u, e
}
func (p *Providers) call(ctx context.Context, r Role, system, user string, schema map[string]any, maxTokens int, search bool) (string, Usage, error) {
	if e := p.Ready(r); e != nil {
		return "", Usage{}, e
	}
	var endpoint string
	var body map[string]any
	headers := map[string]string{}
	if r.Provider == "openai" {
		endpoint = "https://api.openai.com/v1/responses"
		headers["Authorization"] = "Bearer " + p.OpenAIKey
		body = map[string]any{"model": r.Model, "store": false, "instructions": system, "input": user, "max_output_tokens": maxTokens}
		if schema != nil {
			body["text"] = map[string]any{"format": map[string]any{"type": "json_schema", "name": "bolty_output", "strict": true, "schema": schema}}
		}
		if search {
			body["tools"] = []any{map[string]any{"type": "web_search", "search_context_size": "medium", "filters": map[string]any{"allowed_domains": OfficialDomains}}}
			body["tool_choice"] = "required"
		}
	} else {
		endpoint = "https://api.anthropic.com/v1/messages"
		headers["x-api-key"] = p.AnthropicKey
		headers["anthropic-version"] = "2023-06-01"
		body = map[string]any{"model": r.Model, "max_tokens": maxTokens, "system": system, "messages": []any{map[string]any{"role": "user", "content": user}}}
		if schema != nil {
			body["output_config"] = map[string]any{"format": map[string]any{"type": "json_schema", "schema": schema}}
		}
		if search {
			body["tools"] = []any{map[string]any{"type": "web_search_20250305", "name": "web_search", "max_uses": 4, "allowed_domains": OfficialDomains}}
		}
	}
	payload, e := json.Marshal(body)
	if e != nil {
		return "", Usage{}, e
	}
	data, requestID, e := p.post(ctx, endpoint, headers, payload)
	u := Usage{RequestID: requestID}
	if e != nil {
		return "", u, e
	}
	var obj map[string]any
	if e = json.Unmarshal(data, &obj); e != nil {
		return "", u, errors.New("provider returned invalid JSON")
	}
	if v, ok := obj["usage"].(map[string]any); ok {
		u.Input = int(number(v["input_tokens"]))
		u.Output = int(number(v["output_tokens"]))
	}
	var texts []string
	var annotations []Citation
	if r.Provider == "openai" {
		if obj["status"] != "completed" {
			return "", u, fmt.Errorf("provider did not complete the response (status %v); no draft accepted", obj["status"])
		}
		if out, ok := obj["output"].([]any); ok {
			for _, item := range out {
				v, _ := item.(map[string]any)
				if v["type"] == "message" {
					blocks, _ := v["content"].([]any)
					for _, block := range blocks {
						b, _ := block.(map[string]any)
						if b["type"] == "refusal" {
							return "", u, errors.New("provider declined the request; no draft accepted")
						}
						if b["type"] == "output_text" {
							t, _ := b["text"].(string)
							texts = append(texts, t)
							annotations = append(annotations, collectCitations(b["annotations"])...)
						}
					}
				}
			}
		}
	} else {
		stop, _ := obj["stop_reason"].(string)
		if stop != "end_turn" {
			return "", u, fmt.Errorf("provider stopped with %s; incomplete/refused content was not accepted", stop)
		}
		if out, ok := obj["content"].([]any); ok {
			for _, item := range out {
				v, _ := item.(map[string]any)
				if v["type"] == "text" {
					t, _ := v["text"].(string)
					texts = append(texts, t)
					annotations = append(annotations, collectCitations(v["citations"])...)
				}
			}
		}
	}
	text := strings.Join(texts, "\n")
	if strings.TrimSpace(text) == "" {
		return "", u, errors.New("provider returned no text")
	}
	if search {
		unique := []Citation{}
		seen := map[string]bool{}
		for _, c := range annotations {
			if allowedOfficial(c.URL) && !seen[c.URL] {
				seen[c.URL] = true
				unique = append(unique, c)
			}
		}
		if len(unique) == 0 {
			return "", u, errors.New("research returned no actual official-source citation objects; cannot verify mechanics")
		}
		return jsonString(SearchResult{Text: text, Citations: unique}), u, nil
	}
	if !json.Valid([]byte(text)) {
		return "", u, errors.New("structured output is incomplete or invalid JSON")
	}
	return text, u, nil
}
func collectCitations(v any) []Citation {
	out := []Citation{}
	arr, _ := v.([]any)
	for _, a := range arr {
		m, _ := a.(map[string]any)
		url, _ := m["url"].(string)
		title, _ := m["title"].(string)
		if url != "" {
			out = append(out, Citation{url, title})
		}
	}
	return out
}
func number(v any) float64 { n, _ := v.(float64); return n }
func (p *Providers) post(ctx context.Context, endpoint string, headers map[string]string, payload []byte) ([]byte, string, error) {
	for attempt := 0; attempt < 3; attempt++ {
		req, e := http.NewRequestWithContext(ctx, "POST", endpoint, bytes.NewReader(payload))
		if e != nil {
			return nil, "", e
		}
		req.Header.Set("Content-Type", "application/json")
		for k, v := range headers {
			req.Header.Set(k, v)
		}
		resp, e := p.Client.Do(req)
		if e != nil {
			return nil, "", errors.New("provider transport failed or timed out; not auto-replayed because billing is uncertain")
		}
		id := resp.Header.Get("x-request-id")
		if id == "" {
			id = resp.Header.Get("request-id")
		}
		data, e := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
		resp.Body.Close()
		if e != nil {
			return nil, id, errors.New("provider response could not be read")
		}
		if (resp.StatusCode == 429 || resp.StatusCode == 503) && attempt < 2 {
			seconds := 1 << attempt
			if v, err := strconv.Atoi(resp.Header.Get("Retry-After")); err == nil && v > seconds {
				seconds = v
			}
			if seconds > 30 {
				seconds = 30
			}
			timer := time.NewTimer(time.Duration(seconds) * time.Second)
			select {
			case <-ctx.Done():
				timer.Stop()
				return nil, id, ctx.Err()
			case <-timer.C:
				continue
			}
		}
		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			return nil, id, fmt.Errorf("provider HTTP %d (request %s). Check model access, API quota and server configuration; provider body not exposed", resp.StatusCode, id)
		}
		return data, id, nil
	}
	return nil, "", errors.New("provider retry budget exhausted")
}
