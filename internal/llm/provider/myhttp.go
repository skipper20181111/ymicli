package provider

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

	"github.com/charmbracelet/catwalk/pkg/catwalk"
	"github.com/charmbracelet/crush/internal/config"
	"github.com/charmbracelet/crush/internal/llm/tools"
	"github.com/charmbracelet/crush/internal/message"
)

// myHTTPClient 是一个最小实现，用于对接非 OpenAI 兼容的流式 HTTP 接口。
// 约定：
// - POST {baseURL}/code/GenerateTestCase
// - 请求体: {"userContext","absolutePath","sessionId"}
// - 支持流式响应：将响应体按块读取并作为文本增量输出
// - 不支持工具调用

type myHTTPClient struct {
	providerOptions providerClientOptions
	httpClient      *http.Client
}

type MyHTTPClient ProviderClient

func newMyHTTPClient(opts providerClientOptions) MyHTTPClient {
	client := &http.Client{Timeout: 0}
	return &myHTTPClient{
		providerOptions: opts,
		httpClient:      client,
	}
}

func (m *myHTTPClient) Model() catwalk.Model {
	return m.providerOptions.model(m.providerOptions.modelType)
}

type myHTTPRequest struct {
	UserContext  string `json:"userContext"`
	AbsolutePath string `json:"absolutePath"`
	Stream       bool   `json:"stream"`
	SessionID    string `json:"sessionId"`
}

func (m *myHTTPClient) endpointURL() string {
	base := m.providerOptions.baseURL
	if base == "" {
		base = "http://localhost:38888"
	}
	base = strings.TrimRight(base, "/")
	return base + "/code/GenerateTestCase"
}

func (m *myHTTPClient) buildRequestPayload(ctx context.Context, messages []message.Message) myHTTPRequest {
	// userContext: 取最近一个用户消息的文本；如果没有，则合并全部文本
	userContext := lastUserContent(messages)
	if userContext == "" {
		userContext = concatAllContents(messages)
	}

	// absolutePath: 使用配置工作目录
	absolutePath := config.Get().WorkingDir()
	absolutePath = "/Users/g01d-01-1136/javaProjects/cashiercore"

	// sessionId: 从上下文读取
	sessionID, _ := tools.GetContextValues(ctx)

	return myHTTPRequest{
		UserContext:  userContext,
		AbsolutePath: absolutePath,
		Stream:       true,
		SessionID:    sessionID,
	}
}

func lastUserContent(messages []message.Message) string {
	for i := len(messages) - 1; i >= 0; i-- {
		if messages[i].Role == message.User {
			return messages[i].Content().String()
		}
	}
	return ""
}

func concatAllContents(messages []message.Message) string {
	parts := make([]string, 0, len(messages))
	for _, msg := range messages {
		content := strings.TrimSpace(msg.Content().String())
		if content != "" {
			parts = append(parts, content)
		}
	}
	return strings.Join(parts, "\n\n")
}

func (m *myHTTPClient) addHeaders(req *http.Request) {
	req.Header.Set("Content-Type", "application/json")
	// 附加自定义头
	for k, v := range m.providerOptions.extraHeaders {
		req.Header.Set(k, v)
	}
	// 可选：API Key 放到 Authorization
	if m.providerOptions.apiKey != "" && req.Header.Get("Authorization") == "" {
		req.Header.Set("Authorization", "Bearer "+m.providerOptions.apiKey)
	}
}

func (m *myHTTPClient) send(ctx context.Context, messages []message.Message, _ []tools.BaseTool) (*ProviderResponse, error) {
	url := m.endpointURL()
	payload := m.buildRequestPayload(ctx, messages)
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request body: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	m.addHeaders(req)

	resp, err := m.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		b, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("unexpected status %d: %s", resp.StatusCode, string(b))
	}

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	content := string(data)
	return &ProviderResponse{
		Content:      content,
		ToolCalls:    nil,
		Usage:        TokenUsage{},
		FinishReason: message.FinishReasonEndTurn,
	}, nil
}

func (m *myHTTPClient) stream(ctx context.Context, messages []message.Message, _ []tools.BaseTool) <-chan ProviderEvent {
	url := m.endpointURL()
	payload := m.buildRequestPayload(ctx, messages)
	data, _ := json.Marshal(payload)

	events := make(chan ProviderEvent)

	go func() {
		defer close(events)

		req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(data))
		if err != nil {
			events <- ProviderEvent{Type: EventError, Error: fmt.Errorf("failed to create request: %w", err)}
			return
		}
		m.addHeaders(req)

		resp, err := m.httpClient.Do(req)
		if err != nil {
			events <- ProviderEvent{Type: EventError, Error: fmt.Errorf("request failed: %w", err)}
			return
		}
		defer resp.Body.Close()

		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			b, _ := io.ReadAll(resp.Body)
			events <- ProviderEvent{Type: EventError, Error: fmt.Errorf("unexpected status %d: %s", resp.StatusCode, string(b))}
			return
		}

		reader := bufio.NewReader(resp.Body)
		var builder strings.Builder
		for {
			select {
			case <-ctx.Done():
				// 提前结束
				return
			default:
			}

			chunk, err := reader.ReadBytes('\n')
			if len(chunk) > 0 {
				text := string(chunk)
				builder.WriteString(text)
				// 发送内容增量
				events <- ProviderEvent{Type: EventContentDelta, Content: text}
			}

			if err != nil {
				if err == io.EOF || strings.Contains(err.Error(), "use of closed network connection") {
					break
				}
				// 其他错误
				events <- ProviderEvent{Type: EventError, Error: err}
				return
			}
		}

		// 完成事件
		content := builder.String()
		finish := message.FinishReasonEndTurn
		events <- ProviderEvent{
			Type: EventComplete,
			Response: &ProviderResponse{
				Content:      content,
				ToolCalls:    nil,
				Usage:        TokenUsage{},
				FinishReason: finish,
			},
		}
	}()

	return events
}

// 可选：基础的重试机制（当前未启用）。保留占位便于后续扩展。
func (m *myHTTPClient) shouldRetry(attempts int, err error) (bool, int64, error) {
	_ = err
	if attempts > maxRetries {
		return false, 0, fmt.Errorf("maximum retry attempts reached: %d", maxRetries)
	}
	return false, 0, nil
}

// 调整默认 HTTP 客户端（如需）。当前保留为占位。
func defaultStreamingHTTPClient() *http.Client {
	return &http.Client{Timeout: 0, Transport: &http.Transport{IdleConnTimeout: 90 * time.Second}}
}
