package verifier

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
)

// ValidatePatch 校验修复后的代码是否能编译通过
// 返回值：是否成功，错误信息
func ValidatePatch(fullCode []byte) (bool, string) {
	// 1. 创建临时分析目录
	tmpDir, err := os.MkdirTemp("", "golint_verify_*")
	if err != nil {
		return false, "创建临时目录失败"
	}
	defer os.RemoveAll(tmpDir)

	tmpFile := filepath.Join(tmpDir, "main.go")
	if err := os.WriteFile(tmpFile, fullCode, 0644); err != nil {
		return false, "写入临时文件失败"
	}

	// 2. 执行 go build
	// 我们只进行语法和类型检查，不真正生成二进制文件，这样最快
	cmd := exec.Command("go", "build", "-o", filepath.Join(tmpDir, "null"), tmpFile)

	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		// 返回编译器的 stderr 输出
		return false, stderr.String()
	}

	return true, ""
}
