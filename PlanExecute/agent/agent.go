// Package agent implements the Plan-and-Execute agent pattern.
//
// Flow for every user message:
//
//  1. PLAN  — Ask Claude to decompose the task into an ordered list of steps (JSON).
//  2. EXECUTE — For each step, run a mini ReAct loop (think → tool call → observe)
//               until Claude produces a result for that step.
//  3. SYNTHESIZE — Send all step results back to Claude for a final answer.
package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/vijay8059/ai-agent/PlanExecute/llm"
	"github.com/vijay8059/ai-agent/PlanExecute/tools"
)

const maxStepIterations = 10 // max ReAct iterations per step

// ── Prompts ──────────────────────────────────────────────────────────────────

const plannerSystem = `You are a planning assistant. Your job is to break a user task into clear, ordered steps.

Return ONLY a JSON object in exactly this format — no prose, no markdown fences:
{
  "steps": [
    {"id": 1, "description": "..."},
    {"id": 2, "description": "..."}
  ]
}

Rules:
- Each step must be self-contained and actionable.
- Use 2–6 steps. Avoid over-planning trivial tasks.
- Do NOT include a "synthesize results" or "write final answer" step — that is handled automatically.`

const executorSystem = `You are an execution assistant. You are carrying out ONE step of a larger plan.

You have access to tools. Use them to complete the step. When you have gathered enough information,
write a concise result summary (plain text, no tool calls) and stop.

Context about the overall task and previous step results will be provided.`

const synthesizerSystem = `You are a synthesis assistant. Given an original task and the results of each execution step,
write a clear, comprehensive final answer for the user. Be direct and well-structured.`

// ── Types ─────────────────────────────────────────────────────────────────────

// Step is one event emitted during agent execution — useful for live terminal output.
type Step struct {
	Type    StepType
	Content string
}

type StepType string

const (
	StepPlan      StepType = "PLAN"      // full plan produced
	StepExecute   StepType = "EXECUTE"   // starting a plan step
	StepAct       StepType = "ACT"       // tool call within a step
	StepObserve   StepType = "OBSERVE"   // tool result
	StepResult    StepType = "RESULT"    // step completed with result
	StepSynthesize StepType = "SYNTHESIZE" // building final answer
	StepAnswer    StepType = "ANSWER"    // final answer ready
)

// planStep is one item in the JSON plan returned by Claude.
type planStep struct {
	ID          int    `json:"id"`
	Description string `json:"description"`
}

type plan struct {
	Steps []planStep `json:"steps"`
}

// Agent holds the LLM client, tool registry, and conversation history.
type Agent struct {
	llm      *llm.Client
	registry *tools.Registry
	history  []anthropic.MessageParam // cross-turn history for multi-turn sessions
	OnStep   func(Step)               // optional: stream events to the caller
}

// New creates an Agent with all built-in tools registered.
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

// ResetHistory clears conversation history so the next Run starts fresh.
func (a *Agent) ResetHistory() {
	a.history = nil
}

// ── Public entry point ────────────────────────────────────────────────────────

// Run executes the Plan-and-Execute loop for a user message and returns the final answer.
func (a *Agent) Run(ctx context.Context, userMessage string) (string, error) {
	// ── 1. PLAN ───────────────────────────────────────────────────────────────
	p, err := a.buildPlan(ctx, userMessage)
	if err != nil {
		return "", fmt.Errorf("planning failed: %w", err)
	}

	var planLines strings.Builder
	for _, s := range p.Steps {
		planLines.WriteString(fmt.Sprintf("  Step %d: %s\n", s.ID, s.Description))
	}
	a.emit(Step{Type: StepPlan, Content: planLines.String()})

	// ── 2. EXECUTE each step ──────────────────────────────────────────────────
	stepResults := make([]string, 0, len(p.Steps))

	for _, ps := range p.Steps {
		a.emit(Step{
			Type:    StepExecute,
			Content: fmt.Sprintf("[Step %d] %s", ps.ID, ps.Description),
		})

		result, err := a.executeStep(ctx, userMessage, p.Steps, ps, stepResults)
		if err != nil {
			result = fmt.Sprintf("Step %d failed: %s", ps.ID, err.Error())
		}

		a.emit(Step{
			Type:    StepResult,
			Content: fmt.Sprintf("[Step %d result] %s", ps.ID, truncate(result, 400)),
		})
		stepResults = append(stepResults, fmt.Sprintf("Step %d (%s):\n%s", ps.ID, ps.Description, result))
	}

	// ── 3. SYNTHESIZE ─────────────────────────────────────────────────────────
	a.emit(Step{Type: StepSynthesize, Content: "Combining step results into final answer…"})

	answer, err := a.synthesize(ctx, userMessage, stepResults)
	if err != nil {
		return "", fmt.Errorf("synthesis failed: %w", err)
	}

	// Store in history so multi-turn sessions have context.
	a.history = append(a.history,
		anthropic.NewUserMessage(anthropic.NewTextBlock(userMessage)),
		anthropic.NewAssistantMessage(anthropic.NewTextBlock(answer)),
	)

	a.emit(Step{Type: StepAnswer, Content: answer})
	return answer, nil
}

// ── Private helpers ───────────────────────────────────────────────────────────

