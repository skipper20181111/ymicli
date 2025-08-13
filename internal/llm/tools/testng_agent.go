package tools

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/charmbracelet/crush/internal/permission"
)

type TestNGAgentTool struct {
	permissions permission.Service
}

func NewTestNGAgentTool(permissions permission.Service) BaseTool {
	return &TestNGAgentTool{
		permissions: permissions,
	}
}

func (t *TestNGAgentTool) Name() string {
	return "testng_agent"
}

func (t *TestNGAgentTool) Info() ToolInfo {
	return ToolInfo{
		Name:        "testng_agent",
		Description: "Generate TestNG test cases",
		Parameters: map[string]any{
			"user_context": map[string]any{
				"type":        "string",
				"description": "What test cases to generate",
			},
			"absolute_path": map[string]any{
				"type":        "string",
				"description": "Project directory path",
			},
		},
		Required: []string{"user_context", "absolute_path"},
	}
}

type testNGAgentInput struct {
	UserContext  string `json:"user_context"`
	AbsolutePath string `json:"absolute_path"`
}

type testNGAgentRequest struct {
	UserContext  string `json:"userContext"`
	AbsolutePath string `json:"absolutePath"`
	Stream       bool   `json:"stream"`
	SessionID    string `json:"sessionId"`
}

type testNGAgentStreamChunk struct {
	Status  string `json:"status,omitempty"`
	Content string `json:"content,omitempty"`
	Error   string `json:"error,omitempty"`
}

func (t *TestNGAgentTool) Run(ctx context.Context, params ToolCall) (ToolResponse, error) {
	var args testNGAgentInput

	if err := json.Unmarshal([]byte(params.Input), &args); err != nil {
		return NewTextErrorResponse(fmt.Sprintf("invalid input: %v", err)), err
	}

	// Get session ID from context
	sessionID, _ := ctx.Value(SessionIDContextKey).(string)

	// Request permission to call external service
	allowed := t.permissions.Request(permission.CreatePermissionRequest{
		SessionID:   sessionID,
		ToolCallID:  params.ID,
		ToolName:    t.Name(),
		Description: fmt.Sprintf("Call external TestNG agent at localhost:38888 to generate test cases for %s", args.AbsolutePath),
		Action:      "network",
		Path:        "localhost:38888/code/GenerateTestCase",
		Params:      args,
	})
	if !allowed {
		errMsg := "permission denied to call external TestNG agent"
		return NewTextErrorResponse(errMsg), permission.ErrorPermissionDenied
	}

	// Prepare request
	reqBody := testNGAgentRequest{
		UserContext:  args.UserContext,
		AbsolutePath: args.AbsolutePath,
		Stream:       true,
		SessionID:    sessionID,
	}

	jsonBody, err := json.Marshal(reqBody)
	if err != nil {
		return NewTextErrorResponse(fmt.Sprintf("failed to marshal request: %v", err)), err
	}

	// Make HTTP request
	req, err := http.NewRequestWithContext(ctx, "POST", "http://localhost:38888/code/GenerateTestCase", bytes.NewBuffer(jsonBody))
	if err != nil {
		return NewTextErrorResponse(fmt.Sprintf("failed to create request: %v", err)), err
	}
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{
		Timeout: 30 * time.Minute, // Long timeout for test generation
	}

	resp, err := client.Do(req)
	if err != nil {
		return NewTextErrorResponse(fmt.Sprintf("failed to call TestNG agent: %v", err)), err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		errMsg := fmt.Sprintf("TestNG agent returned status %d: %s", resp.StatusCode, string(body))
		return NewTextErrorResponse(errMsg), fmt.Errorf("TestNG agent returned status %d", resp.StatusCode)
	}

	// Process streaming response
	scanner := bufio.NewScanner(resp.Body)
	var fullOutput strings.Builder
	var lastError string

	for scanner.Scan() {
		line := scanner.Text()

		// Skip empty lines
		if strings.TrimSpace(line) == "" {
			continue
		}

		// Try to parse as JSON
		var chunk testNGAgentStreamChunk
		if err := json.Unmarshal([]byte(line), &chunk); err != nil {
			// If not JSON, treat as plain text
			fullOutput.WriteString(line)
			fullOutput.WriteString("\n")
			continue
		}

		// Handle different chunk types
		if chunk.Error != "" {
			lastError = chunk.Error
			fullOutput.WriteString(fmt.Sprintf("[ERROR] %s\n", chunk.Error))
		} else if chunk.Status != "" {
			fullOutput.WriteString(fmt.Sprintf("[STATUS] %s\n", chunk.Status))
		} else if chunk.Content != "" {
			fullOutput.WriteString(chunk.Content)
		}
	}

	if err := scanner.Err(); err != nil {
		lastError = fmt.Sprintf("error reading stream: %v", err)
	}

	// Prepare final response
	output := fullOutput.String()
	if lastError != "" && output == "" {
		output = lastError
	}

	if lastError != "" {
		return NewTextErrorResponse(output), fmt.Errorf("%s", lastError)
	}

	return NewTextResponse(output), nil
}
