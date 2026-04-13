package tools

import (
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"
	"unicode/utf8"
)

const maxBodyBytes = 32 * 1024 // 32 KB

// allowedRequestHeaders is the set of headers the LLM is permitted to set.
var allowedRequestHeaders = map[string]bool{
	"Accept":          true,
	"Accept-Language": true,
	"Accept-Encoding": true,
	"Cache-Control":   true,
	"Content-Type":    true,
	"Range":           true,
}

// isPrivateIP reports whether the IP is loopback, private, or link-local.
func isPrivateIP(ip net.IP) bool {
	return ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast()
}

// validateFetchURL rejects private/internal addresses to prevent SSRF.
func validateFetchURL(rawURL string) error {
	if !strings.HasPrefix(rawURL, "http://") && !strings.HasPrefix(rawURL, "https://") {
		return fmt.Errorf("URL must start with http:// or https://")
	}
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return fmt.Errorf("invalid URL")
	}
	host := parsed.Hostname()
	// Block IP literals directly.
	if ip := net.ParseIP(host); ip != nil {
		if isPrivateIP(ip) {
			return fmt.Errorf("requests to internal addresses not allowed")
		}
		return nil
	}
	// Resolve hostname and check all resulting IPs.
	addrs, err := net.LookupHost(host)
	if err != nil {
		return nil // unresolvable host — let the HTTP client fail naturally
	}
	for _, addr := range addrs {
		if ip := net.ParseIP(addr); ip != nil && isPrivateIP(ip) {
			return fmt.Errorf("requests to internal addresses not allowed")
		}
	}
	return nil
}

// FetchURL fetches the raw content of any URL.
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
				"description": "Optional HTTP headers to include.",
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
	if err := validateFetchURL(input.URL); err != nil {
		return "", err
	}

	req, err := http.NewRequest(http.MethodGet, input.URL, nil)
	if err != nil {
		return "", fmt.Errorf("failed to build request")
	}
	req.Header.Set("User-Agent", "ai-agent/1.0 (educational project)")
	for k, v := range input.Headers {
		if allowedRequestHeaders[k] {
			req.Header.Set(k, v)
		}
	}

	resp, err := f.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("request failed")
	}
	defer resp.Body.Close()

	limited := io.LimitReader(resp.Body, maxBodyBytes)
	body, err := io.ReadAll(limited)
	if err != nil {
		return "", fmt.Errorf("failed to read body")
	}

	if !utf8.Valid(body) {
		return fmt.Sprintf("Status: %d\nNote: response body contains binary data and cannot be displayed as text.", resp.StatusCode), nil
	}

	truncated := ""
	if int64(len(body)) == maxBodyBytes {
		truncated = "\n\n[... response truncated at 32KB ...]"
	}

	return fmt.Sprintf("Status: %d\nURL: %s\n\n%s%s", resp.StatusCode, input.URL, string(body), truncated), nil
}
