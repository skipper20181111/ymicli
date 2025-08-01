package utilFunction

import (
	"context"
	"crypto/md5"
	"encoding/hex"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

// RemoveAllWhitespace 使用正则表达式将字符串中的所有空白字符替换为空。
func RemoveAllWhitespace(s string) string {
	// \s 是正则表达式中匹配任何空白字符的简写，包括空格、tab、换行符等。
	// + 表示匹配一个或多个空白字符的连续序列。
	// compile 编译正则表达式，提高效率（特别是当函数被多次调用时）。
	re := regexp.MustCompile(`\s+`)

	// ReplaceAllString 将所有匹配到的空白字符序列替换为指定的字符串（这里是空字符串）。
	return re.ReplaceAllString(s, "")
}
func BoolPtr(b bool) *bool {
	return &b
}

// PathExists 检查给定路径是否存在
func PathExists(path string) bool {
	_, err := os.Stat(path) // os.Stat 获取文件或目录的信息
	if err == nil {
		return true // err 为 nil，表示路径存在
	}
	if os.IsNotExist(err) {
		return false // err 是文件或目录不存在的错误
	}
	return false // 其他错误（例如权限问题），表示无法确定路径是否存在，将错误返回
}
func ExtractCodeBlocks(input string) map[string][]string {
	// 正则匹配 ```lang\n内容\n```，非贪婪匹配内容，(?s)表示开启单行模式匹配换行符
	re := regexp.MustCompile("(?s)```([a-zA-Z0-9]+)\\s+(.*?)\\s*```")
	matches := re.FindAllStringSubmatch(input, -1)

	result := make(map[string][]string)
	for _, match := range matches {
		lang := match[1]
		content := match[2]
		result[lang] = append(result[lang], content)
	}
	return result
}
func ExtractOnde(OreString, Type string) string {
	blocks := ExtractCodeBlocks(OreString)
	if Lists, ok := blocks[Type]; ok {
		if len(Lists) > 0 {
			return Lists[0]
		}
	}
	return ""
}

// DownloadMarkdownFile 从给定的 URL 下载 Markdown 文件并返回其内容字符串。
// 它会检查响应的 Content-Type 是否为文本类型，以确保下载的是预期的文件。
func DownloadMarkdownFile(url string) (string, error) {
	// 1. 发送 HTTP GET 请求
	resp, err := http.Get(url)
	if err != nil {
		return "", fmt.Errorf("发送 HTTP 请求失败: %w", err)
	}
	defer resp.Body.Close() // 确保在函数退出时关闭响应体

	// 2. 检查 HTTP 状态码
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("HTTP 请求失败，状态码: %d %s", resp.StatusCode, resp.Status)
	}

	// 3. (可选但推荐) 检查 Content-Type
	// 这有助于确保你下载的是文本文件，而不是二进制文件或其他意料之外的内容。
	// Markdown 文件通常会返回 text/markdown, text/plain, text/html (如果服务器将其视为网页) 等。
	contentType := resp.Header.Get("Content-Type")
	if !strings.HasPrefix(contentType, "text/") && !strings.Contains(contentType, "markdown") {
		// 宽松检查，如果不是text/开头且不含markdown，则警告
	}

	// 4. 读取响应体
	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("读取响应体失败: %w", err)
	}

	// 5. 将字节切片转换为字符串并返回
	return string(bodyBytes), nil
}
func GetContextWithTimeOut(TimeOutTime time.Duration) context.Context {
	timeout, _ := context.WithTimeout(context.Background(), TimeOutTime)
	return timeout
}
func CalculateMD5(input string) string {
	// 创建一个新的 MD5 哈希对象
	hasher := md5.New()

	// 将输入字符串转换为字节切片并写入哈希对象
	// Write 方法永远不会返回错误，所以我们忽略第二个返回值
	hasher.Write([]byte(input))

	// 计算最终的哈希值
	hashSum := hasher.Sum(nil)

	// 将哈希值（字节切片）编码为十六进制字符串并返回
	return hex.EncodeToString(hashSum)
}

// GetFilePaths 根据给定的路径返回文件列表。
// 如果路径指向一个目录，它会返回该目录下所有文件的相对路径列表，并忽略 .git 目录下的文件。
// 如果路径指向一个文件，它会返回一个包含该文件名本身的单元素列表。
// 如果路径不存在，则返回错误。
func GetFilePaths(path string) ([]string, error) {
	// 使用 os.Stat 检查路径是否存在并获取其信息
	info, err := os.Stat(path)
	if err != nil {
		// 如果错误表明路径不存在
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("路径不存在: %s", path)
		}
		// 如果是其他类型的错误，返回错误信息
		return nil, fmt.Errorf("无法获取路径信息: %w", err)
	}

	// 如果路径指向的是一个文件
	if !info.IsDir() {
		// 直接返回一个包含该文件名的切片
		return []string{filepath.Base(path)}, nil
	}

	// 如果路径指向的是一个目录，则开始遍历
	var fileList []string
	// 使用 filepath.WalkDir 递归遍历目录下的所有文件和子目录
	err = filepath.WalkDir(path, func(p string, d fs.DirEntry, err error) error {
		// 在遍历过程中，如果遇到任何错误，直接返回该错误
		if err != nil {
			return err
		}

		// 检查当前路径是否是 .git 目录
		// 如果是 .git 目录，则跳过整个子树的遍历
		if d.IsDir() && (strings.Contains(p, ".git") || strings.Contains(p, ".idea")) {
			return filepath.SkipDir
		}

		// 如果当前项不是目录（即是文件）
		if !d.IsDir() {
			// 使用 filepath.Rel 获取文件相对于给定路径的相对路径
			relPath, err := filepath.Rel(path, p)
			if err != nil {
				return err
			}
			// 将相对路径添加到列表中
			fileList = append(fileList, relPath)
		}
		return nil
	})

	// 如果遍历过程最终返回了错误，则将该错误封装后返回
	if err != nil {
		return nil, fmt.Errorf("无法遍历目录 %s: %w", path, err)
	}

	// 返回最终的文件列表
	return fileList, nil
}
func ReadFileContent(filePath string) string {
	// os.ReadFile 会读取整个文件，并将其内容作为字节切片返回。
	content, err := os.ReadFile(filePath)
	if err != nil {
		// 如果有错误发生，直接返回空字符串
		return ""
	}

	// 将字节切片转换为字符串并返回
	return string(content)
}
