package provider

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/charmbracelet/catwalk/pkg/catwalk"
	"github.com/charmbracelet/crush/internal/config"
	"github.com/charmbracelet/crush/internal/llm/tools"
	"github.com/charmbracelet/crush/internal/log"
	"github.com/charmbracelet/crush/internal/message"
)

type httpStreamClient struct {
	providerOptions providerClientOptions
	httpClient      *http.Client
}

type HTTPStreamClient ProviderClient

func newHTTPStreamClient(opts providerClientOptions) HTTPStreamClient {
	return &httpStreamClient{
		providerOptions: opts,
		httpClient:      createHTTPClient(opts),
	}
}

func createHTTPClient(opts providerClientOptions) *http.Client {
	var httpClient *http.Client
	if config.Get().Options.Debug {
		httpClient = log.NewHTTPClient()
	} else {
		httpClient = &http.Client{
			Timeout: 5 * time.Minute,
		}
	}
	return httpClient
}

// logHTTPRequest logs the HTTP request details to httplog.log, excluding system prompts and tool definitions
func logHTTPRequest(requestBody map[string]interface{}, url string) {
	wd, err := os.Getwd()
	if err != nil {
		return
	}

	logFile := filepath.Join(wd, "httplog.log")
	file, err := os.OpenFile(logFile, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return
	}
	defer file.Close()

	timestamp := time.Now().Format("2006-01-02 15:04:05")

	// Create a copy of the request body for logging
	logRequestBody := make(map[string]interface{})
	for k, v := range requestBody {
		logRequestBody[k] = v
	}

	// Filter out system prompt and tool definitions
	if messages, ok := logRequestBody["messages"].([]map[string]interface{}); ok {
		// Filter out the first system message if it exists
		filteredMessages := make([]map[string]interface{}, 0)
		for i, msg := range messages {
			if i == 0 && msg["role"] == "system" {
				// Skip the first system message
				continue
			}
			filteredMessages = append(filteredMessages, msg)
		}
		logRequestBody["messages"] = filteredMessages
	}

	// Remove tools definition
	delete(logRequestBody, "tools")

	// Pretty print the filtered JSON request
	jsonData, err := json.MarshalIndent(logRequestBody, "", "  ")
	if err != nil {
		return
	}

	logEntry := fmt.Sprintf("\n=== HTTP Request - %s ===\n", timestamp)
	logEntry += fmt.Sprintf("URL: %s\n", url)
	logEntry += fmt.Sprintf("Request Body:\n%s\n", string(jsonData))
	logEntry += "=== End Request ===\n\n"

	file.WriteString(logEntry)
}

// HTTPStream API response structures
type StreamResponse struct {
	ID                string   `json:"id"`
	Object            string   `json:"object"`
	Created           int64    `json:"created"`
	Model             string   `json:"model"`
	SystemFingerprint *string  `json:"system_fingerprint"`
	Choices           []Choice `json:"choices"`
	Usage             *Usage   `json:"usage"`
}

type Choice struct {
	Delta        Delta   `json:"delta"`
	LogProbs     *string `json:"logprobs"`
	FinishReason *string `json:"finish_reason"`
	Index        int     `json:"index"`
}

type Delta struct {
	Content     *string                    `json:"content,omitempty"`
	Role        *string                    `json:"role,omitempty"`
	ToolCalls   []StreamToolCall           `json:"tool_calls,omitempty"`
	ExtraFields map[string]json.RawMessage `json:"-"`
}

type StreamToolCall struct {
	Index    int                `json:"index"`
	ID       *string            `json:"id"`
	Type     *string            `json:"type"`
	Function StreamToolFunction `json:"function"`
}

type StreamToolFunction struct {
	Name      *string `json:"name,omitempty"`
	Arguments string  `json:"arguments,omitempty"`
}

type Usage struct {
	PromptTokens            int64                    `json:"prompt_tokens"`
	CompletionTokens        int64                    `json:"completion_tokens"`
	TotalTokens             int64                    `json:"total_tokens"`
	PromptTokensDetails     *PromptTokensDetails     `json:"prompt_tokens_details"`
	CompletionTokensDetails *CompletionTokensDetails `json:"completion_tokens_details"`
	InputTokens             int64                    `json:"input_tokens"`
	OutputTokens            int64                    `json:"output_tokens"`
	InputTokensDetails      *InputTokensDetails      `json:"input_tokens_details"`
}

