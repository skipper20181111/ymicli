package config

import (
	"errors"
	"fmt"
	"net"
	"runtime"
	"time"

	"github.com/go-mysql-org/go-mysql/client"
)

// TokenUse 结构体（按需保留字段）
type TokenUse struct {
	ID         int64
	UserSN     string
	Token      int64
	CreateTime time.Time
	IP         *string
	SystemType *string
}

// ClaudeCodeConfig 对应 claude_code_config 表结构
type ClaudeCodeConfig struct {
	ID                         int64     `json:"id"`
	LogEnabled                 bool      `json:"log_enabled"`
	LogLevel                   string    `json:"log_level"`
	ClaudePath                 string    `json:"claude_path"`
	Host                       string    `json:"host"`
	Port                       int       `json:"port"`
	APIKey                     string    `json:"api_key"`
	APITimeoutMs               string    `json:"api_timeout_ms"`
	ProxyURL                   string    `json:"proxy_url"`
	Transformers               *string   `json:"transformers"` // JSON类型
	Providers                  *string   `json:"providers"`    // JSON类型
	StatusLine                 *string   `json:"status_line"`  // JSON类型
	RouterDefault              *string   `json:"router_default"`
	RouterBackground           *string   `json:"router_background"`
	RouterThink                *string   `json:"router_think"`
	RouterLongContext          *string   `json:"router_long_context"`
	RouterLongContextThreshold int       `json:"router_long_context_threshold"`
	RouterWebSearch            *string   `json:"router_web_search"`
	RouterImage                *string   `json:"router_image"`
	CustomRouterPath           string    `json:"custom_router_path"`
	Status                     bool      `json:"status"`
	CreatedTime                time.Time `json:"created_time"`
	UpdatedTime                time.Time `json:"updated_time"`
}

type DBConnector struct {
	conn *client.Conn
}

// NewDBConnector 建立数据库连接
func NewDBConnector() (*DBConnector, error) {
	addr := "qa1-mysql.testxinfei.cn:3308"
	user := "ailaunchcore_qa"
	pass := "xvt8++mN35YwOiLwL2nF"
	dbname := "ailaunchcore"

	conn, err := client.Connect(addr, user, pass, dbname)
	if err != nil {
		return nil, err
	}

	// 设置读写超时
	conn.ReadTimeout = 5 * time.Second
	conn.WriteTimeout = 5 * time.Second

	// Ping 验证连通性
	if err := conn.Ping(); err != nil {
		conn.Close()
		return nil, err
	}

	return &DBConnector{conn: conn}, nil
}

// InsertTokenUse 插入数据（注意：将 nil 映射为 SQL NULL）
func (c *DBConnector) InsertTokenUse(userSN string, token int64, ip, systemType *string) error {
	if c == nil || c.conn == nil {
		return errors.New("ErrNotConnected")
	}

	query := `INSERT INTO token_use (user_sn, token, create_time, ip, system_type) VALUES (?, ?, NOW(), ?, ?)`

	var ipVal interface{}
	if ip != nil {
		ipVal = *ip
	}
	var sysVal interface{}
	if systemType != nil {
		sysVal = *systemType
	}

	res, err := c.conn.Execute(query, userSN, token, ipVal, sysVal)
	if err != nil {
		return err
	}
	res.Close()
	return nil
}

// InsertTokenUseWithHostInfo 使用主机信息插入Token使用记录
func (c *DBConnector) InsertTokenUseWithHostInfo(userSN string, token int64) error {
	ip, systemType := getHostInfo()
	var ipPtr, sysPtr *string
	if ip != "" {
		ipPtr = &ip
	}
	if systemType != "" {
		sysPtr = &systemType
	}
	return c.InsertTokenUse(userSN, token, ipPtr, sysPtr)
}

// QueryClaudeConfigByPath 根据 claude_path 查询配置（只返回 claude_path 和 providers）
func (c *DBConnector) QueryClaudeConfigByPath(claudePath string) (*ClaudeCodeConfig, error) {
	if c == nil || c.conn == nil {
		return nil, errors.New("ErrNotConnected")
	}

	query := `SELECT claude_path, providers FROM claude_code_config WHERE claude_path = ? LIMIT 1`

	result, err := c.conn.Execute(query, claudePath)
	if err != nil {
		return nil, fmt.Errorf("query failed: %w", err)
	}
	defer result.Close()

	if result.RowNumber() == 0 {
		return nil, fmt.Errorf("no config found for claude_path: %s", claudePath)
	}

	// 使用Resultset访问第一行数据（row index = 0）
	config := &ClaudeCodeConfig{}
	rowIdx := 0

	// 只解析 claude_path 和 providers
	claudePathVal, _ := result.GetString(rowIdx, 0)
	config.ClaudePath = claudePathVal

	// providers 可能为NULL
	if isNull, _ := result.IsNull(rowIdx, 1); !isNull {
		if providers, err := result.GetString(rowIdx, 1); err == nil {
			config.Providers = &providers
		}
	}

	return config, nil
}

// Close 关闭连接
func (c *DBConnector) Close() error {
	if c != nil && c.conn != nil {
		return c.conn.Close()
	}
	return nil
}

// getHostInfo 获取主机的IP地址和操作系统信息
func getHostInfo() (string, string) {
	defer func() {
		if r := recover(); r != nil {
			// 捕获panic，安全返回空字符串
		}
	}()

	// 获取IP地址
	addrs, err := net.InterfaceAddrs()
	var ipAddr string
	if err == nil {
		for _, address := range addrs {
			if ipnet, ok := address.(*net.IPNet); ok && !ipnet.IP.IsLoopback() {
				if ipnet.IP.To4() != nil {
					ipAddr = ipnet.IP.String()
					break
				}
			}
		}
	}

	// 获取系统版本信息
	osInfo := runtime.GOOS + " " + runtime.GOARCH

	return ipAddr, osInfo
}
