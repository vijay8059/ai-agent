package tools

import (
	"encoding/json"
	"fmt"
	"github.com/anthropics/anthropic-sdk-go"
)

// Tool is the interface every tool must implement.
// Adding a new tool = implement this interface + register it.
type Tool interface {
	// Name must match exactly what Claude will call.
	Name() string
	// Description tells Claude when and how to use this tool.
	Description() string
	// Schema returns a JSON Schema object describing the input parameters.
	Schema() map[string]any
	// Execute runs the tool with the given JSON input and returns a string result.
	Execute(input json.RawMessage) (string, error)
}

// Registry holds all registered tools and converts them to the SDK format.
type Registry struct {
	tools map[string]Tool
}

// NewRegistry creates an empty registry.
func NewRegistry() *Registry {
	return &Registry{tools: make(map[string]Tool)}
}

// Register adds a tool. Panics on duplicate name to catch wiring mistakes early.
func (r *Registry) Register(t Tool) {
	if _, exists := r.tools[t.Name()]; exists {
		panic(fmt.Sprintf("tool %q already registered", t.Name()))
	}
	r.tools[t.Name()] = t
}

// Get returns a tool by name.
func (r *Registry) Get(name string) (Tool, bool) {
	t, ok := r.tools[name]
	return t, ok
}

// ToSDKParams converts all registered tools to the Anthropic SDK's ToolUnionParam slice.
func (r *Registry) ToSDKParams() []anthropic.ToolUnionParam {
	params := make([]anthropic.ToolUnionParam, 0, len(r.tools))
	for _, t := range r.tools {
		schema := t.Schema()

		// Schema() returns a full JSON schema object: {type, properties, required}.
		// ToolInputSchemaParam expects properties and required pulled out separately.
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
		// Return the error as a string so the agent can reason about it.
		return fmt.Sprintf("Error: %s", err.Error()), nil
	}
	return result, nil
}
