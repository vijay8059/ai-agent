package llm

import (
	"context"
	"fmt"
	"os"

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/anthropics/anthropic-sdk-go/option"
)

// OrchestratorModel is the most capable model — makes dynamic decisions.
const OrchestratorModel = anthropic.ModelClaudeSonnet4_6

// WorkerModel is a faster, cheaper model for individual task execution.
const WorkerModel = anthropic.ModelClaudeHaiku4_5_20251001

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

// ChatAs sends a message using the specified model.
func (c *Client) ChatAs(
	ctx context.Context,
	model anthropic.Model,
	system string,
	messages []anthropic.MessageParam,
	tools []anthropic.ToolUnionParam,
) (*anthropic.Message, error) {
	params := anthropic.MessageNewParams{
		Model:     model,
		MaxTokens: 8096,
		Messages:  messages,
	}
	if system != "" {
		params.System = []anthropic.TextBlockParam{{Text: system}}
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
