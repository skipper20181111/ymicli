// Package transformer 提供 Claude API 到 OpenAI 格式的完整转换功能
//
// 包含：完整的请求/响应转换、工具调用支持、流式SSE解析、HTTP服务器
//
// 使用示例：
//
//	// 方式1: 直接启动服务器（最简单）
//	transformer.DefaultAPIKey = "your-api-key"
//	transformer.StartServer() // 启动在 :9999
//
//	// 方式2: 作为客户端库使用
//	client := transformer.NewDefaultClient()
//	resp, _ := client.ChatCompletion(ctx, messages, "claude-sonnet-4-5-20250929")
package transformer

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// ============ 全局配置参数 - 可直接修改 ============

var (
	// DefaultAPIKey 默认的 Claude API Key
	DefaultAPIKey = "ak-ba42b93ea28047389f9a621d2d6267b2"

	// DefaultBaseURL 默认的 Claude API 端点
	DefaultBaseURL = "https://api.routin.ai"

	// DefaultModel 默认使用的模型
	DefaultModel = "claude-sonnet-4-5-20250929"

	// DefaultMaxTokens 默认最大 token 数
	DefaultMaxTokens = 4096

	// DefaultAPIVersion Claude API 版本
	DefaultAPIVersion = "2023-06-01"

	// DefaultTimeout HTTP 请求超时时间
	DefaultTimeout = 120 * time.Second
)

const (
	// DefaultServerPort 本地服务器默认端口
	DefaultServerPort = ":9999"

	// EndpointChatCompletions OpenAI 兼容的聊天接口路径
	EndpointChatCompletions = "/v1/chat/completions"

	// EndpointHealth 健康检查接口路径
	EndpointHealth = "/health"
)

// ============ Anthropic (Claude) API 类型定义 ============

type AnthropicMessageRequest struct {
	Model         string          `json:"model"`
	System        json.RawMessage `json:"system,omitempty"`
	Messages      []AnthropicMsg  `json:"messages"`
	Tools         []AnthropicTool `json:"tools,omitempty"`
	MaxTokens     int             `json:"max_tokens,omitempty"`
	Temperature   *float64        `json:"temperature,omitempty"`
	StopSequences []string        `json:"stop_sequences,omitempty"`
	Stream        bool            `json:"stream,omitempty"`
}

type AnthropicMsg struct {
	Role    string          `json:"role"`
	Content json.RawMessage `json:"content"` // string or []AnthropicContent
}

type AnthropicContent struct {
	Type      string           `json:"type"` // text | tool_use | tool_result
	Text      string           `json:"text,omitempty"`
	ID        string           `json:"id,omitempty"`
	Name      string           `json:"name,omitempty"`
	Input     *json.RawMessage `json:"input,omitempty"`
	ToolUseID string           `json:"tool_use_id,omitempty"`
	Content   interface{}      `json:"content,omitempty"`
}

type AnthropicTool struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description,omitempty"`
	InputSchema map[string]interface{} `json:"input_schema"`
}

type AnthropicMessageResponse struct {
	ID           string                   `json:"id"`
	Type         string                   `json:"type"`
	Role         string                   `json:"role"`
	Model        string                   `json:"model"`
	Content      []map[string]interface{} `json:"content"`
	StopReason   *string                  `json:"stop_reason"`
	StopSequence *string                  `json:"stop_sequence"`
	Usage        *AnthropicUsage          `json:"usage,omitempty"`
}

type AnthropicUsage struct {
	InputTokens  int `json:"input_tokens"`
	OutputTokens int `json:"output_tokens"`
}

// ============ OpenAI API 类型定义 ============

type OpenAIChatRequest struct {
	Model       string          `json:"model"`
	Messages    []OpenAIMessage `json:"messages"`
	Tools       []OpenAITool    `json:"tools,omitempty"`
	Temperature *float64        `json:"temperature,omitempty"`
	MaxTokens   int             `json:"max_tokens,omitempty"`
	Stop        []string        `json:"stop,omitempty"`
	Stream      bool            `json:"stream,omitempty"`
}