// buildPlan asks Claude to decompose the task into JSON steps.
func (a *Agent) buildPlan(ctx context.Context, task string) (*plan, error) {
	// Include prior conversation history as context for the planner.
	messages := append(a.history, anthropic.NewUserMessage(
		anthropic.NewTextBlock("Task: "+task),
	))

	resp, err := a.llm.Chat(ctx, plannerSystem, messages, nil)
	if err != nil {
		return nil, err
	}

	// Extract the text block from the response.
	var raw string
	for _, block := range resp.Content {
		if tb, ok := block.AsAny().(anthropic.TextBlock); ok {
			raw = strings.TrimSpace(tb.Text)
			break
		}
	}

	// Strip markdown code fences if the model added them.
	raw = stripFences(raw)

	var p plan
	if err := json.Unmarshal([]byte(raw), &p); err != nil {
		return nil, fmt.Errorf("could not parse plan JSON (%q): %w", raw, err)
	}
	if len(p.Steps) == 0 {
		return nil, fmt.Errorf("planner returned zero steps")
	}
	return &p, nil
}

// executeStep runs a mini ReAct loop for a single plan step.
// It receives the full plan and results of previous steps for context.
func (a *Agent) executeStep(
	ctx context.Context,
	originalTask string,
	allSteps []planStep,
	current planStep,
	previousResults []string,
) (string, error) {
	// Build a rich context message for the executor.
	var ctx_msg strings.Builder
	ctx_msg.WriteString(fmt.Sprintf("Original task: %s\n\n", originalTask))

	ctx_msg.WriteString("Full plan:\n")
	for _, s := range allSteps {
		marker := "  "
		if s.ID == current.ID {
			marker = "→ "
		}
		ctx_msg.WriteString(fmt.Sprintf("%sStep %d: %s\n", marker, s.ID, s.Description))
	}

	if len(previousResults) > 0 {
		ctx_msg.WriteString("\nPrevious step results:\n")
		for _, r := range previousResults {
			ctx_msg.WriteString(r + "\n\n")
		}
	}

	ctx_msg.WriteString(fmt.Sprintf("\nYour task NOW: %s", current.Description))

	messages := []anthropic.MessageParam{
		anthropic.NewUserMessage(anthropic.NewTextBlock(ctx_msg.String())),
	}

	toolParams := a.registry.ToSDKParams()

	for i := 0; i < maxStepIterations; i++ {
		resp, err := a.llm.Chat(ctx, executorSystem, messages, toolParams)
		if err != nil {
			return "", fmt.Errorf("LLM error on step iteration %d: %w", i+1, err)
		}

		var textOut strings.Builder
		var toolUseBlocks []anthropic.ToolUseBlock

		for _, block := range resp.Content {
			switch b := block.AsAny().(type) {
			case anthropic.TextBlock:
				textOut.WriteString(b.Text)
			case anthropic.ToolUseBlock:
				toolUseBlocks = append(toolUseBlocks, b)
			}
		}

		// Append assistant turn to the step's message history.
		messages = append(messages, resp.ToParam())

		// No tool calls → step is done; the text is the result.
		if len(toolUseBlocks) == 0 {
			return strings.TrimSpace(textOut.String()), nil
		}

		// Execute tool calls and collect results.
		var toolResults []anthropic.ContentBlockParamUnion
		for _, tb := range toolUseBlocks {
			a.emit(Step{
				Type:    StepAct,
				Content: fmt.Sprintf("  %s(%s)", tb.Name, string(tb.Input)),
			})

			result, execErr := a.registry.Execute(tb.Name, tb.Input)
			if execErr != nil {
				result = fmt.Sprintf("Error: %s", execErr.Error())
			}

			a.emit(Step{
				Type:    StepObserve,
				Content: fmt.Sprintf("  → %s", truncate(result, 300)),
			})

			toolResults = append(toolResults, anthropic.NewToolResultBlock(tb.ID, result, false))
		}

		messages = append(messages, anthropic.NewUserMessage(toolResults...))
	}

	return "", fmt.Errorf("step exceeded %d iterations without a result", maxStepIterations)
}

// synthesize asks Claude to combine all step results into a final answer.
func (a *Agent) synthesize(ctx context.Context, task string, stepResults []string) (string, error) {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Original task: %s\n\nExecution results:\n\n", task))
	for _, r := range stepResults {
		sb.WriteString(r + "\n\n")
	}
	sb.WriteString("Write the final answer for the user.")

	messages := []anthropic.MessageParam{
		anthropic.NewUserMessage(anthropic.NewTextBlock(sb.String())),
	}

	resp, err := a.llm.Chat(ctx, synthesizerSystem, messages, nil)
	if err != nil {
		return "", err
	}

	var answer strings.Builder
	for _, block := range resp.Content {
		if tb, ok := block.AsAny().(anthropic.TextBlock); ok {
			answer.WriteString(tb.Text)
		}
	}
	return strings.TrimSpace(answer.String()), nil
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

// stripFences removes ```json ... ``` or ``` ... ``` wrappers if present.
func stripFences(s string) string {
	s = strings.TrimSpace(s)
	if strings.HasPrefix(s, "```") {
		lines := strings.SplitN(s, "\n", 2)
		if len(lines) == 2 {
			s = lines[1]
		}
		s = strings.TrimSuffix(s, "```")
		s = strings.TrimSpace(s)
	}
	return s
}
