package llm

import (
	"context"
	"fmt"
	"github.com/anthropics/anthropic-sdk-go"
	"os"

	"github.com/anthropics/anthropic-sdk-go/option"
)

const Model = anthropic.ModelClaudeHaiku4_5_20251001

// Client wraps the Anthropic SDK client.
type Client struct {
	inner *anthropic.Client
}

// NewClient reads ANTHROPIC_API_KEY from env and returns a ready client.
func NewClient() (*Client, error) {
	key := os.Getenv("ANTHROPIC_API_KEY")
	if key == "" {
		return nil, fmt.Errorf("ANTHROPIC_API_KEY is not set")
	}
	c := anthropic.NewClient(option.WithAPIKey(key))
	return &Client{inner: &c}, nil
}

// Chat sends the message history plus available tools to Claude and returns the response.
func (c *Client) Chat(
	ctx context.Context,
	system string,
	messages []anthropic.MessageParam,
	tools []anthropic.ToolUnionParam,
) (*anthropic.Message, error) {
	params := anthropic.MessageNewParams{
		Model:     Model,
		MaxTokens: 8096,
		Messages:  messages,
	}
	if system != "" {
		params.System = []anthropic.TextBlockParam{
			{Text: system},
		}
	}
	if len(tools) > 0 {
		params.Tools = tools
	}

	resp, err := c.inner.Messages.New(ctx, params)
	if err != nil {
		return nil, fmt.Errorf("claude API error: %w", err)
	}
	return resp, nil
}
