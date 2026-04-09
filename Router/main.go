package main

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"strings"

	peagent "github.com/vijay8059/ai-agent/PlanExecute/agent"
	raagent "github.com/vijay8059/ai-agent/ReactAct/agent"
	maagent "github.com/vijay8059/ai-agent/MultiAgent/agent"

	"github.com/vijay8059/ai-agent/Router/llm"
	"github.com/vijay8059/ai-agent/Router/router"
)

func main() {
	client, err := llm.NewClient()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %s\n", err)
		os.Exit(1)
	}

	r, err := router.New(client)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %s\n", err)
		os.Exit(1)
	}

	// ── Router decision ───────────────────────────────────────────────────────
	r.OnDecision = func(d router.Decision) {
		label := map[router.AgentType]string{
			router.AgentDirect:      "Direct LLM  ",
			router.AgentPlanExecute: "Plan-Execute",
			router.AgentReactAct:    "React-Act   ",
			router.AgentMulti:       "Multi-Agent ",
		}[d.Agent]
		fmt.Printf("\n\033[33m[ROUTER]\033[0m → \033[1m%s\033[0m  (%s)\n", label, d.Reason)
		fmt.Println(strings.Repeat("─", 60))
	}

	// ── Plan-Execute events (from standalone PlanExecute module) ──────────────
	r.OnPlanStep = func(s peagent.Step) {
		switch s.Type {
		case peagent.StepPlan:
			fmt.Printf("\n\033[33m[PLAN]\033[0m\n%s\n", s.Content)
		case peagent.StepExecute:
			fmt.Printf("\n\033[36m[EXECUTE]\033[0m %s\n", s.Content)
		case peagent.StepAct:
			fmt.Printf("\033[34m[ACT    ]\033[0m %s\n", s.Content)
		case peagent.StepObserve:
			fmt.Printf("\033[35m[OBS    ]\033[0m %s\n", s.Content)
		case peagent.StepResult:
			fmt.Printf("\033[32m[RESULT ]\033[0m %s\n", s.Content)
		case peagent.StepSynthesize:
			fmt.Printf("\n\033[33m[SYNTHESIZE]\033[0m %s\n", s.Content)
		case peagent.StepAnswer:
			fmt.Printf("\n\033[32m[ANSWER]\033[0m\n%s\n", s.Content)
		}
	}

	// ── ReactAct events (from standalone ReactAct module) ────────────────────
	r.OnRAStep = func(s raagent.Step) {
		switch s.Type {
		case raagent.StepThink:
			fmt.Printf("\033[33m[THINK  ]\033[0m %s\n", s.Content)
		case raagent.StepAct:
			fmt.Printf("\033[34m[ACT    ]\033[0m %s\n", s.Content)
		case raagent.StepObserve:
			fmt.Printf("\033[35m[OBS    ]\033[0m %s\n", s.Content)
		case raagent.StepAnswer:
			fmt.Printf("\n\033[32m[ANSWER]\033[0m\n%s\n", s.Content)
		}
	}

	// ── Multi-Agent events (from standalone MultiAgent module) ────────────────
	r.OnMAStep = func(s maagent.Step) {
		switch s.Type {
		case maagent.StepDelegate:
			fmt.Printf("\n\033[33m[DELEGATE ]\033[0m %s\n", s.Content)
		case maagent.StepWorkerAct:
			fmt.Printf("\033[34m[WORKER ACT]\033[0m %s\n", s.Content)
		case maagent.StepWorkerObserve:
			fmt.Printf("\033[35m[WORKER OBS]\033[0m %s\n", s.Content)
		case maagent.StepWorkerResult:
			fmt.Printf("\033[36m[WORKER RES]\033[0m %s\n", s.Content)
		case maagent.StepAnswer:
			fmt.Printf("\n\033[32m[ANSWER]\033[0m\n%s\n", s.Content)
		}
	}

	fmt.Println("AI Agent Router — automatically picks the right agent for your query.")
	fmt.Println("  Direct LLM    → simple facts, explanations")
	fmt.Println("  Plan-Execute  → structured tasks with known steps")
	fmt.Println("  React-Act     → flexible single-agent tool use")
	fmt.Println("  Multi-Agent   → adaptive, complex investigations")
	fmt.Println("Commands: 'quit' to exit.")
	fmt.Println(strings.Repeat("═", 60))

	scanner := bufio.NewScanner(os.Stdin)
	for {
		fmt.Print("\n> ")
		if !scanner.Scan() {
			break
		}
		input := strings.TrimSpace(scanner.Text())
		if input == "" {
			continue
		}
		if strings.ToLower(input) == "quit" || strings.ToLower(input) == "exit" {
			fmt.Println("Goodbye.")
			return
		}

		fmt.Println(strings.Repeat("─", 60))

		agentUsed, answer, err := r.Run(context.Background(), input)
		if err != nil {
			fmt.Fprintf(os.Stderr, "\033[31m[ERROR]\033[0m %s\n", err)
		}

		// Direct agent has no OnStep — print its answer here.
		// All other agents emit StepAnswer via their OnStep callbacks.
		if agentUsed == router.AgentDirect && answer != "" {
			fmt.Printf("\n\033[32m[ANSWER]\033[0m\n%s\n", answer)
		}

		fmt.Println(strings.Repeat("═", 60))
	}
}
