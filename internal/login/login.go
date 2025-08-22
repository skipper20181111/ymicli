package login

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"net/http"
	"os/exec"
	"runtime"
	"time"
)

const (
	LoginSuccess = `
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
	callbackPath := fmt.Sprintf("/code/token/%s", randomString)

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
			panic(err)
		}

		// 向浏览器返回成功信息，让用户知道可以关闭页面
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(LoginSuccess))

		// 在一个新的goroutine中处理登录成功逻辑
		go func() {
			// 在此goroutine结束时，确保服务器被关闭
			defer func() {
				// 等待一小段时间确保HTTP响应已发送
				time.Sleep(500 * time.Millisecond)
				if err := server.Shutdown(context.Background()); err != nil {
					panic(err)
				}
			}()

			// 只要获取到ticket就认为登录成功
			resultChan <- true
		}()
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
