package main

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/vijay8059/ai-agent/MultiAgent/agent"
	"github.com/vijay8059/ai-agent/MultiAgent/llm"
)

func main() {
	client, err := llm.NewClient()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %s\n", err)
		os.Exit(1)
	}

	orch := agent.New(client)

	// Stream every event to the terminal.
	orch.OnStep = func(s agent.Step) {
		switch s.Type {
		case agent.StepDelegate:
			fmt.Printf("\n\033[33m[DELEGATE ]\033[0m %s\n", s.Content)
		case agent.StepWorkerAct:
			fmt.Printf("\033[34m[WORKER ACT]\033[0m %s\n", s.Content)
		case agent.StepWorkerObserve:
			fmt.Printf("\033[35m[WORKER OBS]\033[0m %s\n", s.Content)
		case agent.StepWorkerResult:
			fmt.Printf("\033[36m[WORKER RESULT]\033[0m %s\n", s.Content)
		case agent.StepAnswer:
			fmt.Printf("\n\033[32m[ANSWER]\033[0m\n%s\n", s.Content)
		}
	}

	fmt.Println("Multi-Agent ready. The orchestrator will dynamically delegate to specialist workers.")
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
			orch.ResetHistory()
			fmt.Println("Conversation history cleared.")
			continue
		}

		fmt.Println(strings.Repeat("─", 60))
		_, err := orch.Run(context.Background(), input)
		if err != nil {
			fmt.Fprintf(os.Stderr, "\033[31m[ERROR]\033[0m %s\n", err)
		}
		fmt.Println(strings.Repeat("─", 60))
	}
}
