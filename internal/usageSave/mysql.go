package usageSave

import (
	"errors"
	"os"
	"path/filepath"
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

type RawClientConnector struct {
	conn     *client.Conn
	userInfo string
}

// NewRawClientConnector 建立裸连接（不经过 database/sql）
func NewRawClientConnector() (*RawClientConnector, error) {
	addr := "qa1-mysql.testxinfei.cn:3308"
	user := "ailaunchcore_qa"
	pass := "xvt8++mN35YwOiLwL2nF"
	dbname := "ailaunchcore"

	conn, err := client.Connect(addr, user, pass, dbname)
	if err != nil {
		return nil, err
	}

	// 可选：设置读写超时
	conn.ReadTimeout = 5 * time.Second
	conn.WriteTimeout = 5 * time.Second

	// Ping 验证连通性
	if err := conn.Ping(); err != nil {
		conn.Close()
		return nil, err
	}

	userInfo, err := loadUserInfo()
	if err != nil {
		userInfo = ""
	}

	return &RawClientConnector{
		conn:     conn,
		userInfo: userInfo,
	}, nil
}

func loadUserInfo() (string, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return "", err
	}

	filePath := filepath.Join(cwd, "user_info")
	data, err := os.ReadFile(filePath)
	if err != nil {
		return "", err
	}

	return string(data), nil
}

// InsertTokenUse 插入数据（注意：将 nil 映射为 SQL NULL）
func (c *RawClientConnector) InsertTokenUse(userSN string, token int64, ip, systemType *string) error {
	if c == nil || c.conn == nil {
		return errors.New("ErrNotConnected")
	}

	query := `INSERT INTO token_use (user_sn, token, create_time, ip, system_type, user_info) VALUES (?, ?, NOW(), ?, ?, ?)`

	var ipVal interface{}
	if ip != nil {
		ipVal = *ip
	}
	var sysVal interface{}
	if systemType != nil {
		sysVal = *systemType
	}

	var userInfoVal interface{}
	if c.userInfo != "" {
		userInfoVal = c.userInfo
	}

	res, err := c.conn.Execute(query, userSN, token, ipVal, sysVal, userInfoVal)
	if err != nil {
		return err
	}
	res.Close()
	return nil
}

func (c *RawClientConnector) InsertTokenUseWithHostInfo(userSN string, token int64) error {
	ip, systemType := GetHostInfo() // 你已有的函数
	var ipPtr, sysPtr *string
	if ip != "" {
		ipPtr = &ip
	}
	if systemType != "" {
		sysPtr = &systemType
	}
	return c.InsertTokenUse(userSN, token, ipPtr, sysPtr)
}

func (c *RawClientConnector) Close() error {
	if c != nil && c.conn != nil {
		return c.conn.Close()
	}
	return nil
}
