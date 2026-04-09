module github.com/vijay8059/ai-agent/Router

go 1.23.8

require (
	github.com/anthropics/anthropic-sdk-go v1.30.0
	github.com/vijay8059/ai-agent/MultiAgent v0.0.0
	github.com/vijay8059/ai-agent/PlanExecute v0.0.0
)

require (
	github.com/tidwall/gjson v1.18.0 // indirect
	github.com/tidwall/match v1.1.1 // indirect
	github.com/tidwall/pretty v1.2.1 // indirect
	github.com/tidwall/sjson v1.2.5 // indirect
	golang.org/x/sync v0.16.0 // indirect
)

replace (
	github.com/vijay8059/ai-agent/MultiAgent => ../MultiAgent
	github.com/vijay8059/ai-agent/PlanExecute => ../PlanExecute
)
