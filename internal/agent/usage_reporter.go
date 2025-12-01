package agent

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"runtime"
	"time"
)

// UsageReportRequest represents the request payload for token usage reporting.
type UsageReportRequest struct {
	UserSn     string `json:"userSn"`
	Token      int64  `json:"token"`
	IP         string `json:"ip"`
	SystemType string `json:"systemType"`
	UserInfo   string `json:"userInfo"`
}

// UsageInfo contains detailed information about the token usage.
type UsageInfo struct {
	InputTokens  int64  `json:"inputTokens"`
	OutputTokens int64  `json:"outputTokens"`
	Model        string `json:"model"`
	Provider     string `json:"provider"`
	UserName     string `json:"userName"`
	Department   string `json:"department"`
	HardwareHash string `json:"hardwareHash"`
	Timestamp    string `json:"timestamp"`
}

// UsageReporter handles reporting token usage to external service.
type UsageReporter struct {
	client   *http.Client
	endpoint string
	userSn   string
	ip       string
	enabled  bool
}

// NewUsageReporter creates a new usage reporter.
// Configuration is hardcoded for now and can be moved to config later.
func NewUsageReporter() *UsageReporter {
	// Hardcoded configuration - can be moved to environment variables later.
	endpoint := "https://qa1-ailaunchercore.testxinfei.cn/api/v1/token/use/save"
	userSn := "1234567"
	ip := "192.168.8.23333"

	// Disable by default - set CRUSH_USAGE_REPORT_ENABLED=true to enable.
	enabled := true

	return &UsageReporter{
		client: &http.Client{
			Timeout: 10 * time.Second,
		},
		endpoint: endpoint,
		userSn:   userSn,
		ip:       ip,
		enabled:  enabled,
	}
}

// ReportUsage reports token usage to the external service.
func (r *UsageReporter) ReportUsage(modelName string, tokens int64) {
	if !r.enabled {
		return
	}

	// Build usage info with hardcoded user details.
	userName := "我是你爸爸"

	usageInfo := UsageInfo{
		InputTokens:  0,
		OutputTokens: 0,
		Model:        modelName,
		Provider:     "",
		UserName:     userName,
		Department:   userName, // Using username as department for now.
		HardwareHash: getHardwareHash(),
		Timestamp:    time.Now().UTC().Format(time.RFC3339),
	}

	userInfoJSON, err := json.Marshal(usageInfo)
	if err != nil {
		slog.Error("Failed to marshal usage info", "error", err)
		return
	}

	totalTokens := tokens

	request := UsageReportRequest{
		UserSn:     r.userSn,
		Token:      totalTokens,
		IP:         r.ip,
		SystemType: fmt.Sprintf("%s %s", runtime.GOOS, getOSVersion()),
		UserInfo:   string(userInfoJSON),
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

// getHardwareHash returns a hash of hardware identifiers (placeholder implementation).
func getHardwareHash() string {
	// TODO: Implement actual hardware hash generation.
	hostname, _ := os.Hostname()
	return fmt.Sprintf("%x", hostname)
}

// getOSVersion returns the OS version string.
func getOSVersion() string {
	// On macOS, this might return something like "darwin"
	// For more detailed version info, platform-specific code would be needed.
	return runtime.GOARCH
}
