package tools

import (
	"encoding/json"
	"fmt"

	"github.com/anthropics/anthropic-sdk-go"
)

// Tool is the interface every tool must implement.
type Tool interface {
	Name() string
	Description() string
	Schema() map[string]any
	Execute(input json.RawMessage) (string, error)
}

// Registry holds all registered tools and converts them to the SDK format.
type Registry struct {
	tools map[string]Tool
}

func NewRegistry() *Registry {
	return &Registry{tools: make(map[string]Tool)}
}

func (r *Registry) Register(t Tool) {
	if _, exists := r.tools[t.Name()]; exists {
		panic(fmt.Sprintf("tool %q already registered", t.Name()))
	}
	r.tools[t.Name()] = t
}

func (r *Registry) Get(name string) (Tool, bool) {
	t, ok := r.tools[name]
	return t, ok
}

// ToSDKParams converts all registered tools to the Anthropic SDK's ToolUnionParam slice.
func (r *Registry) ToSDKParams() []anthropic.ToolUnionParam {
	params := make([]anthropic.ToolUnionParam, 0, len(r.tools))
	for _, t := range r.tools {
		schema := t.Schema()

		var properties any
		var required []string

		if props, ok := schema["properties"]; ok {
			properties = props
		}
		if req, ok := schema["required"].([]string); ok {
			required = req
		}

		params = append(params, anthropic.ToolUnionParam{
			OfTool: &anthropic.ToolParam{
				Name:        t.Name(),
				Description: anthropic.String(t.Description()),
				InputSchema: anthropic.ToolInputSchemaParam{
					Properties: properties,
					Required:   required,
				},
			},
		})
	}
	return params
}

// Execute dispatches a tool call by name and returns the result string.
func (r *Registry) Execute(name string, rawInput json.RawMessage) (string, error) {
	t, ok := r.Get(name)
	if !ok {
		return "", fmt.Errorf("unknown tool %q", name)
	}
	result, err := t.Execute(rawInput)
	if err != nil {
		return fmt.Sprintf("Error: %s", err.Error()), nil
	}
	return result, nil
}