type PromptTokensDetails struct {
	CachedTokens int64 `json:"cached_tokens"`
	TextTokens   int64 `json:"text_tokens"`
	AudioTokens  int64 `json:"audio_tokens"`
	ImageTokens  int64 `json:"image_tokens"`
}

type CompletionTokensDetails struct {
	TextTokens               int64 `json:"text_tokens"`
	AudioTokens              int64 `json:"audio_tokens"`
	ReasoningTokens          int64 `json:"reasoning_tokens"`
	AcceptedPredictionTokens int64 `json:"accepted_prediction_tokens"`
	RejectedPredictionTokens int64 `json:"rejected_prediction_tokens"`
}

type InputTokensDetails struct {
	// Fields to be determined based on future observations
}

// Custom unmarshaling for Delta to handle extra fields like reasoning
func (d *Delta) UnmarshalJSON(data []byte) error {
	type Alias Delta
	aux := &struct {
		*Alias
	}{
		Alias: (*Alias)(d),
	}

	if err := json.Unmarshal(data, &aux); err != nil {
		return err
	}

	// Parse extra fields
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}

	d.ExtraFields = make(map[string]json.RawMessage)
	for key, value := range raw {
		switch key {
		case "content", "role", "tool_calls":
			// Skip known fields
		default:
			d.ExtraFields[key] = value
		}
	}

	return nil
}

// Tool call state for managing streaming tool calls
type toolCallState struct {
	ID        string
	Name      string
	Arguments strings.Builder
	Type      string
	Finished  bool
}

// Stream parser for managing the streaming state
type streamParser struct {
	contentBuffer   strings.Builder
	thinkingBuffer  strings.Builder
	signatureBuffer strings.Builder
	toolCalls       map[int]*toolCallState    // Index -> tool call state
	toolMap         map[string]*toolCallState // ID -> tool call state
	usage           TokenUsage
	finishReason    string
	currentContent  string
	toolCallStarted map[string]bool // Track which tool calls have started
}

func newStreamParser() *streamParser {
	return &streamParser{
		toolCalls:       make(map[int]*toolCallState),
		toolMap:         make(map[string]*toolCallState),
		toolCallStarted: make(map[string]bool),
	}
}

func (h *httpStreamClient) convertMessages(messages []message.Message) []map[string]interface{} {
	openaiMessages := make([]map[string]interface{}, 0)

	// Add system message first
	systemMessage := h.providerOptions.systemMessage
	if h.providerOptions.systemPromptPrefix != "" {
		systemMessage = h.providerOptions.systemPromptPrefix + "\n" + systemMessage
	}

	if systemMessage != "" {
		openaiMessages = append(openaiMessages, map[string]interface{}{
			"role":    "system",
			"content": systemMessage,
		})
	}

	for _, msg := range messages {
		switch msg.Role {
		case message.User:
			content := make([]map[string]interface{}, 0)

			// Add text content
			if msg.Content().String() != "" {
				content = append(content, map[string]interface{}{
					"type": "text",
					"text": msg.Content().String(),
				})
			}

			// Add binary content (images)
			for _, binaryContent := range msg.BinaryContent() {
				content = append(content, map[string]interface{}{
					"type": "image_url",
					"image_url": map[string]interface{}{
						"url": binaryContent.String(catwalk.InferenceProviderOpenAI),
					},
				})
			}

			if len(content) == 1 && content[0]["type"] == "text" {
				// Simple text message
				openaiMessages = append(openaiMessages, map[string]interface{}{
					"role":    "user",
					"content": content[0]["text"],
				})
			} else {
				// Complex message with multiple content parts
				openaiMessages = append(openaiMessages, map[string]interface{}{
					"role":    "user",
					"content": content,
				})
			}

		case message.Assistant:
			assistantMsg := map[string]interface{}{
				"role": "assistant",
			}

			if msg.Content().String() != "" {
				assistantMsg["content"] = msg.Content().Text
			}

			// Add tool calls (include all tool calls that have results, regardless of Finished status)
			if len(msg.ToolCalls()) > 0 {
				// Get all tool call IDs that have results in subsequent messages
				validToolCallIDs := make(map[string]bool)

				// Look ahead to find tool results in the remaining messages
				for i := range messages {
					if messages[i].Role == message.Tool {
						for _, result := range messages[i].ToolResults() {
							validToolCallIDs[result.ToolCallID] = true
						}
					}
				}

				// Include tool calls that either finished successfully or have results
				validCalls := make([]message.ToolCall, 0, len(msg.ToolCalls()))
				for _, call := range msg.ToolCalls() {
					if call.Finished || validToolCallIDs[call.ID] {
						validCalls = append(validCalls, call)
					}
				}

				if len(validCalls) > 0 {
					toolCalls := make([]map[string]interface{}, len(validCalls))
					for i, call := range validCalls {
						toolCalls[i] = map[string]interface{}{
							"id":   call.ID,
							"type": "function",
							"function": map[string]interface{}{
								"name":      call.Name,
								"arguments": call.Input,
							},
						}
					}
					assistantMsg["tool_calls"] = toolCalls
				}
			}

			// Skip empty assistant messages
			if _, hasContent := assistantMsg["content"]; !hasContent {
				if _, hasToolCalls := assistantMsg["tool_calls"]; !hasToolCalls {
					continue
				}
			}

			openaiMessages = append(openaiMessages, assistantMsg)

		case message.Tool:
			for _, result := range msg.ToolResults() {
				openaiMessages = append(openaiMessages, map[string]interface{}{
					"role":         "tool",
					"content":      result.Content,
					"tool_call_id": result.ToolCallID,
				})
			}
		}
	}

	return openaiMessages
}