type OpenAIMessage struct {
	Role       string           `json:"role"`
	Content    interface{}      `json:"content,omitempty"`
	Name       string           `json:"name,omitempty"`
	ToolCallID string           `json:"tool_call_id,omitempty"`
	ToolCalls  []OpenAIToolCall `json:"tool_calls,omitempty"`
}

type OpenAITool struct {
	Type     string         `json:"type"`
	Function OpenAIFunction `json:"function"`
}

type OpenAIFunction struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description,omitempty"`
	Parameters  map[string]interface{} `json:"parameters,omitempty"`
}

type OpenAIToolCallFunction struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
}

type OpenAIToolCall struct {
	ID       string                 `json:"id"`
	Type     string                 `json:"type"`
	Function OpenAIToolCallFunction `json:"function"`
}

type OpenAIChatResponse struct {
	ID      string `json:"id"`
	Object  string `json:"object"`
	Model   string `json:"model"`
	Choices []struct {
		Index        int           `json:"index"`
		FinishReason string        `json:"finish_reason"`
		Message      OpenAIMessage `json:"message"`
	} `json:"choices"`
	Usage *struct {
		PromptTokens     int `json:"prompt_tokens"`
		CompletionTokens int `json:"completion_tokens"`
		TotalTokens      int `json:"total_tokens"`
	} `json:"usage,omitempty"`
}

type OpenAIStreamChunk struct {
	ID      string `json:"id"`
	Object  string `json:"object"`
	Model   string `json:"model"`
	Choices []struct {
		Index int `json:"index"`
		Delta struct {
			Role      string `json:"role,omitempty"`
			Content   string `json:"content,omitempty"`
			ToolCalls []struct {
				ID       string `json:"id,omitempty"`
				Type     string `json:"type"`
				Index    int    `json:"index"`
				Function struct {
					Name      string `json:"name,omitempty"`
					Arguments string `json:"arguments,omitempty"`
				} `json:"function"`
			} `json:"tool_calls,omitempty"`
		} `json:"delta"`
		FinishReason string `json:"finish_reason,omitempty"`
	} `json:"choices"`
}

// ============ 转换函数：OpenAI → Anthropic ============

