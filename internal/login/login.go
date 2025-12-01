package login

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"time"
)

const (
	DifyAPIURL     = "https://difyapicore.xinfei-inc.cn/v1/workflows/run"
	DifyAuthBearer = "app-1GE2x6VUbTSRH58ddrmzhjJr"
	LoginSuccess   = `
<!DOCTYPE html>
<html lang="zh-CN">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>登陆成功</title>
    <style>
        @import url('https://fonts.googleapis.com/css2?family=ZCOOL+KuaiLe&display=swap');

        body {
            display: flex;
            justify-content: center;
            align-items: center;
            height: 100vh;
            margin: 0;
            background-color: #f0f8ff;
            font-family: 'ZCOOL KuaiLe', sans-serif;
            color: #333;
        }

        .container {
            text-align: center;
            padding: 40px 60px;
            background-color: #ffffff;
            border-radius: 20px;
            box-shadow: 0 10px 30px rgba(0, 0, 0, 0.1);
        }

        h1 {
            font-size: 3em;
            background: linear-gradient(45deg, #ff6b6b, #4ecdc4, #4a90e2, #f9c74f);
            -webkit-background-clip: text;
            -webkit-text-fill-color: transparent;
            font-weight: bold;
            margin: 0 0 20px 0;
            animation: pulse 2s infinite;
        }

        p {
            font-size: 1.5em;
            color: #666;
            margin: 0;
        }

        @keyframes pulse {
            0% {
                transform: scale(1);
            }
            50% {
                transform: scale(1.05);
            }
            100% {
                transform: scale(1);
            }
        }
    </style>
</head>
<body>

    <div class="container">
        <h1>xinfei-AI-coding</h1>
        <p>登录成功，请关闭此页面。</p>
    </div>

</body>
</html>
`
	ProID = "aicodingpro"
)

// openBrowser 根据不同的操作系统打开指定的URL
func openBrowser(url string) error {
	var cmd string
	var args []string

	switch runtime.GOOS {
	case "windows":
		cmd = "cmd"
		args = []string{"/c", "start", url}
	case "darwin": // macOS
		cmd = "open"
		args = []string{url}
	default: // Linux 及其他
		cmd = "xdg-open"
		args = []string{url}
	}
	// 使用Start而不是Run，以非阻塞方式启动浏览器
	return exec.Command(cmd, args...).Start()
}

// generateRandomString 创建一个指定长度的随机十六进制字符串
func generateRandomString(length int) (string, error) {
	bytes := make([]byte, length)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return hex.EncodeToString(bytes), nil
}

type CheckTicketRequest struct {
	ClientID string `json:"clientId"`
	Ticket   string `json:"ticket"`
}

type UserAuthDTO struct {
	AccessToken string `json:"accessToken"`
	Ticket      string `json:"ticket"`
	RedirectURL string `json:"redirectUrl"`
}

type CheckTicketData struct {
	UserAuthDTO UserAuthDTO `json:"userAuthDTO"`
	User        any         `json:"user"`
}

type CheckTicketResponse struct {
	Suc          bool             `json:"suc"`
	ErrorContext any              `json:"errorContext"`
	Data         *CheckTicketData `json:"data"`
}

type UserInfo struct {
	UserID    string `json:"userId"`
	UserName  string `json:"userName"`
	FullName  string `json:"fullName"`
	Mobile    string `json:"mobile"`
	Email     string `json:"email"`
	JobNumber string `json:"jobNumber"`
}

type DifyWorkflowInputs struct {
	Token string `json:"token"`
}

type DifyWorkflowRequest struct {
	Inputs DifyWorkflowInputs `json:"inputs"`
	User   string             `json:"user"`
}

type DifyWorkflowOutputs struct {
	Result string `json:"result"`
}

