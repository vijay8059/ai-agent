package main

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/vijay8059/ai-agent/PlanExecute/agent"
	"github.com/vijay8059/ai-agent/PlanExecute/llm"
)

func main() {
	client, err := llm.NewClient()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %s\n", err)
		os.Exit(1)
	}

	a := agent.New(client)

	// Stream every event to the terminal so you can watch the agent think.
	a.OnStep = func(s agent.Step) {
		switch s.Type {
		case agent.StepPlan:
			fmt.Printf("\n\033[33m[PLAN]\033[0m\n%s\n", s.Content)
		case agent.StepExecute:
			fmt.Printf("\n\033[36m[EXECUTE]\033[0m %s\n", s.Content)
		case agent.StepAct:
			fmt.Printf("\033[34m[ACT    ]\033[0m %s\n", s.Content)
		case agent.StepObserve:
			fmt.Printf("\033[35m[OBS    ]\033[0m %s\n", s.Content)
		case agent.StepResult:
			fmt.Printf("\033[32m[RESULT ]\033[0m %s\n", s.Content)
		case agent.StepSynthesize:
			fmt.Printf("\n\033[33m[SYNTHESIZE]\033[0m %s\n", s.Content)
		case agent.StepAnswer:
			fmt.Printf("\n\033[32m[ANSWER]\033[0m\n%s\n", s.Content)
		}
	}

	fmt.Println("Plan-Execute Agent ready. Type your task and press Enter.")
	fmt.Println("Commands: 'reset' to clear history, 'quit' to exit.")
	fmt.Println(strings.Repeat("─", 60))

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
		switch strings.ToLower(input) {
		case "quit", "exit":
			fmt.Println("Goodbye.")
			return
		case "reset":
			a.ResetHistory()
			fmt.Println("Conversation history cleared.")
			continue
		}

		fmt.Println(strings.Repeat("─", 60))
		_, err := a.Run(context.Background(), input)
		if err != nil {
			fmt.Fprintf(os.Stderr, "\033[31m[ERROR]\033[0m %s\n", err)
		}
		fmt.Println(strings.Repeat("─", 60))
	}
}
