# AI Agent — Build Guide

## What is an AI Agent?

An **AI Agent** is a program that:
1. Perceives its environment (via input, tools, or APIs)
2. Reasons about what to do (using an LLM like Claude)
3. Takes actions (calling tools, writing files, browsing the web, etc.)
4. Iterates in a loop until the goal is achieved

Unlike a simple chatbot that just responds once, an agent **thinks → acts → observes → repeats** until it completes a task.

```
User Goal
   │
   ▼
┌─────────────────────────────────────┐
│              Agent Loop             │
│                                     │
│  Think (LLM) ──► Act (Tool Call)   │
│       ▲                 │           │
│       └──── Observe ◄───┘           │
└─────────────────────────────────────┘
   │
   ▼
Result
```

---

## Core Concepts

| Concept        | Description                                                  |
|----------------|--------------------------------------------------------------|
| **LLM**        | The brain — Claude, GPT-4, Gemini, etc.                     |
| **Tools**      | Functions the agent can call (search, run code, read files) |
| **Memory**     | Short-term (context window) + Long-term (vector DB / files) |
| **Planner**    | Breaks a goal into sub-tasks                                |
| **Executor**   | Runs each step and feeds results back to the LLM            |

---

## Agent Patterns

### 1. ReAct Pattern (Reason + Act)
The most common pattern. The LLM reasons step by step and decides which tool to call.
```
Thought: I need to search for X
Action: search("X")
Observation: [results]
Thought: Now I can answer
Answer: ...
```

### 2. Plan-and-Execute
LLM creates a full plan first, then executes each step. Better for long tasks.
```
Plan: [step1, step2, step3]
Execute step1 → result
Execute step2 → result
Execute step3 → final answer
```

### 3. Multi-Agent (Orchestrator + Workers)
One orchestrator agent delegates to specialist worker agents.
```
Orchestrator
  ├── Research Agent
  ├── Code Agent
  └── Writer Agent
```

### 4. Reflection / Self-Critique
Agent critiques its own output and refines it.
```
Draft answer → Critique → Improved answer
```

---

## Best Language to Build Agents

| Language       | Pros                                              | Best For                         |
|----------------|---------------------------------------------------|----------------------------------|
| **Python**     | Most ecosystem (LangChain, LlamaIndex, CrewAI)    | Prototyping, data-heavy agents   |
| **Go**         | Fast, low memory, great for production services   | Production APIs, high-throughput |
| **TypeScript** | Best for web integrations, Vercel AI SDK          | Web apps, fullstack agents       |
| **Rust**       | Maximum performance, memory safety                | Embedded, ultra-low latency      |

**Recommendation:** 
- Start in **Python** to prototype fast
- Move to **Go** for production (your project is already in a Go path!)

---

## Project Structure (Go)

```
ai-agent/
├── main.go              # Entry point
├── agent/
│   ├── agent.go         # Core agent loop
│   ├── memory.go        # Conversation history
│   └── planner.go       # Optional: plan-and-execute
├── tools/
│   ├── tools.go         # Tool registry
│   ├── search.go        # Web search tool
│   ├── calculator.go    # Calculator tool
│   └── file.go          # File read/write tool
├── llm/
│   └── claude.go        # Claude API client
├── go.mod
└── go.sum
```

---

## Step-by-Step Build Plan

### Phase 1 — Foundations
- [ ] Set up Go module
- [ ] Create Claude API client (`llm/claude.go`)
- [ ] Build basic chat loop (single turn)
- [ ] Add conversation memory (multi-turn)

### Phase 2 — Tools
- [ ] Define tool interface
- [ ] Build tool registry
- [ ] Implement calculator tool (simple test)
- [ ] Implement file read/write tool
- [ ] Wire tools into agent loop (ReAct pattern)

### Phase 3 — Agent Loop
- [ ] Parse LLM tool-call responses
- [ ] Execute tool, feed result back to LLM
- [ ] Loop until LLM returns a final answer (no tool call)
- [ ] Add max-iteration safety limit

### Phase 4 — Memory
- [ ] Short-term: sliding window of last N messages
- [ ] Long-term: persist conversation to file (JSON)

### Phase 5 — Polish
- [ ] Structured logging
- [ ] CLI interface
- [ ] Config via environment variables

---

## Quick Start (after build)

```bash
export ANTHROPIC_API_KEY=your_key_here
go run main.go

> What is 42 * 7?
Agent thinking...
[Tool: calculator] 42 * 7 = 294
Answer: 42 multiplied by 7 is 294.
```

---

## Dependencies (Go)

```
github.com/anthropics/anthropic-sdk-go   # Official Claude SDK
```

---

## What We Are Building

A **ReAct-pattern AI agent** in Go that:
- Talks to Claude via the Anthropic SDK
- Has a set of pluggable tools (calculator, file I/O, web search)
- Loops autonomously until it solves the user's task
- Keeps conversation memory across turns
