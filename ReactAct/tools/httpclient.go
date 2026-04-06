package tools

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
	"unicode/utf8"
)

const maxBodyBytes = 32 * 1024 // 32 KB — enough for most pages, keeps context lean

// FetchURL fetches the raw content of any URL.
// Great for reading API responses, documentation pages, JSON endpoints, etc.
type FetchURL struct {
	client *http.Client
}

func NewFetchURL() *FetchURL {
	return &FetchURL{client: &http.Client{Timeout: 20 * time.Second}}
}

func (f *FetchURL) Name() string { return "fetch_url" }

func (f *FetchURL) Description() string {
	return "Fetch the content of any URL via HTTP GET. Returns the response body as text (truncated at 32KB). Use this to read API responses, JSON data, documentation pages, or any web content."
}

func (f *FetchURL) Schema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"url": map[string]any{
				"type":        "string",
				"description": "The full URL to fetch (must start with http:// or https://).",
			},
			"headers": map[string]any{
				"type":        "object",
				"description": "Optional HTTP headers to include (e.g. {\"Accept\": \"application/json\"}).",
				"additionalProperties": map[string]any{
					"type": "string",
				},
			},
		},
		"required": []string{"url"},
	}
}

type fetchURLInput struct {
	URL     string            `json:"url"`
	Headers map[string]string `json:"headers"`
}

func (f *FetchURL) Execute(raw json.RawMessage) (string, error) {
	var input fetchURLInput
	if err := json.Unmarshal(raw, &input); err != nil {
		return "", fmt.Errorf("invalid input: %w", err)
	}
	if !strings.HasPrefix(input.URL, "http://") && !strings.HasPrefix(input.URL, "https://") {
		return "", fmt.Errorf("URL must start with http:// or https://")
	}

	req, err := http.NewRequest(http.MethodGet, input.URL, nil)
	if err != nil {
		return "", fmt.Errorf("failed to build request: %w", err)
	}
	req.Header.Set("User-Agent", "ai-agent/1.0 (educational project)")
	for k, v := range input.Headers {
		req.Header.Set(k, v)
	}

	resp, err := f.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	limited := io.LimitReader(resp.Body, maxBodyBytes)
	body, err := io.ReadAll(limited)
	if err != nil {
		return "", fmt.Errorf("failed to read body: %w", err)
	}

	// Ensure valid UTF-8 — binary responses get a notice instead.
	if !utf8.Valid(body) {
		return fmt.Sprintf("Status: %d\nNote: response body contains binary data and cannot be displayed as text.", resp.StatusCode), nil
	}

	truncated := ""
	if int64(len(body)) == maxBodyBytes {
		truncated = "\n\n[... response truncated at 32KB ...]"
	}

	return fmt.Sprintf("Status: %d\nURL: %s\n\n%s%s", resp.StatusCode, input.URL, string(body), truncated), nil
}