func OpenAIToAnthropicRequest(oreq OpenAIChatRequest) (AnthropicMessageRequest, error) {
	var systemStr string
	var msgs []AnthropicMsg

	for _, m := range oreq.Messages {
		switch m.Role {
		case "system":
			if systemStr == "" {
				if s, ok := m.Content.(string); ok {
					systemStr = s
				} else if arr, ok := m.Content.([]interface{}); ok {
					var buf []string
					for _, it := range arr {
						if mp, ok := it.(map[string]interface{}); ok {
							if mp["type"] == "text" {
								if ts, ok := mp["text"].(string); ok && strings.TrimSpace(ts) != "" {
									buf = append(buf, ts)
								}
							}
						}
					}
					if len(buf) > 0 {
						systemStr = strings.Join(buf, "\n\n")
					}
				}
			}
		case "user":
			if s, ok := m.Content.(string); ok {
				arr := []AnthropicContent{{Type: "text", Text: s}}
				raw, _ := json.Marshal(arr)
				msgs = append(msgs, AnthropicMsg{Role: "user", Content: raw})
			} else if arr, ok := m.Content.([]interface{}); ok {
				var parts []AnthropicContent
				for _, it := range arr {
					if mp, ok := it.(map[string]interface{}); ok {
						if mp["type"] == "text" {
							if ts, ok := mp["text"].(string); ok && strings.TrimSpace(ts) != "" {
								parts = append(parts, AnthropicContent{Type: "text", Text: ts})
							}
						}
					}
				}
				if len(parts) > 0 {
					raw, _ := json.Marshal(parts)
					msgs = append(msgs, AnthropicMsg{Role: "user", Content: raw})
				}
			}
		case "assistant":
			var parts []AnthropicContent
			if s, ok := m.Content.(string); ok && strings.TrimSpace(s) != "" {
				parts = append(parts, AnthropicContent{Type: "text", Text: s})
			}
			if arr, ok := m.Content.([]interface{}); ok {
				for _, it := range arr {
					if mp, ok := it.(map[string]interface{}); ok {
						if mp["type"] == "text" {
							if ts, ok := mp["text"].(string); ok && strings.TrimSpace(ts) != "" {
								parts = append(parts, AnthropicContent{Type: "text", Text: ts})
							}
						}
					}
				}
			}
			for _, tc := range m.ToolCalls {
				var inRaw json.RawMessage
				if tc.Function.Arguments != "" {
					inRaw = json.RawMessage([]byte(tc.Function.Arguments))
				}
				parts = append(parts, AnthropicContent{Type: "tool_use", ID: tc.ID, Name: tc.Function.Name, Input: &inRaw})
			}
			if len(parts) > 0 {
				raw, _ := json.Marshal(parts)
				msgs = append(msgs, AnthropicMsg{Role: "assistant", Content: raw})
			}
		case "tool":
			var contentStr string
			switch v := m.Content.(type) {
			case string:
				contentStr = v
			case nil:
				contentStr = ""
			default:
				b, _ := json.Marshal(v)
				contentStr = string(b)
			}
			parts := []AnthropicContent{{Type: "tool_result", ToolUseID: m.ToolCallID, Content: contentStr}}
			raw, _ := json.Marshal(parts)
			msgs = append(msgs, AnthropicMsg{Role: "user", Content: raw})
		}
	}

	var sysRaw json.RawMessage
	if systemStr != "" {
		sysRaw = json.RawMessage([]byte(strconvQuote(systemStr)))
	}

	return AnthropicMessageRequest{
		Model:         oreq.Model,
		System:        sysRaw,
		Messages:      msgs,
		Tools:         mapToolsToAnthropic(oreq.Tools),
		MaxTokens:     oreq.MaxTokens,
		Temperature:   oreq.Temperature,
		StopSequences: oreq.Stop,
		Stream:        oreq.Stream,
	}, nil
}

func mapToolsToAnthropic(tools []OpenAITool) []AnthropicTool {
	if len(tools) == 0 {
		return nil
	}
	out := make([]AnthropicTool, 0, len(tools))
	for _, t := range tools {
		if strings.ToLower(t.Type) != "function" {
			continue
		}
		out = append(out, AnthropicTool{
			Name:        t.Function.Name,
			Description: t.Function.Description,
			InputSchema: t.Function.Parameters,
		})
	}
	return out
}

func strconvQuote(s string) string {
	b, _ := json.Marshal(s)
	return string(b)
}

// ============ 转换函数：Anthropic → OpenAI ============

func AnthropicToOpenAIResponse(a AnthropicMessageResponse, openaiModel string) (OpenAIChatResponse, error) {
	var contentStr string
	var toolCalls []OpenAIToolCall

	for _, c := range a.Content {
		if t, ok := c["type"].(string); ok {
			switch t {
			case "text":
				if s, ok := c["text"].(string); ok {
					if contentStr == "" {
						contentStr = s
					} else {
						contentStr += "\n\n" + s
					}
				}
			case "tool_use":
				name, _ := c["name"].(string)
				id, _ := c["id"].(string)
				args := "{}"
				if in, ok := c["input"]; ok && in != nil {
					b, _ := json.Marshal(in)
					if len(b) > 0 {
						args = string(b)
					}
				}
				toolCalls = append(toolCalls, OpenAIToolCall{ID: id, Type: "function", Function: OpenAIToolCallFunction{Name: name, Arguments: args}})
			}
		}
	}

	msg := OpenAIMessage{Role: "assistant"}
	if contentStr != "" {
		msg.Content = contentStr
	}
	if len(toolCalls) > 0 {
		msg.ToolCalls = toolCalls
	}

	finish := "stop"
	if a.StopReason != nil && *a.StopReason == "tool_use" {
		finish = "tool_calls"
	}

	return OpenAIChatResponse{
		ID:     a.ID,
		Object: "chat.completion",
		Model:  openaiModel,
		Choices: []struct {
			Index        int           `json:"index"`
			FinishReason string        `json:"finish_reason"`
			Message      OpenAIMessage `json:"message"`
		}{{Index: 0, FinishReason: finish, Message: msg}},
		Usage: nil,
	}, nil
}

