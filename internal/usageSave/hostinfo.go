package usageSave

import (
	"net"
	"runtime"
)

// GetHostInfo 获取主机的IP地址和操作系统信息。
// 如果获取IP地址失败，则返回空字符串。
// 即使发生 panic，此函数也会安全地恢复并返回空字符串。
func GetHostInfo() (string, string) {
	// 兜底：使用 defer 和 recover 捕获所有 panic，
	// 并在发生 panic 时安全地返回空字符串。
	defer func() {
		if r := recover(); r != nil {
			// 如果发生 panic，此处的返回语句会覆盖外部的返回值。
			// 我们不需要在这里做任何其他事情，因为函数末尾的
			// return 语句已经处理了返回空字符串的情况。
		}
	}()

	// 获取IP地址
	addrs, err := net.InterfaceAddrs()
	var ipAddr string
	if err == nil {
		for _, address := range addrs {
			// 检查地址是否为IP地址
			if ipnet, ok := address.(*net.IPNet); ok && !ipnet.IP.IsLoopback() {
				// 检查是否为IPv4地址
				if ipnet.IP.To4() != nil {
					ipAddr = ipnet.IP.String()
					break // 找到第一个非回环的IPv4地址就出
				}
			}
		}
	}

	// 获取系统版本信息
	osInfo := runtime.GOOS + " " + runtime.GOARCH

	return ipAddr, osInfo
}
