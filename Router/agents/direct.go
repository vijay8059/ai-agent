// Package agents contains the three agent types the router can dispatch to.
package agents

import (
	"context"
	"fmt"
	"strings"

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/vijay8059/ai-agent/Router/llm"
)

// DirectAgent answers simple factual questions in a single LLM call — no tools.
// Use for: definitions, explanations, conversions, math, general knowledge.
type DirectAgent struct {
	llm *llm.Client
}

func NewDirectAgent(client *llm.Client) *DirectAgent {
	return &DirectAgent{llm: client}
}

const directSystem = `You are a knowledgeable assistant. Answer the user's question directly and concisely.
You do not have access to tools — rely on your training knowledge.
Be factual, clear, and well-structured.`

// Run answers the query in a single LLM call and returns the answer.
func (a *DirectAgent) Run(ctx context.Context, query string) (string, error) {
	messages := []anthropic.MessageParam{
		anthropic.NewUserMessage(anthropic.NewTextBlock(query)),
	}

	resp, err := a.llm.Chat(ctx, llm.DirectModel, directSystem, messages, nil)
	if err != nil {
		return "", fmt.Errorf("direct agent error: %w", err)
	}

	var sb strings.Builder
	for _, block := range resp.Content {
		if tb, ok := block.AsAny().(anthropic.TextBlock); ok {
			sb.WriteString(tb.Text)
		}
	}
	return strings.TrimSpace(sb.String()), nil
}