// ============ 流式转换：Anthropic SSE → OpenAI ============

func ConvertAnthropicStreamToOpenAI(ctx context.Context, openaiModel string, body io.Reader, emit func(chunk map[string]interface{})) error {
	roleSent := false
	nextToolIdx := 0
	contentIdxToToolIdx := map[int]int{}
	toolArgsByToolIdx := map[int]string{}
	reader := bufio.NewReader(body)

	send := func(delta map[string]interface{}, finishReason string) {
		ch := map[string]interface{}{
			"id":      fmt.Sprintf("chatcmplchunk_%d", time.Now().UnixNano()),
			"object":  "chat.completion.chunk",
			"model":   openaiModel,
			"choices": []map[string]interface{}{{"index": 0, "delta": delta}},
		}
		if finishReason != "" {
			ch["choices"].([]map[string]interface{})[0]["finish_reason"] = finishReason
		}
		emit(ch)
	}

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		line, err := reader.ReadString('\n')
		if err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			break
		}

		line = strings.TrimSpace(line)
		if line == "" || !strings.HasPrefix(line, "data:") {
			continue
		}

		payload := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
		if payload == "" || payload == "[DONE]" {
			continue
		}

		// 解析事件类型
		var baseEvent struct {
			Type string `json:"type"`
		}
		if err := json.Unmarshal([]byte(payload), &baseEvent); err != nil {
			continue
		}

		switch baseEvent.Type {
		case "message_start":
			if !roleSent {
				send(map[string]interface{}{"role": "assistant"}, "")
				roleSent = true
			}

		case "content_block_start":
			var obj struct {
				Type         string                 `json:"type"`
				Index        int                    `json:"index"`
				ContentBlock map[string]interface{} `json:"content_block"`
			}
			if err := json.Unmarshal([]byte(payload), &obj); err != nil {
				continue
			}
			if t, _ := obj.ContentBlock["type"].(string); t == "tool_use" {
				id, _ := obj.ContentBlock["id"].(string)
				name, _ := obj.ContentBlock["name"].(string)
				toolIdx := nextToolIdx
				contentIdxToToolIdx[obj.Index] = toolIdx
				delta := map[string]interface{}{
					"tool_calls": []map[string]interface{}{
						{
							"id":    id,
							"type":  "function",
							"index": toolIdx,
							"function": map[string]interface{}{
								"name": name,
							},
						},
					},
				}
				send(delta, "")
				nextToolIdx++
			}

		case "content_block_delta":
			var obj struct {
				Type  string                 `json:"type"`
				Index int                    `json:"index"`
				Delta map[string]interface{} `json:"delta"`
			}
			if err := json.Unmarshal([]byte(payload), &obj); err != nil {
				continue
			}
			if obj.Delta == nil {
				continue
			}
			if obj.Delta["type"] == "text_delta" {
				if s, _ := obj.Delta["text"].(string); s != "" {
					send(map[string]interface{}{"content": s}, "")
				}
			} else if obj.Delta["type"] == "input_json_delta" {
				piece, _ := obj.Delta["partial_json"].(string)
				if piece == "" {
					if v, ok := obj.Delta["delta"].(string); ok {
						piece = v
					}
				}
				toolIdx, ok := contentIdxToToolIdx[obj.Index]
				if !ok {
					continue
				}
				toolArgsByToolIdx[toolIdx] += piece
				delta := map[string]interface{}{
					"tool_calls": []map[string]interface{}{
						{
							"index": toolIdx,
							"type":  "function",
							"function": map[string]interface{}{
								"arguments": piece,
							},
						},
					},
				}
				send(delta, "")
			}

		case "content_block_stop":
			// ignore

		case "message_delta":
			var obj struct {
				Type  string `json:"type"`
				Delta struct {
					StopReason string `json:"stop_reason"`
				} `json:"delta"`
			}
			if err := json.Unmarshal([]byte(payload), &obj); err != nil {
				continue
			}
			if obj.Delta.StopReason != "" {
				finishReason := "stop"
				switch obj.Delta.StopReason {
				case "end_turn":
					finishReason = "stop"
				case "max_tokens":
					finishReason = "length"
				case "tool_use":
					finishReason = "tool_calls"
				case "stop_sequence":
					finishReason = "stop"
				default:
					finishReason = "stop"
				}
				send(map[string]interface{}{}, finishReason)
			}

		case "message_stop":
			// finish_reason already sent in message_delta

		case "ping":
			// ignore

		case "error":
			// ignore
		}
	}
	return nil
}