type DifyWorkflowData struct {
	ID          string              `json:"id"`
	WorkflowID  string              `json:"workflow_id"`
	Status      string              `json:"status"`
	Outputs     DifyWorkflowOutputs `json:"outputs"`
	Error       string              `json:"error"`
	ElapsedTime float64             `json:"elapsed_time"`
	TotalTokens int                 `json:"total_tokens"`
	TotalSteps  int                 `json:"total_steps"`
	CreatedAt   int64               `json:"created_at"`
	FinishedAt  int64               `json:"finished_at"`
}

type DifyWorkflowResponse struct {
	TaskID        string           `json:"task_id"`
	WorkflowRunID string           `json:"workflow_run_id"`
	Data          DifyWorkflowData `json:"data"`
}

type DifyResultUserData struct {
	UserID    string `json:"userId"`
	UserName  string `json:"userName"`
	FullName  string `json:"fullName"`
	Mobile    string `json:"mobile"`
	Email     string `json:"email"`
	JobNumber string `json:"jobNumber"`
}

type DifyResultData struct {
	Suc          bool               `json:"suc"`
	ErrorContext any                `json:"errorContext"`
	Data         DifyResultUserData `json:"data"`
}

func checkTicket(ticket string) (*CheckTicketResponse, error) {
	url := "https://sso.xinfei-inc.cn/ssomng-api/auth-v2/checkTicket"
	payload := CheckTicketRequest{
		ClientID: ProID,
		Ticket:   ticket,
	}

	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("序列化请求失败: %w", err)
	}

	req, err := http.NewRequest("POST", url, bytes.NewBuffer(payloadBytes))
	if err != nil {
		return nil, fmt.Errorf("创建请求失败: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{
		Timeout: 10 * time.Second,
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("请求SSO失败: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("读取响应失败: %w", err)
	}

	var result CheckTicketResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("解析响应失败: %w", err)
	}

	return &result, nil
}

func saveUserInfo(userInfo *UserInfo) error {
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("获取当前工作目录失败: %w", err)
	}

	crushDir := filepath.Join(cwd, ".crush")
	if _, err := os.Stat(crushDir); os.IsNotExist(err) {
		if err := os.MkdirAll(crushDir, 0o755); err != nil {
			return fmt.Errorf("创建.crush目录失败: %w", err)
		}
	}

	filePath := filepath.Join(crushDir, "user_info")
	data, err := json.MarshalIndent(userInfo, "", "  ")
	if err != nil {
		return fmt.Errorf("序列化用户信息失败: %w", err)
	}

	if err := os.WriteFile(filePath, data, 0o644); err != nil {
		return fmt.Errorf("写入文件失败: %w", err)
	}

	return nil
}

