package tools

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// ---- ReadFile ---------------------------------------------------------------

// ReadFile reads any text file from disk.
type ReadFile struct{}

func NewReadFile() *ReadFile { return &ReadFile{} }

func (r *ReadFile) Name() string { return "read_file" }

func (r *ReadFile) Description() string {
	return "Read the contents of a file from the local filesystem. Returns the file content as text. Use this to inspect source code, config files, logs, or any text file."
}

func (r *ReadFile) Schema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"path": map[string]any{
				"type":        "string",
				"description": "Absolute or relative path to the file to read.",
			},
		},
		"required": []string{"path"},
	}
}

type readFileInput struct {
	Path string `json:"path"`
}

func (r *ReadFile) Execute(raw json.RawMessage) (string, error) {
	var input readFileInput
	if err := json.Unmarshal(raw, &input); err != nil {
		return "", fmt.Errorf("invalid input: %w", err)
	}
	data, err := os.ReadFile(input.Path)
	if err != nil {
		return "", fmt.Errorf("cannot read %q: %w", input.Path, err)
	}
	return fmt.Sprintf("File: %s\n\n%s", input.Path, string(data)), nil
}

// ---- WriteFile --------------------------------------------------------------

// WriteFile writes text to a file, creating parent directories as needed.
type WriteFile struct{}

func NewWriteFile() *WriteFile { return &WriteFile{} }

func (w *WriteFile) Name() string { return "write_file" }

func (w *WriteFile) Description() string {
	return "Write text content to a file on the local filesystem. Creates the file and any missing parent directories. Overwrites existing content. Use this to save results, generate reports, or write code."
}

func (w *WriteFile) Schema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"path": map[string]any{
				"type":        "string",
				"description": "Absolute or relative path to write to.",
			},
			"content": map[string]any{
				"type":        "string",
				"description": "The text content to write into the file.",
			},
		},
		"required": []string{"path", "content"},
	}
}

type writeFileInput struct {
	Path    string `json:"path"`
	Content string `json:"content"`
}

func (w *WriteFile) Execute(raw json.RawMessage) (string, error) {
	var input writeFileInput
	if err := json.Unmarshal(raw, &input); err != nil {
		return "", fmt.Errorf("invalid input: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(input.Path), 0755); err != nil {
		return "", fmt.Errorf("cannot create directories: %w", err)
	}
	if err := os.WriteFile(input.Path, []byte(input.Content), 0644); err != nil {
		return "", fmt.Errorf("cannot write %q: %w", input.Path, err)
	}
	return fmt.Sprintf("Successfully wrote %d bytes to %s", len(input.Content), input.Path), nil
}

// ---- ListDirectory ----------------------------------------------------------

// ListDirectory lists files and directories at a given path.
type ListDirectory struct{}

func NewListDirectory() *ListDirectory { return &ListDirectory{} }

func (l *ListDirectory) Name() string { return "list_directory" }

func (l *ListDirectory) Description() string {
	return "List files and subdirectories at a given path. Returns names, sizes, and whether each entry is a directory. Use this to explore the filesystem before reading or writing files."
}

func (l *ListDirectory) Schema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"path": map[string]any{
				"type":        "string",
				"description": "Path to the directory to list. Defaults to current directory if empty.",
			},
		},
		"required": []string{"path"},
	}
}

type listDirInput struct {
	Path string `json:"path"`
}

func (l *ListDirectory) Execute(raw json.RawMessage) (string, error) {
	var input listDirInput
	if err := json.Unmarshal(raw, &input); err != nil {
		return "", fmt.Errorf("invalid input: %w", err)
	}
	dir := input.Path
	if dir == "" {
		dir = "."
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		return "", fmt.Errorf("cannot list %q: %w", dir, err)
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Contents of %s:\n\n", dir))
	for _, e := range entries {
		info, _ := e.Info()
		kind := "file"
		size := ""
		if e.IsDir() {
			kind = "dir "
		} else if info != nil {
			size = fmt.Sprintf(" (%d bytes)", info.Size())
		}
		sb.WriteString(fmt.Sprintf("  [%s] %s%s\n", kind, e.Name(), size))
	}
	return sb.String(), nil
}