// ============ HTTP 服务器功能 ============

// StartServer 启动 HTTP 转发服务器（使用全局默认配置）
//
// 使用示例：
//
//	transformer.DefaultAPIKey = "your-api-key"
//	transformer.StartServer() // 启动在 :9999
func StartServer() error {
	return StartServerWithPort(DefaultServerPort)
}

// StartServerWithPort 在指定端口启动 HTTP 转发服务器
func StartServerWithPort(port string) error {
	mux := http.NewServeMux()

	// 健康检查
	mux.HandleFunc(EndpointHealth, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	})

	// OpenAI 兼容的聊天接口
	mux.HandleFunc(EndpointChatCompletions, handleChatCompletion)

	server := &http.Server{
		Addr:         port,
		Handler:      loggingMiddleware(mux),
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 120 * time.Second,
	}

	fmt.Printf("🚀 Claude API 转发服务启动在 http://localhost%s\n", port)
	fmt.Printf("📝 OpenAI 兼容端点: http://localhost%s%s\n", port, EndpointChatCompletions)
	fmt.Printf("🔧 转发到: %s (模型: %s)\n", DefaultBaseURL, DefaultModel)
	fmt.Println("---")

	return server.ListenAndServe()
}

// handleChatCompletion 处理聊天请求
func handleChatCompletion(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// 解析 OpenAI 格式请求
	var oreq OpenAIChatRequest
	if err := json.NewDecoder(r.Body).Decode(&oreq); err != nil {
		http.Error(w, "invalid json", http.StatusBadRequest)
		return
	}

	// 转换为 Anthropic 格式
	areq, err := OpenAIToAnthropicRequest(oreq)
	if err != nil {
		http.Error(w, "invalid messages: "+err.Error(), http.StatusBadRequest)
		return
	}

	// 设置默认 max_tokens（Anthropic 必需）
	if areq.MaxTokens == 0 {
		areq.MaxTokens = DefaultMaxTokens
	}

	// 处理流式请求
	if areq.Stream {
		handleStreamRequest(w, r.Context(), areq, oreq.Model)
		return
	}

	// 处理非流式请求
	handleNonStreamRequest(w, r.Context(), areq, oreq.Model)
}