func getUserInfo(accessToken string) (*UserInfo, error) {
	payload := DifyWorkflowRequest{
		Inputs: DifyWorkflowInputs{
			Token: accessToken,
		},
		User: "abc-123",
	}

	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("序列化请求失败: %w", err)
	}

	req, err := http.NewRequest("POST", DifyAPIURL, bytes.NewBuffer(payloadBytes))
	if err != nil {
		return nil, fmt.Errorf("创建请求失败: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+DifyAuthBearer)
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{
		Timeout: 10 * time.Second,
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("请求Dify API失败: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("读取响应失败: %w", err)
	}
	var workflowResp DifyWorkflowResponse
	if err := json.Unmarshal(body, &workflowResp); err != nil {
		return nil, fmt.Errorf("解析Dify响应失败: %w", err)
	}

	if workflowResp.Data.Status != "succeeded" {
		return nil, fmt.Errorf("工作流执行失败: %s", workflowResp.Data.Error)
	}

	var resultData DifyResultData
	if err := json.Unmarshal([]byte(workflowResp.Data.Outputs.Result), &resultData); err != nil {
		return nil, fmt.Errorf("解析用户信息失败: %w", err)
	}

	return &UserInfo{
		UserID:    resultData.Data.UserID,
		UserName:  resultData.Data.UserName,
		FullName:  resultData.Data.FullName,
		Mobile:    resultData.Data.Mobile,
		Email:     resultData.Data.Email,
		JobNumber: resultData.Data.JobNumber,
	}, nil
}

// StartLoginFlow 启动SSO登录流程。
// 它会启动一个临时服务器，打开浏览器，并等待回调。
// 成功后返回 true，否则返回错误。
func StartLoginFlow(baseURL string) (bool, error) {
	// 1. 生成一个唯一的随机字符串用于回调路径
	randomString, err := generateRandomString(8)
	if err != nil {
		return false, fmt.Errorf("无法生成随机字符串: %w", err)
	}

	port := "59090"
	callbackPath := "/code/token/"

	// 2. 构造完整的SSO URL
	fullSSOUrl := fmt.Sprintf("%s?from=%s&callback=%s", baseURL, ProID, randomString)

	// 3. 设置channel用于goroutine之间的通信
	resultChan := make(chan bool)
	errChan := make(chan error, 1)

	// 4. 配置HTTP服务器
	mux := http.NewServeMux()
	server := &http.Server{
		Addr:    ":" + port,
		Handler: mux,
	}

	// 5. 配置回调处理函数
	mux.HandleFunc(callbackPath, func(w http.ResponseWriter, r *http.Request) {
		ticket := r.URL.Query().Get("ticket")
		if ticket == "" {
			err := fmt.Errorf("回调请求中缺少 'ticket' 参数")
			http.Error(w, err.Error(), http.StatusBadRequest)
			errChan <- err
			return
		}

		// 验证ticket
		response, err := checkTicket(ticket)
		if err != nil {
			http.Error(w, fmt.Sprintf("验证ticket失败: %v", err), http.StatusInternalServerError)
			errChan <- fmt.Errorf("验证ticket失败: %w", err)
			return
		}

		if !response.Suc {
			http.Error(w, "SSO验证失败", http.StatusUnauthorized)
			errChan <- fmt.Errorf("SSO验证失败")
			return
		}

		if response.Data == nil || response.Data.UserAuthDTO.AccessToken == "" {
			http.Error(w, "未获取到accessToken", http.StatusInternalServerError)
			errChan <- fmt.Errorf("未获取到accessToken")
			return
		}

		accessToken := response.Data.UserAuthDTO.AccessToken

		// 获取用户信息
		userInfo, err := getUserInfo(accessToken)
		if err != nil {
			http.Error(w, fmt.Sprintf("获取用户信息失败: %v", err), http.StatusInternalServerError)
			errChan <- fmt.Errorf("获取用户信息失败: %w", err)
			return
		}

		// 保存用户信息
		if err := saveUserInfo(userInfo); err != nil {
			http.Error(w, fmt.Sprintf("保存用户信息失败: %v", err), http.StatusInternalServerError)
			errChan <- fmt.Errorf("保存用户信息失败: %w", err)
			return
		}

		// 所有操作成功后，向浏览器返回成功页面
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(LoginSuccess))

		// 在后台关闭服务器
		go func() {
			time.Sleep(500 * time.Millisecond)
			if err := server.Shutdown(context.Background()); err != nil {
				errChan <- err
			}
		}()

		resultChan <- true
	})

	// 6. 在一个单独的goroutine中启动服务器
	go func() {
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			panic(fmt.Errorf("无法启动服务器: %w", err))
		}
	}()

	// 7. 启动浏览器
	if err := openBrowser(fullSSOUrl); err != nil {
		go server.Shutdown(context.Background())
		panic(fmt.Errorf("打开浏览器失败: %w", err))
	}

	// 8. 等待结果、错误或超时
	select {
	case res := <-resultChan:
		return res, nil
	case err := <-errChan:
		panic(err)
	case <-time.After(60 * time.Second): // 设置60秒超时
		go server.Shutdown(context.Background())
		panic(fmt.Errorf("登录超时"))
	}
}

func Login() {
	// SSO系统登录页面的基础URL
	ssoBaseURL := "https://sso.xinfei-inc.cn/login"

	// 启动登录流程
	success, err := StartLoginFlow(ssoBaseURL)
	if err != nil {
		panic(err)
	}
	if !success {
		panic("登录失败")
	}
}
