package tools

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// WebSearch uses the DuckDuckGo Instant Answer API — no API key required.
type WebSearch struct {
	client *http.Client
}

func NewWebSearch() *WebSearch {
	return &WebSearch{client: &http.Client{Timeout: 15 * time.Second}}
}

func (w *WebSearch) Name() string { return "web_search" }

func (w *WebSearch) Description() string {
	return "Search the web using DuckDuckGo. Returns a summary and related topics for the query. Use this to look up current events, facts, documentation, or anything you don't know."
}

func (w *WebSearch) Schema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"query": map[string]any{
				"type":        "string",
				"description": "The search query to look up on the web.",
			},
		},
		"required": []string{"query"},
	}
}

type webSearchInput struct {
	Query string `json:"query"`
}

// ddgResponse is a minimal parse of the DuckDuckGo Instant Answer JSON.
type ddgResponse struct {
	Abstract        string `json:"Abstract"`
	AbstractSource  string `json:"AbstractSource"`
	AbstractURL     string `json:"AbstractURL"`
	Answer          string `json:"Answer"`
	Definition      string `json:"Definition"`
	DefinitionSource string `json:"DefinitionSource"`
	RelatedTopics   []struct {
		Text     string `json:"Text"`
		FirstURL string `json:"FirstURL"`
		Topics   []struct {
			Text     string `json:"Text"`
			FirstURL string `json:"FirstURL"`
		} `json:"Topics"`
	} `json:"RelatedTopics"`
}

func (w *WebSearch) Execute(raw json.RawMessage) (string, error) {
	var input webSearchInput
	if err := json.Unmarshal(raw, &input); err != nil {
		return "", fmt.Errorf("invalid input: %w", err)
	}
	if strings.TrimSpace(input.Query) == "" {
		return "", fmt.Errorf("query cannot be empty")
	}
	if len(input.Query) > 500 {
		return "", fmt.Errorf("query too long (max 500 characters)")
	}

	apiURL := fmt.Sprintf(
		"https://api.duckduckgo.com/?q=%s&format=json&no_html=1&skip_disambig=1",
		url.QueryEscape(input.Query),
	)

	resp, err := w.client.Get(apiURL)
	if err != nil {
		return "", fmt.Errorf("HTTP request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read response: %w", err)
	}

	var ddg ddgResponse
	if err := json.Unmarshal(body, &ddg); err != nil {
		return "", fmt.Errorf("failed to parse DDG response: %w", err)
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Search results for: %q\n\n", input.Query))

	if ddg.Answer != "" {
		sb.WriteString(fmt.Sprintf("Direct Answer: %s\n\n", ddg.Answer))
	}
	if ddg.Abstract != "" {
		sb.WriteString(fmt.Sprintf("Summary (%s): %s\n", ddg.AbstractSource, ddg.Abstract))
		if ddg.AbstractURL != "" {
			sb.WriteString(fmt.Sprintf("Source: %s\n", ddg.AbstractURL))
		}
		sb.WriteString("\n")
	}
	if ddg.Definition != "" {
		sb.WriteString(fmt.Sprintf("Definition (%s): %s\n\n", ddg.DefinitionSource, ddg.Definition))
	}

	// Collect up to 5 related topics.
	count := 0
	if len(ddg.RelatedTopics) > 0 {
		sb.WriteString("Related Topics:\n")
		for _, t := range ddg.RelatedTopics {
			if count >= 5 {
				break
			}
			if t.Text != "" {
				sb.WriteString(fmt.Sprintf("  - %s\n    %s\n", t.Text, t.FirstURL))
				count++
			}
			// Handle nested topic groups
			for _, sub := range t.Topics {
				if count >= 5 {
					break
				}
				if sub.Text != "" {
					sb.WriteString(fmt.Sprintf("  - %s\n    %s\n", sub.Text, sub.FirstURL))
					count++
				}
			}
		}
	}

	result := sb.String()
	if strings.TrimSpace(result) == fmt.Sprintf("Search results for: %q\n\n", input.Query) {
		return fmt.Sprintf("No results found for %q. "+
			"The DuckDuckGo Instant Answer API only returns results for well-known entities and topics — "+
			"it does not support local business, product, or price searches. "+
			"Do NOT retry with similar queries. Instead, use fetch_url with a specific URL to get the information directly.", input.Query), nil
	}
	return result, nil
}
