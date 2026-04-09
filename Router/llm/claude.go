package llm

import (
	"context"
	"fmt"
	"os"

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/anthropics/anthropic-sdk-go/option"
)

// Model tiers — right model for the right job.
const (
	RouterModel       = anthropic.ModelClaudeHaiku4_5_20251001  // fast classification
	DirectModel       = anthropic.ModelClaudeHaiku4_5_20251001  // simple Q&A
	PlanModel         = anthropic.ModelClaudeHaiku4_5_20251001  // plan-execute steps
	OrchestratorModel = anthropic.ModelClaudeSonnet4_6          // dynamic decisions
	WorkerModel       = anthropic.ModelClaudeHaiku4_5_20251001  // specialist execution
)

// Client wraps the Anthropic SDK.
type Client struct {
	inner *anthropic.Client
}

func NewClient() (*Client, error) {
	key := os.Getenv("ANTHROPIC_API_KEY")
	if key == "" {
		return nil, fmt.Errorf("ANTHROPIC_API_KEY is not set")
	}
	c := anthropic.NewClient(option.WithAPIKey(key))
	return &Client{inner: &c}, nil
}

// Chat sends a message using the specified model.
func (c *Client) Chat(
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
