package agent

// Package agent implements the Multi-Agent pattern.
//
// Architecture:
//
//	Orchestrator (Sonnet) — dynamic decision maker
//	  └── delegates to Workers via the "delegate_to_worker" tool
//	        ├── ResearchWorker (Haiku) — web_search
//	        ├── FetchWorker    (Haiku) — fetch_url
//	        ├── FileWorker     (Haiku) — read_file, write_file, list_directory
//	        └── GeneralWorker  (Haiku) — all tools
//
// Key difference from Plan-Execute:
//   The orchestrator sees each worker's result BEFORE deciding the next action.
//   It can change course, spawn different workers, or stop — dynamically.

import (
	"context"
	"fmt"
	"strings"

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/vijay8059/ai-agent/MultiAgent/llm"
	"github.com/vijay8059/ai-agent/MultiAgent/tools"
)

const maxWorkerIterations = 10

// WorkerType identifies the kind of specialist worker.
type WorkerType string

const (
	WorkerResearch WorkerType = "research" // web_search only
	WorkerFetch    WorkerType = "fetch"    // fetch_url only
	WorkerFile     WorkerType = "file"     // read_file, write_file, list_directory
	WorkerGeneral  WorkerType = "general"  // all tools
)

var workerSystems = map[WorkerType]string{
	WorkerResearch: `You are a research specialist. Use web_search to find information on the web.
Return a comprehensive, factual summary of your findings. Be specific — include names, numbers, and URLs where relevant.`,

	WorkerFetch: `You are a web content specialist. Use fetch_url to retrieve content from specific URLs.
Extract and return the relevant information from the page clearly and concisely.`,

	WorkerFile: `You are a file system specialist. Use read_file, write_file, and list_directory to work with local files.
Report exactly what you found or what you did.`,

	WorkerGeneral: `You are a general-purpose assistant with access to all tools.
Use whatever tools are needed to complete the given goal. Return a clear summary of your findings or actions.`,
}

// worker is a specialist agent with a focused tool set and its own ReAct loop.
type worker struct {
	workerType WorkerType
	system     string
	registry   *tools.Registry
	llm        *llm.Client
	onStep     func(Step)
}

// newWorker creates a specialist worker of the given type.
func newWorker(wtype WorkerType, client *llm.Client, onStep func(Step)) *worker {
	reg := tools.NewRegistry()

	switch wtype {
	case WorkerResearch:
		reg.Register(tools.NewWebSearch())
	case WorkerFetch:
		reg.Register(tools.NewFetchURL())
	case WorkerFile:
		reg.Register(tools.NewReadFile())
		reg.Register(tools.NewWriteFile())
		reg.Register(tools.NewListDirectory())
	case WorkerGeneral:
		reg.Register(tools.NewWebSearch())
		reg.Register(tools.NewFetchURL())
		reg.Register(tools.NewReadFile())
		reg.Register(tools.NewWriteFile())
		reg.Register(tools.NewListDirectory())
	}

	return &worker{
		workerType: wtype,
		system:     workerSystems[wtype],
		registry:   reg,
		llm:        client,
		onStep:     onStep,
	}
}

// Run executes the worker's mini ReAct loop for the given goal.
// It returns a plain-text result summary when done.
func (w *worker) Run(ctx context.Context, goal string) (string, error) {
	messages := []anthropic.MessageParam{
		anthropic.NewUserMessage(anthropic.NewTextBlock(goal)),
	}
	toolParams := w.registry.ToSDKParams()

	for i := 0; i < maxWorkerIterations; i++ {
		resp, err := w.llm.ChatAs(ctx, llm.WorkerModel, w.system, messages, toolParams)
		if err != nil {
			return "", fmt.Errorf("[%s] LLM error: %w", w.workerType, err)
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

		// No tool calls → worker is done; text is the result.
		if len(toolUseBlocks) == 0 {
			return strings.TrimSpace(textOut.String()), nil
		}

		// Execute each tool call and collect results.
		var toolResults []anthropic.ContentBlockParamUnion
		for _, tb := range toolUseBlocks {
			w.emit(Step{
				Type:    StepWorkerAct,
				Content: fmt.Sprintf("[%s] %s(%s)", w.workerType, tb.Name, string(tb.Input)),
			})

			result, execErr := w.registry.Execute(tb.Name, tb.Input)
			if execErr != nil {
				result = fmt.Sprintf("Error: %s", execErr.Error())
			}

			w.emit(Step{
				Type:    StepWorkerObserve,
				Content: fmt.Sprintf("[%s] → %s", w.workerType, truncate(result, 300)),
			})

			toolResults = append(toolResults, anthropic.NewToolResultBlock(tb.ID, result, false))
		}

		messages = append(messages, anthropic.NewUserMessage(toolResults...))
	}

	return "", fmt.Errorf("worker %s exceeded %d iterations without a result", w.workerType, maxWorkerIterations)
}

func (w *worker) emit(s Step) {
	if w.onStep != nil {
		w.onStep(s)
	}
}
