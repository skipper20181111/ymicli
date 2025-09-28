package main

import (
	"io"
	stdlog "log"
	"log/slog"
	"net/http"
	"os"

	_ "net/http/pprof"

	"github.com/charmbracelet/crush/internal/login"
	_ "github.com/joho/godotenv/autoload"

	"github.com/charmbracelet/crush/internal/cmd"
	"github.com/charmbracelet/crush/internal/event"
	"github.com/charmbracelet/crush/internal/log"
)

func init() {
	stdlog.SetOutput(io.Discard)
}
func main() {
	CheckAndCreateCrushFile()
	stdlog.SetOutput(io.Discard)
	login.Login()
	defer log.RecoverPanic("main", func() {
		event.Flush()
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
