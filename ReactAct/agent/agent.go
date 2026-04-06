// Package agent implements the ReAct (Reason + Act) agent loop.
//
// Flow for every user message:
//
//  1. Build messages slice (system + history + new user message)
//  2. Call Claude with available tools              ← THINK
//  3. If response has tool_use blocks → execute them ← ACT + OBSERVE
//  4. Append tool results to messages and loop back to step 2
//  5. When Claude returns stop_reason="end_turn" with no tool calls → done
package agent

import (
	"context"
	"fmt"
	"strings"

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/vijay8059/ai-agent/ReactAct/llm"
	"github.com/vijay8059/ai-agent/ReactAct/tools"
)

const (
	maxIterations = 20 // safety: never loop forever
	systemPrompt  = `You are a capable AI agent with access to the following tools:
- web_search: search the web for information
- fetch_url: fetch content from any URL
- read_file: read files from the filesystem
- write_file: write files to the filesystem
- list_directory: list directory contents

When given a task:
1. Think about what you need to do.
2. Use tools to gather information or take actions.
3. Reason about the results before deciding the next step.
4. When you have enough information, provide a clear final answer.

Be thorough but efficient. Prefer using tools when you need real data rather than guessing.`
)

// Agent is the ReAct agent. It holds conversation history so multi-turn
// conversations work naturally — each call to Run continues the same session.
type Agent struct {
	llm      *llm.Client
	registry *tools.Registry
	history  []anthropic.MessageParam // accumulated conversation
	OnStep   func(step Step)          // optional: called after each Think/Act/Observe
}

// Step describes one event in the agent loop — useful for streaming output to the user.
type Step struct {
	Type    StepType
	Content string // human-readable description of what happened
}

type StepType string

const (
	StepThink   StepType = "THINK"   // Claude produced text reasoning
	StepAct     StepType = "ACT"     // Claude called a tool
	StepObserve StepType = "OBSERVE" // tool result fed back in
	StepAnswer  StepType = "ANSWER"  // final answer, loop ends
)

// New creates an Agent with all tools registered.
func New(client *llm.Client) *Agent {
	reg := tools.NewRegistry()
	reg.Register(tools.NewWebSearch())
	reg.Register(tools.NewFetchURL())
	reg.Register(tools.NewReadFile())
	reg.Register(tools.NewWriteFile())
	reg.Register(tools.NewListDirectory())

	return &Agent{
		llm:      client,
		registry: reg,
	}
}

// Run sends a user message through the ReAct loop and returns the final answer.
// It appends to history so subsequent calls continue the conversation.
func (a *Agent) Run(ctx context.Context, userMessage string) (string, error) {
	// Append the user's message to history.
	a.history = append(a.history, anthropic.NewUserMessage(
		anthropic.NewTextBlock(userMessage),
	))

	toolParams := a.registry.ToSDKParams()

	for i := 0; i < maxIterations; i++ {
		// ── THINK ────────────────────────────────────────────────────────────
		resp, err := a.llm.Chat(ctx, systemPrompt, a.history, toolParams)
		if err != nil {
			return "", fmt.Errorf("LLM error on iteration %d: %w", i+1, err)
		}

		// Collect text blocks the model returned (its visible reasoning).
		var thinkText strings.Builder
		var toolUseBlocks []anthropic.ToolUseBlock

		for _, block := range resp.Content {
			// AsAny() returns the concrete type so we can type-switch cleanly.
			switch b := block.AsAny().(type) {
			case anthropic.TextBlock:
				thinkText.WriteString(b.Text)
			case anthropic.ToolUseBlock:
				toolUseBlocks = append(toolUseBlocks, b)
			}
		}

		if thinkText.Len() > 0 {
			a.emit(Step{Type: StepThink, Content: thinkText.String()})
		}

		// Append the assistant's full response to history (required by the API).
		// resp.ToParam() converts the response Message → MessageParam correctly.
		a.history = append(a.history, resp.ToParam())

		// ── No tool calls → final answer ────────────────────────────────────
		if len(toolUseBlocks) == 0 {
			answer := thinkText.String()
			a.emit(Step{Type: StepAnswer, Content: answer})
			return answer, nil
		}

		// ── ACT + OBSERVE ────────────────────────────────────────────────────
		// Execute every tool call Claude requested, then send ALL results back
		// in a single user message (the API requires tool_result blocks to be
		// in a user turn immediately following the assistant turn).
		var toolResults []anthropic.ContentBlockParamUnion

		for _, tb := range toolUseBlocks {
			a.emit(Step{
				Type:    StepAct,
				Content: fmt.Sprintf("%s(%s)", tb.Name, string(tb.Input)),
			})

			result, execErr := a.registry.Execute(tb.Name, tb.Input)
			if execErr != nil {
				result = fmt.Sprintf("Error: %s", execErr.Error())
			}

			a.emit(Step{
				Type:    StepObserve,
				Content: fmt.Sprintf("[%s] → %s", tb.Name, truncate(result, 300)),
			})

			toolResults = append(toolResults, anthropic.NewToolResultBlock(tb.ID, result, false))
		}

		// Append tool results as a user message so the loop continues.
		a.history = append(a.history, anthropic.NewUserMessage(toolResults...))
	}

	return "", fmt.Errorf("agent exceeded %d iterations without a final answer", maxIterations)
}

// ResetHistory clears the conversation so the next Run starts fresh.
func (a *Agent) ResetHistory() {
	a.history = nil
}

func (a *Agent) emit(s Step) {
	if a.OnStep != nil {
		a.OnStep(s)
	}
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}
