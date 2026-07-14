package tools

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// QuickSyntaxCheck runs a fast syntax check on a file after writing.
// Returns an error message string if issues found, empty string if OK.
// This is lightweight — full verification still happens via the verify service.
func QuickSyntaxCheck(filePath string) string {
	ext := strings.ToLower(filepath.Ext(filePath))

	switch ext {
	case ".go":
		return checkGoSyntax(filePath)
	case ".py":
		return checkPythonSyntax(filePath)
	case ".ts", ".tsx":
		return checkTypeScriptSyntax(filePath)
	case ".json":
		return checkJSONSyntax(filePath)
	default:
		return "" // no quick check available for this language
	}
}

func checkGoSyntax(filePath string) string {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// go fmt first
	cmd := exec.CommandContext(ctx, "go", "fmt", filePath)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	_ = cmd.Run() // ignore error from fmt (it returns 1 if file was formatted)

	// go vet for fast syntax/type check
	cmd2 := exec.CommandContext(ctx, "go", "vet", filePath)
	cmd2.Stderr = &stderr
	if err := cmd2.Run(); err != nil {
		return fmt.Sprintf("⚠️ 语法检查发现问题: %s", strings.TrimSpace(stderr.String()))
	}
	return ""
}

func checkPythonSyntax(filePath string) string {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, "python", "-m", "py_compile", filePath)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return fmt.Sprintf("⚠️ Python 语法错误: %s", strings.TrimSpace(stderr.String()))
	}
	return ""
}

func checkTypeScriptSyntax(filePath string) string {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, "npx", "tsc", "--noEmit", filePath)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		errStr := strings.TrimSpace(stderr.String())
		if len(errStr) > 300 {
			errStr = errStr[:300] + "..."
		}
		return fmt.Sprintf("⚠️ TypeScript 类型/语法错误: %s", errStr)
	}
	return ""
}

func checkJSONSyntax(filePath string) string {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, "python", "-c",
		fmt.Sprintf("import json; json.load(open(%q))", filePath))
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return fmt.Sprintf("⚠️ JSON 格式错误: %s", strings.TrimSpace(stderr.String()))
	}
	return ""
}
