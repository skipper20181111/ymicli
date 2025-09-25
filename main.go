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
