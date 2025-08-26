package utilFunction

import (
	_ "embed"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
)

//go:embed crush.json
var crushJson []byte

// SaveToCrushJSON 将一个 byte 数组转换为字符串，并存入 crush.json 文件。
// 文件路径会根据操作系统自动确定：
// - Windows: %LOCALAPPDATA%/crush/crush.json
// - Linux & macOS: $HOME/.local/share/crush/crush.json
// 如果文件已存在，函数将直接返回，不会执行任何操作。
func SaveToCrushJSON() error {
	data := crushJson
	var crushDir string
	var err error

	// 1. 根据操作系统确定基础目录
	switch runtime.GOOS {
	case "windows":
		// 获取 %LOCALAPPDATA% 环境变量
		localAppData := os.Getenv("LOCALAPPDATA")
		crushDir = filepath.Join(localAppData, "crush")
	case "linux", "darwin": // darwin 是 macOS
		// 获取 $HOME 环境变量
		homeDir, _ := os.UserHomeDir()
		crushDir = filepath.Join(homeDir, ".local", "share", "crush")
	default:
		return fmt.Errorf("不支持的操作系统: %s", runtime.GOOS)
	}

	// 2. 创建目录（如果不存在）
	// os.MkdirAll 会创建所有必需的父目录，如果目录已存在则不会返回错误。
	if err = os.MkdirAll(crushDir, 0755); err != nil {
		return fmt.Errorf("创建目录失败 '%s': %w", crushDir, err)
	}

	// 3. 构造完整的文件路径
	filePath := filepath.Join(crushDir, "crush.json")

	// 4. 检查文件是否存在
	// 使用 os.Stat 检查文件。如果文件不存在，它会返回一个错误。
	if _, err = os.Stat(filePath); err == nil {
		// 文件已存在，根据要求忽略操作
		return nil
	} else if !os.IsNotExist(err) {
		// 如果返回的错误不是 "not exist"，说明发生了其他问题（例如权限问题）
		return fmt.Errorf("检查文件 '%s' 时发生错误: %w", filePath, err)
	}

	// 5. 将 byte 数组写入文件
	// 文件不存在，所以我们创建并写入它。
	// os.WriteFile 会处理文件的创建、写入和关闭。
	content := string(data)
	err = os.WriteFile(filePath, []byte(content), 0644)
	if err != nil {
		return fmt.Errorf("写入文件 '%s' 失败: %w", filePath, err)
	}

	return nil
}
