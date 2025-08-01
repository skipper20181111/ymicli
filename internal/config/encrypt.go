package config

// 导入所有需要用到的标准库包。
import (
	"bytes"         // 导入 bytes 包，用于高效处理字节切片，如此处的填充操作。
	"crypto/aes"    // 导入 AES（高级加密标准）算法实现。
	"crypto/cipher" // 导入密码学相关的接口和模式，如此处的 CBC 模式。
	"crypto/rand"   // 导入加密安全的伪随机数生成器，用于生成初始化向量（IV）。
	"encoding/hex"  // 导入 hex 包，用于实现字节切片和十六进制字符串之间的转换。
	"errors"        // 导入 errors 包，用于创建自定义的错误信息。
	"fmt"           // 导入 fmt 包，用于格式化输出，方便在控制台打印信息。
	"io"            // 导入 io 包，io.ReadFull 用于从数据流中准确读取指定长度的字节。
)

const (
	// 修正了密钥长度。AES要求密钥必须是16字节（AES-128）、24字节（AES-192）或32字节（AES-256）。
	// 原始的 "ABABABAB" (8字节) 会导致程序错误。这里修正为16字节。
	cryptKey = "8272e6366be6233f120b340760d5e956"
)

// =================================================================
// 新增的转换辅助函数
// =================================================================

// BytesToHex 函数：将一个字节切片（[]byte）转换为其十六进制表示的字符串。
// data: 输入的原始字节切片。
func BytesToHex(data []byte) string {
	// 调用标准库 `hex.EncodeToString`，它会高效地完成转换工作。
	return hex.EncodeToString(data)
} // 函数结束

// HexToBytes 函数：将一个十六进制字符串转换为其代表的字节切片（[]byte）。
// hexStr: 输入的十六进制格式的字符串。
// 由于输入的字符串可能不是有效的十六进制，此函数还返回一个 error。
func HexToBytes(hexStr string) ([]byte, error) {
	// 调用标准库 `hex.DecodeString`，它会尝试解码。
	data, err := hex.DecodeString(hexStr)
	// 将解码得到的数据（字节切片）和可能发生的错误返回。
	return data, err
} // 函数结束

// =================================================================
// 核心加解密逻辑（已重构）
// =================================================================

// pkcs7Padding 函数：使用 PKCS#7 方案对数据进行填充。
func pkcs7Padding(data []byte, blockSize int) []byte {
	// ... 此函数内部逻辑不变 ...
	padding := blockSize - len(data)%blockSize
	padtext := bytes.Repeat([]byte{byte(padding)}, padding)
	return append(data, padtext...)
}

// pkcs7UnPadding 函数：从已填充的数据中移除 PKCS#7 填充。
func pkcs7UnPadding(data []byte) ([]byte, error) {
	// ... 此函数内部逻辑不变 ...
	length := len(data)
	if length == 0 {
		return nil, errors.New("unpadding error: empty data")
	}
	unpadding := int(data[length-1])
	if unpadding > length {
		return nil, errors.New("unpadding error: invalid padding size")
	}
	return data[:(length - unpadding)], nil
}
func EasyEncrypt(cipherTextHex string) string {
	decrypt, _ := AesCbcEncrypt(cipherTextHex)
	return decrypt
}

// AesCbcEncrypt 函数：执行 AES CBC 加密，输入和输出均为十六进制字符串。
func AesCbcEncrypt(plainTextHex string) (string, error) {
	// --- 入口处理：【修改点】使用 HexToBytes 辅助函数 ---
	// 首先，将输入的十六进制明文字符串解码为字节切片。
	plainTextBytes := []byte(plainTextHex)

	// --- 核心加密逻辑（保持不变） ---
	block, err := aes.NewCipher([]byte(cryptKey))
	if err != nil {
		return "", err
	}
	blockSize := block.BlockSize()
	paddedPlainText := pkcs7Padding(plainTextBytes, blockSize)
	cipherTextBytes := make([]byte, blockSize+len(paddedPlainText))
	iv := cipherTextBytes[:blockSize]
	if _, err := io.ReadFull(rand.Reader, iv); err != nil {
		return "", err
	}
	mode := cipher.NewCBCEncrypter(block, iv)
	mode.CryptBlocks(cipherTextBytes[blockSize:], paddedPlainText)

	// --- 出口处理：【修改点】使用 BytesToHex 辅助函数 ---
	// 将最终的字节密文（IV + 加密数据）编码为十六进制字符串并返回。
	return BytesToHex(cipherTextBytes), nil
}
func EasyDecrypt(cipherTextHex string) string {
	decrypt, _ := AesCbcDecrypt(cipherTextHex)
	return decrypt
}

// AesCbcDecrypt 函数：执行 AES CBC 解密，输入和输出均为十六进制字符串。
func AesCbcDecrypt(cipherTextHex string) (string, error) {
	// --- 入口处理：【修改点】使用 HexToBytes 辅助函数 ---
	// 首先，将输入的十六进制密文字符串解码为字节切片。
	cipherTextBytes, err := HexToBytes(cipherTextHex)
	// 检查解码过程中是否发生错误。
	if err != nil {
		return "", fmt.Errorf("ciphertext hex decoding failed: %w", err)
	}

	// --- 核心解密逻辑（保持不变） ---
	block, err := aes.NewCipher([]byte(cryptKey))
	if err != nil {
		return "", err
	}
	blockSize := block.BlockSize()
	if len(cipherTextBytes) < blockSize {
		return "", errors.New("decryption error: ciphertext too short")
	}
	iv := cipherTextBytes[:blockSize]
	actualCipherText := cipherTextBytes[blockSize:]
	if len(actualCipherText)%blockSize != 0 {
		return "", errors.New("decryption error: ciphertext is not a multiple of the block size")
	}
	mode := cipher.NewCBCDecrypter(block, iv)
	mode.CryptBlocks(actualCipherText, actualCipherText)
	plainTextBytes, err := pkcs7UnPadding(actualCipherText)
	if err != nil {
		return "", err
	}

	// --- 出口处理：【修改点】使用 BytesToHex 辅助函数 ---
	// 将恢复的原始明文字节切片编码为十六进制字符串并返回。
	return string(plainTextBytes), nil
}
