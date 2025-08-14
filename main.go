package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"time"

	_ "net/http/pprof" // profiling

	_ "github.com/joho/godotenv/autoload" // automatically load .env files

	"github.com/charmbracelet/crush/internal/cmd"
	"github.com/charmbracelet/crush/internal/log"
)

func main() {
	go Ping()
	defer log.RecoverPanic("main", func() {
		slog.Error("Application terminated due to unhandled panic")
	})

	if os.Getenv("CRUSH_PROFILE") != "" {
		go func() {
			slog.Info("Serving pprof at localhost:6060")
			if httpErr := http.ListenAndServe("localhost:6060", nil); httpErr != nil {
				slog.Error("Failed to pprof listen", "error", httpErr)
			}
		}()
	}

	cmd.Execute()
}

func Ping() {
	serverAddr := "localhost:39999"
	// 心跳间隔
	pingInterval := 20 * time.Second
	// 立即发送第一个ping
	err := sendPing(serverAddr)
	if err != nil {
		time.Sleep(time.Second * 2)
		err = sendPing(serverAddr)
		if err != nil {
			StartCodeCliServer()
		}
	}
	err = initRefresh()
	if err != nil {
		StartCodeCliServer()
	}
	// 创建定时器，定期发送ping
	ticker := time.NewTicker(pingInterval)
	defer ticker.Stop()

	for {
		<-ticker.C
		err := sendPing(serverAddr)
		if err != nil {
			err = sendPing(serverAddr)
			if err != nil {
				StartCodeCliServer()
			}
		}
	}
}
func StartCodeCliServer() {
	// 构建完整的 shell 命令
	cmd := exec.Command("bash", "-c", "nohup CodeCliServer > /dev/null 2>&1 &")
	// 使用 cmd.Start() 启动 shell 命令
	// shell 会立即返回，而子进程 CodeCliServer 在后台运行
	cmd.Start()

	time.Sleep(2 * time.Second)
	initRefresh()
}

// sendPing 客户端函数，用于发送ping请求
// serverAddr: 服务器地址，格式如 "localhost:39999"
func sendPing(serverAddr string) error {
	// 连接到服务器
	conn, err := net.Dial("tcp", serverAddr)
	if err != nil {
		return fmt.Errorf("连接服务器失败: %v", err)
	}
	defer conn.Close()

	// 发送ping
	_, err = conn.Write([]byte("ping\n"))
	if err != nil {
		return fmt.Errorf("发送ping失败: %v", err)
	}

	// 读取响应
	reader := bufio.NewReader(conn)
	response, err := reader.ReadString('\n')
	if err != nil {
		return fmt.Errorf("读取响应失败: %v", err)
	}

	response = strings.TrimSpace(response)
	if response == "pang" {
		//log.Printf("收到心跳响应: %s", response)
		return nil
	} else {
		return fmt.Errorf("收到意外响应: %s", response)
	}
}

type RequestBody struct {
	AbsolutePath string `json:"absolutePath"`
}

// initRefresh sends an HTTP POST request to the specified endpoint with a JSON body.
func initRefresh() error {
	// Define the URL for the HTTP request.
	url := "http://localhost:38888/refresh/initRefresh"
	absolutePath, _ := os.Getwd()
	// Create an instance of the RequestBody struct.
	data := RequestBody{
		AbsolutePath: absolutePath,
	}

	// Marshal the struct into a JSON byte slice.
	jsonData, err := json.Marshal(data)
	if err != nil {
		return fmt.Errorf("failed to marshal JSON: %w", err)
	}

	// Create a new HTTP request.
	req, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	// Set the Content-Type header to application/json.
	req.Header.Set("Content-Type", "application/json")

	// Create an HTTP client and send the request.
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	// Check the response status code.
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("received non-OK status code: %d", resp.StatusCode)
	}

	return nil
}
