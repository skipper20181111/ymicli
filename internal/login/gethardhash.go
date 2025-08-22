package login

// 导入需要使用的标准库包。
import (
	"crypto/sha256" // 导入SHA256哈希算法相关的库。
	"encoding/hex"  // 导入用于十六进制编码和解码的库。
	"fmt"           // 导入格式化I/O库，这里主要用于在panic中格式化错误信息。
	"os/exec"       // 导入执行外部命令的库。
	"runtime"       // 导入与Go运行时环境交互的库，用于获取操作系统类型。
	"strings"       // 导入字符串处理相关的库。
)

//================================================================
// 1. 抽象层 (Abstraction Layer)
//================================================================

// UniqueIdentifierProvider 定义了一个行为接口，用于获取平台唯一的标识符。
type UniqueIdentifierProvider interface {
	// GetIdentifier 是接口中唯一的方法，它负责获取并返回一个字符串类型的唯一标识符。
	// 如果在获取过程中发生不可恢复的错误，该方法的实现应该直接 panic。
	GetIdentifier() (identifier string)
}

//================================================================
// 2. 实现层 (Implementation Layer)
//================================================================

// --- Linux 实现 ---

// linuxProvider 结构体是为Linux系统实现的 UniqueIdentifierProvider。
type linuxProvider struct{} // 定义一个空结构体，因为它不需要任何字段，只用于实现接口方法。

// GetIdentifier 在Linux上获取CPU的Processor ID作为唯一标识符。
func (p linuxProvider) GetIdentifier() string { // 为 linuxProvider 实现 GetIdentifier 方法。
	// 使用 "dmidecode" 命令获取处理器的ID。
	cpuSerial, err := runCommand("dmidecode", "-s", "processor-id") // 调用辅助函数执行外部命令。
	// 检查命令执行是否出错，或者返回的结果是否为空。
	if err != nil || cpuSerial == "" { // 如果条件为真，说明获取失败。
		// 如果获取失败，则调用 panic 抛出致命错误，中断程序执行。
		panic(fmt.Errorf("在Linux上获取CPU Processor ID失败: %w", err))
	} // 结束 if 条件判断。
	// 如果成功获取，则返回获取到的CPU序列号。
	return cpuSerial
} // GetIdentifier 方法结束。

// --- Windows 实现 ---

// windowsProvider 结构体是为Windows系统实现的 UniqueIdentifierProvider。
type windowsProvider struct{} // 定义一个空结构体，用于在Windows上实现接口。

// GetIdentifier 在Windows上获取CPU的Processor ID作为唯一标识符。
func (p windowsProvider) GetIdentifier() string { // 为 windowsProvider 实现 GetIdentifier 方法。
	// 使用 "wmic" 命令获取CPU的Processor ID。
	cpuOutput, err := runCommand("wmic", "cpu", "get", "processorid") // 执行命令并获取其输出。
	// 检查命令执行过程中是否发生错误。
	if err != nil { // 如果 err 不是 nil，则表示有错误发生。
		// 如果执行wmic命令失败，则 panic 并报告错误。
		panic(fmt.Errorf("在Windows上执行wmic获取CPU Processor ID失败: %w", err))
	} // 结束 if 条件判断。
	// 调用辅助函数解析wmic命令的输出，提取出有效的ID。
	cpuSerial := parseWmicOutput(cpuOutput) // 将命令输出传入解析函数。
	// 检查解析出的ID是否为空。
	if cpuSerial == "" { // 如果序列号为空字符串。
		// 如果未能从wmic的输出中解析出ID，则 panic。
		panic("在Windows上未能解析出CPU Processor ID")
	} // 结束 if 条件判断。
	// 返回成功解析出的CPU序列号。
	return cpuSerial
} // GetIdentifier 方法结束。

// --- macOS (Darwin) 实现 ---

// darwinProvider 结构体是为macOS系统实现的 UniqueIdentifierProvider。
type darwinProvider struct{} // 定义一个空结构体，用于在macOS上实现接口。

