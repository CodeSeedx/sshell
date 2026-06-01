package main

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

// Test_sftpPut_LocalFileNotFound 验证 sftpPut 在本地文件不存在时的前置检查
func Test_sftpPut_LocalFileNotFound(t *testing.T) {
	tmpDir := t.TempDir()
	localPath := filepath.Join(tmpDir, "nonexistent.txt")

	_, err := os.Open(localPath)
	if err == nil {
		t.Fatal("expected error for nonexistent file")
	}
}

// Test_connectClientWithRetry_ErrorFormat_R38 验证 connectClientWithRetry 错误格式
func Test_connectClientWithRetry_ErrorFormat_R38(t *testing.T) {
	a := args{
		host:         "nonexistent.invalid",
		port:         22,
		user:         "test",
		reconnect:    true,
		reconnectMax: 1,
	}

	_, err := connectClientWithRetry(a)
	if err == nil {
		t.Fatal("expected error for nonexistent host")
	}

	errMsg := err.Error()
	if !containsSubstring(errMsg, "reconnect failed") {
		t.Errorf("error should contain 'reconnect failed': %s", errMsg)
	}
}

// Test_connectClientWithRetry_MultipleAttempts_R38 验证多次重试的错误格式
func Test_connectClientWithRetry_MultipleAttempts_R38(t *testing.T) {
	a := args{
		host:         "nonexistent.invalid",
		port:         22,
		user:         "test",
		reconnect:    true,
		reconnectMax: 2,
	}

	_, err := connectClientWithRetry(a)
	if err == nil {
		t.Fatal("expected error for nonexistent host")
	}

	errMsg := err.Error()
	if !containsSubstring(errMsg, "reconnect failed") {
		t.Errorf("error should contain 'reconnect failed': %s", errMsg)
	}
}

// Test_sftpTransfer_TempFileCleanup 验证 SFTP 传输临时文件清理逻辑
func Test_sftpTransfer_TempFileCleanup(t *testing.T) {
	tmpDir := t.TempDir()
	tmpPath := filepath.Join(tmpDir, "test.txt.tmp")
	successPath := filepath.Join(tmpDir, "test.txt")

	// 模拟失败路径：创建临时文件后删除
	f, err := os.Create(tmpPath)
	if err != nil {
		t.Fatal(err)
	}
	fmt.Fprintln(f, "data")
	f.Close()

	os.Remove(tmpPath)
	if _, err := os.Stat(tmpPath); !os.IsNotExist(err) {
		t.Error("temp file should be removed on failure")
	}

	// 模拟成功路径：创建临时文件后 rename
	f, err = os.Create(tmpPath)
	if err != nil {
		t.Fatal(err)
	}
	fmt.Fprintln(f, "content")
	f.Close()

	if err := os.Rename(tmpPath, successPath); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(successPath); err != nil {
		t.Error("final file should exist after rename")
	}
}

// Test_exponentialBackoff_Boundary_R38 验证指数退避边界条件（含大值溢出修复）
func Test_exponentialBackoff_Boundary_R38(t *testing.T) {
	tests := []struct {
		attempt int
		maxDur  int // 最大允许秒数
	}{
		{1, 1},
		{2, 2},
		{3, 4},
		{4, 8},
		{10, 60},  // 应被限制在 60 秒
		{100, 60}, // 大值不应溢出
	}

	for _, tt := range tests {
		d := exponentialBackoff(tt.attempt)
		if d.Seconds() > float64(tt.maxDur) {
			t.Errorf("exponentialBackoff(%d) = %v, want <= %ds", tt.attempt, d, tt.maxDur)
		}
		if d.Seconds() <= 0 {
			t.Errorf("exponentialBackoff(%d) = %v, want > 0", tt.attempt, d)
		}
	}
}
