package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/vijay8059/ai-agent/MultiAgent/llm"
)

const maxOrchestratorIterations = 20

// ── Step types ────────────────────────────────────────────────────────────────

// Step is one event emitted during agent execution — used for live terminal output.
type Step struct {
	Type    StepType
	Content string
}

type StepType string

const (
	StepDelegate     StepType = "DELEGATE"      // orchestrator delegating to a worker
	StepWorkerAct    StepType = "WORKER_ACT"    // worker calling a tool
	StepWorkerObserve StepType = "WORKER_OBS"   // worker received a tool result
	StepWorkerResult StepType = "WORKER_RESULT" // worker completed with a result
	StepAnswer       StepType = "ANSWER"        // orchestrator produced the final answer
)

// ── Orchestrator ──────────────────────────────────────────────────────────────

const orchestratorSystem = `You are an orchestrator agent. Your job is to solve a user's task by dynamically
delegating sub-tasks to specialist workers using the delegate_to_worker tool.

Available workers:
- "research"  → searches the web for information (use for: facts, news, prices, comparisons)
- "fetch"     → retrieves content from specific URLs (use for: reading a page, API responses)
- "file"      → reads, writes, and lists local files (use for: saving results, reading configs)
- "general"   → can use all tools (use for: complex tasks that don't fit one specialty)

How to work:
1. Analyze the task and decide which worker to delegate to first.
2. Read the worker's result carefully.
3. Based on the result, decide whether to delegate another sub-task, or produce a final answer.
4. When you have enough information, write a comprehensive final answer as plain text (no tool calls).

IMPORTANT: You make decisions AFTER seeing each result. Adapt your plan based on what workers find.
Do NOT pre-plan all steps — react to what you learn.`

// delegateInput is the JSON input for the delegate_to_worker tool.
type delegateInput struct {
	Worker WorkerType `json:"worker"`
	Goal   string     `json:"goal"`
}

// Orchestrator is the top-level agent that dynamically delegates to workers.
type Orchestrator struct {
	llm     *llm.Client
	history []anthropic.MessageParam // cross-turn conversation memory
	OnStep  func(Step)               // optional: stream events to caller
}

// New creates an Orchestrator.
func New(client *llm.Client) *Orchestrator {
	return &Orchestrator{llm: client}
}

// ResetHistory clears conversation history so the next Run starts fresh.
func (o *Orchestrator) ResetHistory() {
	o.history = nil
}

// Run executes the multi-agent loop for a user message and returns the final answer.
//
// Loop:
//  1. Orchestrator (Sonnet) receives the task + conversation history.
//  2. Orchestrator calls delegate_to_worker(worker, goal).
//  3. The named worker runs its own ReAct loop and returns a result.
//  4. Result is fed back to the orchestrator.
//  5. Repeat until orchestrator writes a final answer (no tool calls).
func (o *Orchestrator) Run(ctx context.Context, userMessage string) (string, error) {
	// Build the delegate_to_worker tool that the orchestrator can call.
	delegateTool := anthropic.ToolUnionParam{
		OfTool: &anthropic.ToolParam{
			Name: "delegate_to_worker",
			Description: anthropic.String(
				`Delegate a sub-task to a specialist worker agent. The worker will autonomously ` +
					`complete the goal using its tools and return a result. ` +
					`Choose the worker that best matches the sub-task.`,
			),
			InputSchema: anthropic.ToolInputSchemaParam{
				Properties: map[string]any{
					"worker": map[string]any{
						"type":        "string",
						"enum":        []string{"research", "fetch", "file", "general"},
						"description": "The specialist worker to delegate to.",
					},
					"goal": map[string]any{
						"type":        "string",
						"description": "A clear, self-contained goal for the worker to accomplish.",
					},
				},
				Required: []string{"worker", "goal"},
			},
		},
	}

	// Start orchestrator message history with prior conversation + new user message.
	messages := append(o.history, anthropic.NewUserMessage(
		anthropic.NewTextBlock(userMessage),
	))

	for i := 0; i < maxOrchestratorIterations; i++ {
		resp, err := o.llm.ChatAs(ctx, llm.OrchestratorModel, orchestratorSystem, messages, []anthropic.ToolUnionParam{delegateTool})
		if err != nil {
			return "", fmt.Errorf("orchestrator LLM error: %w", err)
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

		messages = append(messages, resp.ToParam())

		// No tool calls → orchestrator has written the final answer.
		if len(toolUseBlocks) == 0 {
			answer := strings.TrimSpace(textOut.String())

			// Persist this turn to conversation history.
			o.history = append(o.history,
				anthropic.NewUserMessage(anthropic.NewTextBlock(userMessage)),
				anthropic.NewAssistantMessage(anthropic.NewTextBlock(answer)),
			)

			o.emit(Step{Type: StepAnswer, Content: answer})
			return answer, nil
		}

		// Process each delegate_to_worker call.
		var toolResults []anthropic.ContentBlockParamUnion

		for _, tb := range toolUseBlocks {
			var input delegateInput
			if err := json.Unmarshal(tb.Input, &input); err != nil {
				result := fmt.Sprintf("Error: could not parse delegate input: %s", err)
				toolResults = append(toolResults, anthropic.NewToolResultBlock(tb.ID, result, true))
				continue
			}

			o.emit(Step{
				Type:    StepDelegate,
				Content: fmt.Sprintf("→ [%s worker] %s", input.Worker, input.Goal),
			})

			// Spin up the worker and run it.
			w := newWorker(input.Worker, o.llm, o.OnStep)
			workerResult, workerErr := w.Run(ctx, input.Goal)
			if workerErr != nil {
				workerResult = fmt.Sprintf("Worker error: %s", workerErr.Error())
			}

			o.emit(Step{
				Type:    StepWorkerResult,
				Content: fmt.Sprintf("[%s] %s", input.Worker, truncate(workerResult, 400)),
			})

			toolResults = append(toolResults, anthropic.NewToolResultBlock(tb.ID, workerResult, false))
		}

		messages = append(messages, anthropic.NewUserMessage(toolResults...))
	}

	return "", fmt.Errorf("orchestrator exceeded %d iterations without a final answer", maxOrchestratorIterations)
}

func (o *Orchestrator) emit(s Step) {
	if o.OnStep != nil {
		o.OnStep(s)
	}
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}
