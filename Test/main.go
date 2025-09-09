package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

type Message struct {
	Role      string     `json:"role"`
	Content   string     `json:"content,omitempty"`
	ToolCalls []ToolCall `json:"tool_calls,omitempty"`
}

type ToolCall struct {
	ID       string   `json:"id"`
	Type     string   `json:"type"`
	Function Function `json:"function"`
}

type Function struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
}

type ChatRequest struct {
	Model    string    `json:"model"`
	Messages []Message `json:"messages"`
	Stream   bool      `json:"stream"`
}

type StreamingHTTPTester struct {
	client  *http.Client
	url     string
	apiKey  string
	timeout time.Duration
}

func NewStreamingHTTPTester() *StreamingHTTPTester {
	return &StreamingHTTPTester{
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
		url:     "https://api.bltcy.ai/v1/chat/completions",
		apiKey:  "sk-FGpV8OIhWZDgcJMZGeUUhf1zPbvThdWi11KjWOh8Q0MdMvy9",
		timeout: 2 * time.Second, // 5秒后中断连接
	}
}

func (t *StreamingHTTPTester) createTestRequest() *ChatRequest {
	return &ChatRequest{
		Model: "claude-sonnet-4-20250514",
		Messages: []Message{
			{
				Role:    "user",
				Content: "帮我查看文件A.txt、B.txt、C.txt和D.txt的内容。你必须并发调用多个文件，让我测试你的并发调用tool的能力",
			},
			{
				Role:    "assistant",
				Content: "我来帮你并发读取这四个文件的内容。让我同时调用多个文件读取函数来测试并发能力：",
				ToolCalls: []ToolCall{
					{
						ID:   "toolu_01Y2D8dQqwV629B977NPGncj",
						Type: "function",
						Function: Function{
							Name:      "read_file",
							Arguments: "{\"file_name\":\"A.txt\"}",
						},
					},
					{
						ID:   "toolu_013SN1NWq9no35XHinCHKJS2",
						Type: "function",
						Function: Function{
							Name:      "read_file",
							Arguments: "{\"file_name\":\"B.txt\"}",
						},
					},
					{
						ID:   "toolu_017je26MU8WCGpbF1uXPV9SJ",
						Type: "function",
						Function: Function{
							Name:      "read_file",
							Arguments: "{\"file_name\":\"C.txt\"}",
						},
					},
					{
						ID:   "toolu_001",
						Type: "function",
						Function: Function{
							Name:      "read_file",
							Arguments: "{\"file_name\":\"D.txt\"}",
						},
					},
				},
			},
			{
				Role:    "tool",
				Content: "这是A.txt的内容: 你好A......",
			},
			{
				Role:    "tool",
				Content: "这是B.txt的内容: 你好B......",
			},
			{
				Role:    "tool",
				Content: "这是C.txt的内容: 你好C......",
			},
			{
				Role:    "tool",
				Content: "这是D.txt的内容: 你好D......",
			},
		},
		Stream: true,
	}
}

func (t *StreamingHTTPTester) testStreamingWithInterruption() error {
	fmt.Println("开始测试流式HTTP请求并中断连接...")

	// 创建请求
	request := t.createTestRequest()
	jsonData, err := json.Marshal(request)
	if err != nil {
		return fmt.Errorf("序列化请求失败: %v", err)
	}

	// 创建带超时的context
	ctx, cancel := context.WithTimeout(context.Background(), t.timeout)
	defer cancel()

	// 创建HTTP请求
	req, err := http.NewRequestWithContext(ctx, "POST", t.url, bytes.NewBuffer(jsonData))
	if err != nil {
		return fmt.Errorf("创建HTTP请求失败: %v", err)
	}

	// 设置请求头
	req.Header.Set("Authorization", "Bearer "+t.apiKey)
	req.Header.Set("Content-Type", "application/json")

	fmt.Printf("发送请求到: %s\n", t.url)
	fmt.Printf("超时设置: %v\n", t.timeout)

	// 发送请求
	resp, err := t.client.Do(req)
	if err != nil {
		return fmt.Errorf("发送HTTP请求失败: %v", err)
	}
	defer resp.Body.Close()

	fmt.Printf("响应状态码: %d\n", resp.StatusCode)
	fmt.Printf("响应头: %+v\n", resp.Header)

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("API返回错误状态码: %d, 响应体: %s", resp.StatusCode, string(body))
	}

	// 读取流式响应直到超时中断
	fmt.Println("\n开始读取流式响应数据...")
	buffer := make([]byte, 1024)
	totalBytes := 0
	startTime := time.Now()

	for {
		select {
		case <-ctx.Done():
			fmt.Printf("\n连接在 %v 后被中断 (按设计)\n", time.Since(startTime))
			fmt.Printf("总共接收到 %d 字节数据\n", totalBytes)
			fmt.Println("测试成功：成功在流式输出中间中断了HTTP连接")
			return nil

		default:
			n, err := resp.Body.Read(buffer)
			if n > 0 {
				totalBytes += n
				fmt.Printf("接收到数据块 (%d 字节): %s", n, string(buffer[:n]))
			}

			if err != nil {
				if err == io.EOF {
					fmt.Println("\n流结束")
					return nil
				}
				fmt.Printf("\n读取错误: %v\n", err)
				return err
			}
		}
	}
}

// timeoutReader 包装器，用于实现读取超时
type timeoutReader struct {
	reader  io.Reader
	timeout time.Duration
}

func (tr *timeoutReader) Read(p []byte) (n int, err error) {
	return tr.reader.Read(p)
}

func (t *StreamingHTTPTester) runMultipleTests() {
	fmt.Println("=== 流式HTTP中断测试套件 ===\n")

	// 测试1: 5秒超时
	fmt.Println("测试1: 5秒超时中断")
	t.timeout = 5 * time.Second
	if err := t.testStreamingWithInterruption(); err != nil {
		fmt.Printf("测试1失败: %v\n", err)
	}

	fmt.Println("\n" + strings.Repeat("=", 50) + "\n")

	// 测试2: 3秒超时
	fmt.Println("测试2: 3秒超时中断")
	t.timeout = 3 * time.Second
	if err := t.testStreamingWithInterruption(); err != nil {
		fmt.Printf("测试2失败: %v\n", err)
	}

	fmt.Println("\n" + strings.Repeat("=", 50) + "\n")

	// 测试3: 1秒超时
	fmt.Println("测试3: 1秒超时中断")
	t.timeout = 1 * time.Second
	if err := t.testStreamingWithInterruption(); err != nil {
		fmt.Printf("测试3失败: %v\n", err)
	}
}

func main() {
	fmt.Println("HTTP流式请求中断测试程序")
	fmt.Println("此程序会发送流式请求并在中途中断连接以测试程序的处理能力\n")

	tester := NewStreamingHTTPTester()

	// 运行多个测试
	tester.runMultipleTests()

	fmt.Println("\n所有测试完成!")
}