// handleNonStreamRequest 处理非流式请求
func handleNonStreamRequest(w http.ResponseWriter, ctx context.Context, areq AnthropicMessageRequest, openaiModel string) {
	// 发送到 Anthropic
	reqBody, _ := json.Marshal(areq)
	req, err := http.NewRequestWithContext(ctx, "POST", DefaultBaseURL+"/v1/messages", bytes.NewReader(reqBody))
	if err != nil {
		http.Error(w, "create request failed: "+err.Error(), http.StatusInternalServerError)
		return
	}

	setAuthHeaders(req)

	client := &http.Client{Timeout: DefaultTimeout}
	resp, err := client.Do(req)
	if err != nil {
		http.Error(w, "anthropic request failed: "+err.Error(), http.StatusBadGateway)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 8192))
		http.Error(w, fmt.Sprintf("anthropic error %d: %s", resp.StatusCode, string(body)), http.StatusBadGateway)
		return
	}

	// 解析 Anthropic 响应
	var aresp AnthropicMessageResponse
	if err := json.NewDecoder(resp.Body).Decode(&aresp); err != nil {
		http.Error(w, "invalid anthropic response", http.StatusBadGateway)
		return
	}

	// 转换为 OpenAI 格式
	oresp, err := AnthropicToOpenAIResponse(aresp, openaiModel)
	if err != nil {
		http.Error(w, "mapping error: "+err.Error(), http.StatusBadGateway)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(oresp)
}

// handleStreamRequest 处理流式请求
func handleStreamRequest(w http.ResponseWriter, ctx context.Context, areq AnthropicMessageRequest, openaiModel string) {
	areq.Stream = true

	// 发送到 Anthropic
	reqBody, _ := json.Marshal(areq)
	req, err := http.NewRequestWithContext(ctx, "POST", DefaultBaseURL+"/v1/messages", bytes.NewReader(reqBody))
	if err != nil {
		http.Error(w, "create request failed: "+err.Error(), http.StatusInternalServerError)
		return
	}

	setAuthHeaders(req)

	client := &http.Client{Timeout: DefaultTimeout}
	resp, err := client.Do(req)
	if err != nil {
		http.Error(w, "anthropic stream failed: "+err.Error(), http.StatusBadGateway)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 8192))
		http.Error(w, fmt.Sprintf("anthropic error %d: %s", resp.StatusCode, string(body)), http.StatusBadGateway)
		return
	}

	// 设置 SSE 响应头
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming not supported", http.StatusInternalServerError)
		return
	}

	// 转换 Anthropic SSE 到 OpenAI 格式
	_ = ConvertAnthropicStreamToOpenAI(ctx, openaiModel, resp.Body, func(chunk map[string]interface{}) {
		b, _ := json.Marshal(chunk)
		fmt.Fprintf(w, "data: %s\n\n", string(b))
		flusher.Flush()
	})

	fmt.Fprintf(w, "data: [DONE]\n\n")
	flusher.Flush()
}

// setAuthHeaders 设置 Anthropic API 认证头
func setAuthHeaders(req *http.Request) {
	req.Header.Set("Content-Type", "application/json")
	if DefaultAPIKey != "" {
		req.Header.Set("x-api-key", DefaultAPIKey)
		req.Header.Set("Authorization", "Bearer "+DefaultAPIKey)
		req.Header.Set("Meteor-Api-Key", DefaultAPIKey) // routin.ai 特定
	}
	if DefaultAPIVersion != "" {
		req.Header.Set("anthropic-version", DefaultAPIVersion)
	}
}

// loggingMiddleware 日志中间件
func loggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		sw := &statusWriter{ResponseWriter: w, statusCode: 200}
		next.ServeHTTP(sw, r)
		fmt.Printf("%s %s %s %d %s\n",
			r.RemoteAddr,
			r.Method,
			r.URL.Path,
			sw.statusCode,
			time.Since(start))
	})
}

type statusWriter struct {
	http.ResponseWriter
	statusCode int
}

func (sw *statusWriter) WriteHeader(code int) {
	sw.statusCode = code
	sw.ResponseWriter.WriteHeader(code)
}

func (sw *statusWriter) Flush() {
	if f, ok := sw.ResponseWriter.(http.Flusher); ok {
		f.Flush()
	}
}