func (h *httpStreamClient) convertTools(tools []tools.BaseTool) []map[string]interface{} {
	if len(tools) == 0 {
		return nil
	}

	openaiTools := make([]map[string]interface{}, len(tools))
	for i, tool := range tools {
		info := tool.Info()
		openaiTools[i] = map[string]interface{}{
			"type": "function",
			"function": map[string]interface{}{
				"name":        info.Name,
				"description": info.Description,
				"parameters": map[string]interface{}{
					"type":       "object",
					"properties": info.Parameters,
					"required":   info.Required,
				},
			},
		}
	}

	return openaiTools
}

func (h *httpStreamClient) prepareRequest(messages []message.Message, tools []tools.BaseTool) map[string]interface{} {
	model := h.providerOptions.model(h.providerOptions.modelType)
	cfg := config.Get()

	modelConfig := cfg.Models[config.SelectedModelTypeLarge]
	if h.providerOptions.modelType == config.SelectedModelTypeSmall {
		modelConfig = cfg.Models[config.SelectedModelTypeSmall]
	}

	request := map[string]interface{}{
		"model":    model.ID,
		"messages": h.convertMessages(messages),
		"stream":   true,
	}

	// Add tools if available
	if convertedTools := h.convertTools(tools); len(convertedTools) > 0 {
		request["tools"] = convertedTools
		request["tool_choice"] = "auto"
	}

	// Set max tokens
	maxTokens := model.DefaultMaxTokens
	if modelConfig.MaxTokens > 0 {
		maxTokens = modelConfig.MaxTokens
	}
	if h.providerOptions.maxTokens > 0 {
		maxTokens = h.providerOptions.maxTokens
	}

	if model.CanReason {
		request["max_completion_tokens"] = maxTokens
		if reasoningEffort := modelConfig.ReasoningEffort; reasoningEffort != "" {
			request["reasoning_effort"] = reasoningEffort
		}
	} else {
		request["max_tokens"] = maxTokens
	}

	// Add extra body parameters
	for key, value := range h.providerOptions.extraBody {
		request[key] = value
	}

	return request
}

func (h *httpStreamClient) createHTTPRequest(ctx context.Context, requestBody map[string]interface{}) (*http.Request, error) {
	jsonData, err := json.Marshal(requestBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	baseURL := h.providerOptions.baseURL
	if baseURL == "" {
		return nil, fmt.Errorf("base URL is required for HTTP stream client")
	}

	resolvedBaseURL, err := config.Get().Resolve(baseURL)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve base URL: %w", err)
	}

	// Ensure the URL ends with the chat completions endpoint
	if !strings.HasSuffix(resolvedBaseURL, "/chat/completions") {
		if !strings.HasSuffix(resolvedBaseURL, "/") {
			resolvedBaseURL += "/"
		}
		resolvedBaseURL += "chat/completions"
	}

	// Log the HTTP request
	logHTTPRequest(requestBody, resolvedBaseURL)

	req, err := http.NewRequestWithContext(ctx, "POST", resolvedBaseURL, bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Set headers
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "text/event-stream")

	if h.providerOptions.apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+h.providerOptions.apiKey)
	}

	// Add extra headers
	for key, value := range h.providerOptions.extraHeaders {
		req.Header.Set(key, value)
	}

	return req, nil
}

