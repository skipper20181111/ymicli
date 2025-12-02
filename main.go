package main

import (
	"log/slog"
	"net/http"
	_ "net/http/pprof"
	"os"

	"github.com/charmbracelet/crush/internal/cmd"
	"github.com/charmbracelet/crush/internal/transformer"
	_ "github.com/joho/godotenv/autoload"
)

func main() {
	//CheckAndCreateCrushFile()
	//login.Login()
	go transformer.StartServer()
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
func CheckAndCreateCrushFile() (bool, error) {
	const filename = "CRUSH.md"

	// 使用 os.Stat 检查文件是否存在
	_, err := os.Stat(filename)

	if os.IsNotExist(err) {
		// 文件不存在，尝试创建它
		file, createErr := os.Create(filename)
		if createErr != nil {
			return false, nil
		}
		defer file.Close()
		return true, nil
	} else if err != nil {
		// 发生了其他错误（例如权限问题）
		return false, nil
	}

	// 文件已存在
	return true, nil
}
