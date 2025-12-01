package agent

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/charmbracelet/crush/internal/login"
)

// UsageReportRequest represents the request payload for token usage reporting.
type UsageReportRequest struct {
	UserSn     string `json:"userSn"`
	Token      int64  `json:"token"`
	IP         string `json:"ip"`
	SystemType string `json:"systemType"`
	UserInfoL  string `json:"userinfo"` // lowercase userinfo
	UserInfo   string `json:"userInfo"` // camelCase userInfo
}

// UsageInfo contains detailed information about the usage report.
type UsageInfo struct {
	UserID       string `json:"userId"`
	UserName     string `json:"userName"`
	FullName     string `json:"fullName"`
	Mobile       string `json:"mobile"`
	Email        string `json:"email"`
	JobNumber    string `json:"jobNumber"`
	HardwareHash string `json:"hardwareHash"`
	Timestamp    string `json:"timestamp"`
}

// UsageReporter handles reporting token usage to external service.
type UsageReporter struct {
	client   *http.Client
	endpoint string
	enabled  bool
	userInfo *login.UserInfo
}

// NewUsageReporter creates a new usage reporter.
// Configuration is hardcoded for now and can be moved to config later.
func NewUsageReporter() *UsageReporter {
	// Hardcoded configuration - can be moved to environment variables later.
	endpoint := "https://qa1-ailaunchercore.testxinfei.cn/api/v1/token/use/save"

	// Disable by default - set CRUSH_USAGE_REPORT_ENABLED=true to enable.
	enabled := true

	// Load user info from .crush/user_info file.
	userInfo := loadUserInfo()

	return &UsageReporter{
		client: &http.Client{
			Timeout: 10 * time.Second,
		},
		endpoint: endpoint,
		enabled:  enabled,
		userInfo: userInfo,
	}
}

// ReportUsage reports token usage to the external service.
func (r *UsageReporter) ReportUsage(modelName string, tokens int64) {
	if !r.enabled {
		return
	}

	// Build usage info from cached user info.
	userSn := ""
	var usageInfo UsageInfo
	if r.userInfo != nil {
		userSn = r.userInfo.UserID
		usageInfo = UsageInfo{
			UserID:       r.userInfo.UserID,
			UserName:     r.userInfo.UserName,
			FullName:     r.userInfo.FullName,
			Mobile:       r.userInfo.Mobile,
			Email:        r.userInfo.Email,
			JobNumber:    r.userInfo.JobNumber,
			HardwareHash: login.GetHardwareHash(),
			Timestamp:    time.Now().UTC().Format(time.RFC3339Nano),
		}
	} else {
		usageInfo = UsageInfo{
			HardwareHash: login.GetHardwareHash(),
			Timestamp:    time.Now().UTC().Format(time.RFC3339Nano),
		}
	}

	userInfoJSON, err := json.Marshal(usageInfo)
	if err != nil {
		slog.Error("Failed to marshal usage info", "error", err)
		return
	}

	userInfoStr := string(userInfoJSON)

	request := UsageReportRequest{
		UserSn:     userSn,
		Token:      tokens,
		IP:         getLocalIP(),
		SystemType: getSystemType(),
		UserInfoL:  userInfoStr,
		UserInfo:   userInfoStr,
	}
	// Report in background to avoid blocking.
	go r.sendReport(request)
}

func (r *UsageReporter) sendReport(request UsageReportRequest) {
	defer func() {
		if r := recover(); r != nil {
			// Silently recover from any panic - reporting is non-critical.
		}
	}()

	// Create a context with timeout for the HTTP request.
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	payload, err := json.Marshal(request)
	if err != nil {
		return
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, r.endpoint, bytes.NewReader(payload))
	if err != nil {
		return
	}

	req.Header.Set("Content-Type", "application/json")

	resp, err := r.client.Do(req)
	if err != nil {
		return
	}
	defer resp.Body.Close()
}

// getLocalIP returns the local IP address.
func getLocalIP() string {
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		return ""
	}

	for _, addr := range addrs {
		if ipNet, ok := addr.(*net.IPNet); ok && !ipNet.IP.IsLoopback() {
			if ipNet.IP.To4() != nil {
				return ipNet.IP.String()
			}
		}
	}
	return ""
}

// getSystemType returns the system type string like "macOS 23.6.0" or "Windows 10.0.19041".
func getSystemType() string {
	switch runtime.GOOS {
	case "darwin":
		// Get macOS kernel version using uname -r
		out, err := exec.Command("uname", "-r").Output()
		if err != nil {
			return fmt.Sprintf("macOS %s", runtime.GOARCH)
		}
		version := strings.TrimSpace(string(out))
		return fmt.Sprintf("macOS %s", version)
	case "windows":
		// Get Windows version
		out, err := exec.Command("cmd", "/c", "ver").Output()
		if err != nil {
			return fmt.Sprintf("Windows %s", runtime.GOARCH)
		}
		version := strings.TrimSpace(string(out))
		return version
	case "linux":
		// Get Linux kernel version
		out, err := exec.Command("uname", "-r").Output()
		if err != nil {
			return fmt.Sprintf("Linux %s", runtime.GOARCH)
		}
		version := strings.TrimSpace(string(out))
		return fmt.Sprintf("Linux %s", version)
	default:
		return fmt.Sprintf("%s %s", runtime.GOOS, runtime.GOARCH)
	}
}

// loadUserInfo loads user info from .crush/user_info file.
func loadUserInfo() *login.UserInfo {
	cwd, err := os.Getwd()
	if err != nil {
		return nil
	}

	filePath := filepath.Join(cwd, ".crush", "user_info")
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil
	}

	var userInfo login.UserInfo
	if err := json.Unmarshal(data, &userInfo); err != nil {
		return nil
	}

	return &userInfo
}
