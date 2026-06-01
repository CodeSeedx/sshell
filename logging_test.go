package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestNewSessionLogger(t *testing.T) {
	dir := t.TempDir()
	logPath := filepath.Join(dir, "test.log")

	logger, err := newSessionLogger(logPath, false)
	if err != nil {
		t.Fatalf("newSessionLogger failed: %v", err)
	}
	defer logger.Close()

	// 验证文件已创建
	if _, err := os.Stat(logPath); os.IsNotExist(err) {
		t.Error("log file was not created")
	}

	// 验证头部已写入
	data, _ := os.ReadFile(logPath)
	if !strings.Contains(string(data), "sshell session log") {
		t.Error("log header not found")
	}
}

func TestSessionLoggerWrapWriter(t *testing.T) {
	dir := t.TempDir()
	logPath := filepath.Join(dir, "test.log")

	logger, err := newSessionLogger(logPath, false)
	if err != nil {
		t.Fatalf("newSessionLogger failed: %v", err)
	}
	defer logger.Close()

	// 用 WrapWriter 同时写入 buffer 和日志
	var buf bytes.Buffer
	w := logger.WrapWriter(&buf)
	w.Write([]byte("hello world"))

	// 验证 buffer 收到数据
	if buf.String() != "hello world" {
		t.Errorf("buffer = %q, want %q", buf.String(), "hello world")
	}

	// 验证日志文件收到数据
	logger.Close()
	data, _ := os.ReadFile(logPath)
	if !strings.Contains(string(data), "hello world") {
		t.Errorf("log file missing data, got %q", string(data))
	}
}

func TestSessionLoggerClose(t *testing.T) {
	dir := t.TempDir()
	logPath := filepath.Join(dir, "test.log")

	logger, err := newSessionLogger(logPath, false)
	if err != nil {
		t.Fatalf("newSessionLogger failed: %v", err)
	}

	if err := logger.Close(); err != nil {
		t.Fatalf("Close failed: %v", err)
	}

	// 验证尾部写入
	data, _ := os.ReadFile(logPath)
	if !strings.Contains(string(data), "ended") {
		t.Error("log footer not found")
	}
}

func TestSessionLoggerDirectory(t *testing.T) {
	dir := t.TempDir()
	logPath := filepath.Join(dir, "subdir", "test.log")

	logger, err := newSessionLogger(logPath, false)
	if err != nil {
		t.Fatalf("newSessionLogger with subdirectory failed: %v", err)
	}
	defer logger.Close()

	if _, err := os.Stat(logPath); os.IsNotExist(err) {
		t.Error("log file was not created in subdirectory")
	}
}