func (h *httpStreamClient) send(ctx context.Context, messages []message.Message, tools []tools.BaseTool) (*ProviderResponse, error) {
	// For non-streaming requests, we'll collect all streaming events and return the final result
	eventChan := h.stream(ctx, messages, tools)

	var finalResponse *ProviderResponse
	for event := range eventChan {
		if event.Type == EventError {
			return nil, event.Error
		}
		if event.Type == EventComplete && event.Response != nil {
			finalResponse = event.Response
		}
	}

	if finalResponse == nil {
		return nil, fmt.Errorf("no response received from HTTP stream")
	}

	return finalResponse, nil
}

func (h *httpStreamClient) stream(ctx context.Context, messages []message.Message, tools []tools.BaseTool) <-chan ProviderEvent {
	eventChan := make(chan ProviderEvent)

	go func() {
		defer close(eventChan)

		attempts := 0
		for {
			attempts++

			requestBody := h.prepareRequest(messages, tools)
			req, err := h.createHTTPRequest(ctx, requestBody)
			if err != nil {
				sendEvent(eventChan, ProviderEvent{Type: EventError, Error: err})
				return
			}

			resp, err := h.httpClient.Do(req)
			if err != nil {
				retry, after, retryErr := h.shouldRetry(attempts, err)
				if retryErr != nil {
					sendEvent(eventChan, ProviderEvent{Type: EventError, Error: retryErr})
					return
				}
				if retry {
					slog.Warn("Retrying HTTP request", "attempt", attempts, "max_retries", maxRetries, "error", err)
					select {
					case <-ctx.Done():
						sendEvent(eventChan, ProviderEvent{Type: EventError, Error: ctx.Err()})
						return
					case <-time.After(time.Duration(after) * time.Millisecond):
						continue
					}
				}
				sendEvent(eventChan, ProviderEvent{Type: EventError, Error: retryErr})
				return
			}
			defer resp.Body.Close()

			if resp.StatusCode != http.StatusOK {
				body, _ := io.ReadAll(resp.Body)
				err := fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(body))

				retry, after, retryErr := h.shouldRetryHTTPStatus(attempts, resp.StatusCode, err)
				if retryErr != nil {
					sendEvent(eventChan, ProviderEvent{Type: EventError, Error: retryErr})
					return
				}
				if retry {
					slog.Warn("Retrying HTTP request due to status code", "attempt", attempts, "status_code", resp.StatusCode, "max_retries", maxRetries)
					select {
					case <-ctx.Done():
						sendEvent(eventChan, ProviderEvent{Type: EventError, Error: ctx.Err()})
						return
					case <-time.After(time.Duration(after) * time.Millisecond):
						continue
					}
				}
				sendEvent(eventChan, ProviderEvent{Type: EventError, Error: retryErr})
				return
			}

			// Process the streaming response
			if err := h.processStreamingResponse(ctx, resp.Body, eventChan); err != nil {
				if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
					return
				}
				retry, after, retryErr := h.shouldRetry(attempts, err)
				if retryErr != nil {
					sendEvent(eventChan, ProviderEvent{Type: EventError, Error: retryErr})
					return
				}
				if retry {
					slog.Warn("Retrying due to streaming error", "attempt", attempts, "max_retries", maxRetries, "error", err)
					select {
					case <-ctx.Done():
						return
					case <-time.After(time.Duration(after) * time.Millisecond):
						continue
					}
				}
				sendEvent(eventChan, ProviderEvent{Type: EventError, Error: retryErr})
				return
			}

			// Successfully processed the stream
			return
		}
	}()

	return eventChan
}

func (h *httpStreamClient) processStreamingResponse(ctx context.Context, body io.Reader, eventChan chan<- ProviderEvent) error {
	parser := newStreamParser()
	scanner := bufio.NewScanner(body)

	for scanner.Scan() {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}

		if !strings.HasPrefix(line, "data: ") {
			continue
		}

		data := strings.TrimPrefix(line, "data: ")
		if data == "[DONE]" {
			// Stream completed
			response := parser.buildFinalResponse()
			completeEvent := ProviderEvent{
				Type:     EventComplete,
				Response: response,
			}
			// Log complete tool calls with full arguments and send event
			sendEvent(eventChan, completeEvent)
			return nil
		}

		// Parse the JSON chunk
		var streamResp StreamResponse
		if err := json.Unmarshal([]byte(data), &streamResp); err != nil {
			slog.Warn("Failed to parse streaming response chunk", "error", err, "data", data)
			continue
		}

		// Process the parsed chunk
		events := parser.processChunk(streamResp)
		for _, event := range events {
			// Log and send event
			sendEvent(eventChan, event)
		}
	}

	if err := scanner.Err(); err != nil {
		return fmt.Errorf("error reading streaming response: %w", err)
	}

	return nil
}