// GetIdentifier 在macOS上获取主板序列号作为唯一标识符。
func (p darwinProvider) GetIdentifier() string { // 为 darwinProvider 实现 GetIdentifier 方法。
	// 使用 "ioreg" 命令获取IO平台设备信息。
	output, err := runCommand("ioreg", "-rd1", "-c", "IOPlatformExpertDevice") // 执行命令并捕获输出和错误。
	// 检查命令执行过程中是否出错。
	if err != nil { // 如果 err 不是 nil，说明命令执行失败。
		// 如果执行ioreg失败，则 panic。
		panic(fmt.Errorf("在macOS上执行ioreg失败: %w", err))
	} // 结束 if 条件判断。

	// 遍历命令输出的每一行，以查找包含序列号的行。
	for _, line := range strings.Split(output, "\n") { // 按换行符分割输出并开始循环。
		// 检查当前行是否包含 "IOPlatformSerialNumber" 这个关键字。
		if strings.Contains(line, "IOPlatformSerialNumber") { // 如果包含关键字。
			// 按 "=" 分割这一行，以分离键和值。
			parts := strings.Split(line, "=") // 将行分割成字符串切片。
			// 确保分割后正好有两个部分（一个键和一个值）。
			if len(parts) == 2 { // 如果切片长度为2。
				// 清理值部分，去除两端的空格和双引号。
				boardSerial := strings.Trim(parts[1], ` "`) // 获取并清理序列号字符串。
				// 检查清理后的序列号是否不为空。
				if boardSerial != "" { // 如果序列号有效。
					// 如果找到了有效的序列号，则立即返回它。
					return boardSerial
				} // 结束内层 if 判断。
			} // 结束外层 if 判断。
		} // 结束对行内容的检查。
	} // 结束 for 循环。

	// 如果循环结束仍未返回，说明没有找到主板序列号，此时 panic。
	panic("在macOS上未能找到或解析出主板序列号")
} // GetIdentifier 方法结束。

//================================================================
// 3. 工厂函数 (Factory Function)
//================================================================

// NewIdentifierProvider 是一个工厂函数，它根据当前操作系统返回一个合适的 UniqueIdentifierProvider 实例。
func NewIdentifierProvider() UniqueIdentifierProvider { // 定义工厂函数，返回一个接口类型。
	// 使用 runtime.GOOS 获取当前运行的操作系统名称。
	switch runtime.GOOS { // 开始一个 switch 语句来判断操作系统。
	case "linux": // 如果是 "linux"。
		// 返回一个 linuxProvider 的实例。
		return linuxProvider{}
	case "windows": // 如果是 "windows"。
		// 返回一个 windowsProvider 的实例。
		return windowsProvider{}
	case "darwin": // 如果是 "darwin" (macOS)。
		// 返回一个 darwinProvider 的实例。
		return darwinProvider{}
	default: // 如果是任何其他不支持的操作系统。
		// 调用 panic，因为程序无法在当前系统上运行。
		panic(fmt.Sprintf("不支持的操作系统: %s", runtime.GOOS))
	} // 结束 switch 语句。
} // NewIdentifierProvider 函数结束。

//================================================================
// 4. 核心业务逻辑 (Core Business Logic)
//================================================================

// GetHardwareHash 是公开的核心函数，负责获取硬件标识并计算其SHA256哈希值。
func GetHardwareHash() string { // 定义获取硬件哈希的函数，返回一个字符串。
	// 1. 通过工厂函数获取当前平台的标识符提供者。
	provider := NewIdentifierProvider() // 调用工厂函数，获取 provider 实例。

	// 2. 使用提供者获取唯一的硬件标识符。
	identifier := provider.GetIdentifier() // 调用实例的 GetIdentifier 方法。

	// 3. 对获取到的标识符进行SHA256哈希计算。
	hasher := sha256.New()                      // 创建一个新的SHA256哈希器实例。
	hasher.Write([]byte(identifier))            // 将标识符字符串转换为字节切片并写入哈希器。
	hashBytes := hasher.Sum(nil)                // 计算哈希值并返回字节切片。
	hashString := hex.EncodeToString(hashBytes) // 将哈希值的字节切片编码为十六进制字符串。

	// 返回最终计算出的十六进制哈希字符串。
	return hashString
} // GetHardwareHash 函数结束。

//================================================================
// 5. 辅助函数与主程序入口 (Helpers & Main Entry)
//================================================================

// runCommand 执行一个外部命令并返回其标准输出。
func runCommand(name string, arg ...string) (string, error) { // 定义一个可变参数的辅助函数。
	cmd := exec.Command(name, arg...)             // 创建一个表示外部命令的对象。
	output, err := cmd.Output()                   // 执行命令并捕获其标准输出。
	return strings.TrimSpace(string(output)), err // 返回清理了首尾空白的输出字符串和执行错误。
} // runCommand 函数结束。

// parseWmicOutput 解析 Windows WMIC 命令的输出，提取有效的数据行。
func parseWmicOutput(output string) string { // 定义一个解析wmic输出的函数。
	// 将输出字符串按换行符分割成多行。
	lines := strings.Split(strings.TrimSpace(output), "\n") // 首先清理首尾空白，然后分割。
	// 检查输出是否多于一行（第一行通常是标题）。
	if len(lines) > 1 { // 如果行数大于1。
		// 返回最后一行的数据，并清理其两端的空白字符。
		return strings.TrimSpace(lines[len(lines)-1])
	} // 结束 if 判断。
	// 如果行数不大于1，则返回空字符串。
	return ""
} // parseWmicOutput 函数结束。
