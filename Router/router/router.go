// Package router classifies incoming queries and dispatches them to the right agent.
package router

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/anthropics/anthropic-sdk-go"

	// Standalone PlanExecute module
	peagent "github.com/vijay8059/ai-agent/PlanExecute/agent"
	pellm "github.com/vijay8059/ai-agent/PlanExecute/llm"

	// Standalone MultiAgent module
	maagent "github.com/vijay8059/ai-agent/MultiAgent/agent"
	mallm "github.com/vijay8059/ai-agent/MultiAgent/llm"

	// Standalone ReactAct module
	raagent "github.com/vijay8059/ai-agent/ReactAct/agent"
	rallm "github.com/vijay8059/ai-agent/ReactAct/llm"

	// Router-local packages
	"github.com/vijay8059/ai-agent/Router/agents"
	"github.com/vijay8059/ai-agent/Router/llm"
)

// AgentType identifies which agent will handle the query.
type AgentType string

const (
	AgentDirect      AgentType = "direct"
	AgentPlanExecute AgentType = "plan_execute"
	AgentReactAct    AgentType = "react_act"
	AgentMulti       AgentType = "multi_agent"
)

// Decision is the router's classification result.
type Decision struct {
	Agent  AgentType `json:"agent"`
	Reason string    `json:"reason"`
}

const classifierSystem = `You are a query router. Classify the user's query into exactly one agent type.

Agent types:
- "direct"        → Simple factual questions, definitions, explanations, math, conversions.
                    No real-time data needed. Answered from training knowledge alone.
                    Examples: "What is photosynthesis?", "Convert 100 USD to INR", "Explain recursion"

- "plan_execute"  → Tasks with a known, predictable sequence of steps. Real-time data or tools
                    needed, but the full plan can be decided upfront before execution starts.
                    Examples: "Compare iPhone prices on Amazon and Flipkart",
                              "Search top 5 AI news today and save to a file",
                              "Find Samsung TV prices across 3 Indian e-commerce sites"

- "react_act"     → Single-agent flexible tasks. Tools needed, steps not fully known upfront,
                    but complexity doesn't require specialist workers. Good for debugging,
                    fetching and summarizing a single source, or simple tool-based tasks.
                    Examples: "Summarize the top story on Hacker News today",
                              "Debug why this Go function panics",
                              "Fetch this URL and extract the key points"

- "multi_agent"   → Complex adaptive tasks requiring specialist workers and dynamic decisions.
                    The next step depends on what you discover. Multiple domains involved.
                    Examples: "Research the EV market in India and give a deep analysis",
                              "Find the best budget phone and explain why it's the best",
                              "Investigate and compare two competing technologies"

Return ONLY a JSON object — no prose, no markdown:
{"agent": "<direct|plan_execute|react_act|multi_agent>", "reason": "<one sentence why>"}`

// Router classifies queries and dispatches to the right agent.
type Router struct {
	routerLLM   *llm.Client
	direct      *agents.DirectAgent
	planExecute *peagent.Agent
	reactAct    *raagent.Agent
	multiAgent  *maagent.Orchestrator
	OnDecision  func(Decision)
	OnPlanStep  func(peagent.Step)
	OnRAStep    func(raagent.Step)
	OnMAStep    func(maagent.Step)
}

// New creates a Router wiring in all standalone agent modules.
func New(routerLLM *llm.Client) (*Router, error) {
	peClient, err := pellm.NewClient()
	if err != nil {
		return nil, fmt.Errorf("plan-execute llm: %w", err)
	}
	raClient, err := rallm.NewClient()
	if err != nil {
		return nil, fmt.Errorf("react-act llm: %w", err)
	}
	maClient, err := mallm.NewClient()
	if err != nil {
		return nil, fmt.Errorf("multi-agent llm: %w", err)
	}

	pe := peagent.New(peClient)
	ra := raagent.New(raClient)
	ma := maagent.New(maClient)

	r := &Router{
		routerLLM:   routerLLM,
		direct:      agents.NewDirectAgent(routerLLM),
		planExecute: pe,
		reactAct:    ra,
		multiAgent:  ma,
	}

	pe.OnStep = func(s peagent.Step) {
		if r.OnPlanStep != nil {
			r.OnPlanStep(s)
		}
	}

	ra.OnStep = func(s raagent.Step) {
		if r.OnRAStep != nil {
			r.OnRAStep(s)
		}
	}

	ma.OnStep = func(s maagent.Step) {
		if r.OnMAStep != nil {
			r.OnMAStep(s)
		}
	}

	return r, nil
}

// Run classifies the query and dispatches to the appropriate agent.
func (r *Router) Run(ctx context.Context, query string) (AgentType, string, error) {
	decision, err := r.classify(ctx, query)
	if err != nil {
		decision = Decision{Agent: AgentDirect, Reason: "classifier error — falling back to direct"}
	}

	if r.OnDecision != nil {
		r.OnDecision(decision)
	}

	var answer string
	switch decision.Agent {
	case AgentDirect:
		answer, err = r.direct.Run(ctx, query)
	case AgentPlanExecute:
		answer, err = r.planExecute.Run(ctx, query)
	case AgentReactAct:
		answer, err = r.reactAct.Run(ctx, query)
	case AgentMulti:
		answer, err = r.multiAgent.Run(ctx, query)
	default:
		answer, err = r.direct.Run(ctx, query)
	}

	if err != nil {
		return decision.Agent, "", fmt.Errorf("[%s] %w", decision.Agent, err)
	}
	return decision.Agent, answer, nil
}

// classify asks Claude to classify the query into one of the three agent types.
func (r *Router) classify(ctx context.Context, query string) (Decision, error) {
	messages := []anthropic.MessageParam{
		anthropic.NewUserMessage(anthropic.NewTextBlock("Query: " + query)),
	}
	resp, err := r.routerLLM.Chat(ctx, llm.RouterModel, classifierSystem, messages, nil)
	if err != nil {
		return Decision{}, fmt.Errorf("classifier error: %w", err)
	}

	var raw string
	for _, block := range resp.Content {
		if tb, ok := block.AsAny().(anthropic.TextBlock); ok {
			raw = strings.TrimSpace(tb.Text)
			break
		}
	}

	if strings.HasPrefix(raw, "```") {
		lines := strings.SplitN(raw, "\n", 2)
		if len(lines) == 2 {
			raw = lines[1]
		}
		raw = strings.TrimSuffix(raw, "```")
		raw = strings.TrimSpace(raw)
	}

	var d Decision
	if err := json.Unmarshal([]byte(raw), &d); err != nil {
		return Decision{}, fmt.Errorf("could not parse classifier JSON (%q): %w", raw, err)
	}

	switch AgentType(strings.TrimSpace(string(d.Agent))) {
	case AgentDirect, AgentPlanExecute, AgentReactAct, AgentMulti:
		d.Agent = AgentType(strings.TrimSpace(string(d.Agent)))
	default:
		d.Agent = AgentDirect
	}

	return d, nil
}