func (p *streamParser) processChunk(resp StreamResponse) []ProviderEvent {
	var events []ProviderEvent

	// Process usage information
	if resp.Usage != nil {
		// Calculate output tokens excluding reasoning and audio tokens
		outputTokens := resp.Usage.CompletionTokens
		reasoningTokens := int64(0)
		audioOutputTokens := int64(0)
		//acceptedPredictionTokens := int64(0)
		//rejectedPredictionTokens := int64(0)

		if resp.Usage.CompletionTokensDetails != nil {
			reasoningTokens = resp.Usage.CompletionTokensDetails.ReasoningTokens
			audioOutputTokens = resp.Usage.CompletionTokensDetails.AudioTokens
			//acceptedPredictionTokens = resp.Usage.CompletionTokensDetails.AcceptedPredictionTokens
			//rejectedPredictionTokens = resp.Usage.CompletionTokensDetails.RejectedPredictionTokens
			// Only count text tokens for output, exclude reasoning and audio
			if resp.Usage.CompletionTokensDetails.TextTokens > 0 {
				outputTokens = resp.Usage.CompletionTokensDetails.TextTokens
			} else {
				// Fallback: subtract reasoning and audio from total completion tokens
				outputTokens = resp.Usage.CompletionTokens - reasoningTokens - audioOutputTokens
			}
		}

		p.usage = TokenUsage{
			InputTokens:  resp.Usage.PromptTokens,
			OutputTokens: outputTokens,
			//ReasoningTokens: reasoningTokens,
			//AudioOutputTokens: audioOutputTokens,
			//AcceptedPredictionTokens: acceptedPredictionTokens,
			//RejectedPredictionTokens: rejectedPredictionTokens,
		}
		if resp.Usage.PromptTokensDetails != nil {
			p.usage.CacheReadTokens = resp.Usage.PromptTokensDetails.CachedTokens
			//p.usage.AudioInputTokens = resp.Usage.PromptTokensDetails.AudioTokens
		}
	}

	// Process choices
	for _, choice := range resp.Choices {
		delta := choice.Delta

		// Handle role initialization
		if delta.Role != nil && *delta.Role == "assistant" && (delta.Content == nil || *delta.Content == "") {
			events = append(events, ProviderEvent{Type: EventContentStart})
			continue
		}

		// Handle text content
		if delta.Content != nil && *delta.Content != "" {
			p.contentBuffer.WriteString(*delta.Content)
			p.currentContent += *delta.Content
			events = append(events, ProviderEvent{
				Type:    EventContentDelta,
				Content: *delta.Content,
			})
		}

		// Handle reasoning content (for o1 models)
		if reasoning, exists := delta.ExtraFields["reasoning"]; exists {
			reasoningStr := ""
			if err := json.Unmarshal(reasoning, &reasoningStr); err == nil && reasoningStr != "" {
				p.thinkingBuffer.WriteString(reasoningStr)
				events = append(events, ProviderEvent{
					Type:     EventThinkingDelta,
					Thinking: reasoningStr,
				})
			}
		}

		// Handle tool calls
		for _, toolCall := range delta.ToolCalls {
			toolEvents := p.processToolCall(toolCall)
			events = append(events, toolEvents...)
		}

		// Handle finish reason
		if choice.FinishReason != nil && *choice.FinishReason != "" {
			p.finishReason = *choice.FinishReason

			// Mark all tool calls as finished (but don't emit stop events)
			for _, state := range p.toolCalls {
				state.Finished = true
			}
		}
	}

	return events
}

func (p *streamParser) processToolCall(tc StreamToolCall) []ProviderEvent {
	var events []ProviderEvent

	existingToolCall, existsByIndex := p.toolCalls[tc.Index]
	newToolCall := false

	if existsByIndex {
		// Tool call exists by index
		if tc.ID != nil && *tc.ID != "" && *tc.ID != existingToolCall.ID {
			// ID changed, try to find by ID in toolMap
			if foundByID, existsByID := p.toolMap[*tc.ID]; existsByID {
				// Found existing tool call by ID, accumulate to it
				existingToolCall = foundByID
				if tc.Function.Arguments != "" {
					existingToolCall.Arguments.WriteString(tc.Function.Arguments)
				}
				p.toolCalls[tc.Index] = existingToolCall
				p.toolMap[existingToolCall.ID] = existingToolCall
			} else {
				// New tool call with different ID
				newToolCall = true
			}
		} else {
			// Same index, accumulate arguments
			if tc.Function.Arguments != "" {
				existingToolCall.Arguments.WriteString(tc.Function.Arguments)
			}
			p.toolCalls[tc.Index] = existingToolCall
			p.toolMap[existingToolCall.ID] = existingToolCall
		}
	} else {
		// New tool call
		newToolCall = true
	}

	if newToolCall {
		// Create new tool call
		if tc.ID != nil && *tc.ID != "" && tc.Function.Name != nil && *tc.Function.Name != "" {
			state := &toolCallState{
				ID:   *tc.ID,
				Name: *tc.Function.Name,
				Type: "function",
			}
			if tc.Function.Arguments != "" {
				state.Arguments.WriteString(tc.Function.Arguments)
			}

			p.toolCalls[tc.Index] = state
			p.toolMap[*tc.ID] = state

			// Only emit start event once per tool call
			if !p.toolCallStarted[*tc.ID] {
				p.toolCallStarted[*tc.ID] = true
				events = append(events, ProviderEvent{
					Type: EventToolUseStart,
					ToolCall: &message.ToolCall{
						ID:       *tc.ID,
						Name:     *tc.Function.Name,
						Input:    "",
						Type:     "function",
						Finished: false,
					},
				})
			}
		}
	}

	// Arguments are accumulated silently - no delta events needed
	// The existingToolCall case is handled above in the accumulation logic

	return events
}

func (p *streamParser) buildFinalResponse() *ProviderResponse {
	// Build tool calls
	var toolCalls []message.ToolCall
	for _, state := range p.toolCalls {
		toolCalls = append(toolCalls, message.ToolCall{
			ID:       state.ID,
			Name:     state.Name,
			Input:    state.Arguments.String(),
			Type:     state.Type,
			Finished: true,
		})
	}

	// Determine finish reason
	finishReason := message.FinishReasonEndTurn
	switch p.finishReason {
	case "stop":
		finishReason = message.FinishReasonEndTurn
	case "length":
		finishReason = message.FinishReasonMaxTokens
	case "tool_calls":
		finishReason = message.FinishReasonToolUse
	default:
		if len(toolCalls) > 0 {
			finishReason = message.FinishReasonToolUse
		} else {
			finishReason = message.FinishReasonUnknown
		}
	}

	return &ProviderResponse{
		Content:      p.currentContent,
		ToolCalls:    toolCalls,
		Usage:        p.usage,
		FinishReason: finishReason,
	}
}

func (h *httpStreamClient) shouldRetry(attempts int, err error) (bool, int64, error) {
	if attempts > maxRetries {
		return false, 0, fmt.Errorf("maximum retry attempts reached: %d retries", maxRetries)
	}

	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return false, 0, err
	}

	// Exponential backoff with jitter
	backoffMs := 2000 * (1 << (attempts - 1))
	jitterMs := int(float64(backoffMs) * 0.2)
	retryMs := backoffMs + jitterMs

	return true, int64(retryMs), nil
}

func (h *httpStreamClient) shouldRetryHTTPStatus(attempts int, statusCode int, err error) (bool, int64, error) {
	if attempts > maxRetries {
		return false, 0, fmt.Errorf("maximum retry attempts reached for HTTP status %d: %d retries", statusCode, maxRetries)
	}

	// Retry on rate limiting (429) and server errors (5xx)
	if statusCode == 429 || (statusCode >= 500 && statusCode < 600) {
		backoffMs := 2000 * (1 << (attempts - 1))
		jitterMs := int(float64(backoffMs) * 0.2)
		retryMs := backoffMs + jitterMs
		return true, int64(retryMs), nil
	}

	// Don't retry on client errors (4xx except 429)
	return false, 0, err
}

func (h *httpStreamClient) Model() catwalk.Model {
	return h.providerOptions.model(h.providerOptions.modelType)
}
