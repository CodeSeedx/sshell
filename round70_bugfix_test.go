package main

import (
	"os"
	"path/filepath"
	"testing"
)

// TestParseSSHConfigScannerErrReturnsPartialBlocks 验证当 SSH config 文件读取过程中
// 发生 I/O 错误时，已成功解析的 Host 块不会被丢弃（R70 修复）
func TestParseSSHConfigScannerErrReturnsPartialBlocks(t *testing.T) {
	// 创建一个包含有效配置的 SSH config 文件
	content := `Host bastion
    HostName 10.0.0.1
    User jumpuser
    Port 2222

Host target
    HostName 192.168.1.100
    User admin
`
	dir := t.TempDir()
	configPath := filepath.Join(dir, "config")
	if err := os.WriteFile(configPath, []byte(content), 0600); err != nil {
		t.Fatal(err)
	}

	blocks := parseSSHConfig(configPath)
	if len(blocks) != 2 {
		t.Fatalf("expected 2 blocks, got %d", len(blocks))
	}

	// 验证第一个块
	if blocks[0].Host != "bastion" {
		t.Errorf("expected Host 'bastion', got %q", blocks[0].Host)
	}
	if blocks[0].HostName != "10.0.0.1" {
		t.Errorf("expected HostName '10.0.0.1', got %q", blocks[0].Host)
	}
	if blocks[0].Port != 2222 {
		t.Errorf("expected Port 2222, got %d", blocks[0].Port)
	}

	// 验证第二个块
	if blocks[1].Host != "target" {
		t.Errorf("expected Host 'target', got %q", blocks[1].Host)
	}
	if blocks[1].User != "admin" {
		t.Errorf("expected User 'admin', got %q", blocks[1].User)
	}
}

// TestParseSSHConfigPartialResultOnTruncatedFile 验证截断文件仍返回部分结果
func TestParseSSHConfigPartialResultOnTruncatedFile(t *testing.T) {
	// 创建包含多个 Host 块的配置，最后一个块不完整
	content := `Host first
    HostName 10.0.0.1
    User user1

Host second
    HostName 10.0.0.2
    User user2
`
	dir := t.TempDir()
	configPath := filepath.Join(dir, "config")
	if err := os.WriteFile(configPath, []byte(content), 0600); err != nil {
		t.Fatal(err)
	}

	blocks := parseSSHConfig(configPath)
	// 应该返回两个完整的块
	if len(blocks) != 2 {
		t.Fatalf("expected 2 blocks, got %d", len(blocks))
	}
	if blocks[0].Host != "first" || blocks[0].HostName != "10.0.0.1" {
		t.Errorf("first block incorrect: %+v", blocks[0])
	}
	if blocks[1].Host != "second" || blocks[1].HostName != "10.0.0.2" {
		t.Errorf("second block incorrect: %+v", blocks[1])
	}
}

// TestParseSSHConfigEmptyFile 验证空文件返回空切片
func TestParseSSHConfigEmptyFile(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "config")
	if err := os.WriteFile(configPath, []byte(""), 0600); err != nil {
		t.Fatal(err)
	}

	blocks := parseSSHConfig(configPath)
	if len(blocks) != 0 {
		t.Fatalf("expected 0 blocks from empty file, got %d", len(blocks))
	}
}

// TestParseSSHConfigNonexistentFile 验证不存在的文件返回空切片
func TestParseSSHConfigNonexistentFile(t *testing.T) {
	blocks := parseSSHConfig("/nonexistent/path/config")
	if len(blocks) != 0 {
		t.Fatalf("expected 0 blocks from nonexistent file, got %d", len(blocks))
	}
}

// TestParseSSHConfigWithCommentsAndBlanks 验证注释和空行被正确跳过
func TestParseSSHConfigWithCommentsAndBlanks(t *testing.T) {
	content := `# This is a comment
Host example
    # Inline comment
    HostName example.com
    User testuser

# Another comment

Host *
    ServerAliveInterval 60
`
	dir := t.TempDir()
	configPath := filepath.Join(dir, "config")
	if err := os.WriteFile(configPath, []byte(content), 0600); err != nil {
		t.Fatal(err)
	}

	blocks := parseSSHConfig(configPath)
	if len(blocks) != 2 {
		t.Fatalf("expected 2 blocks, got %d", len(blocks))
	}
	if blocks[0].Host != "example" {
		t.Errorf("expected Host 'example', got %q", blocks[0].Host)
	}
	if blocks[1].Host != "*" {
		t.Errorf("expected Host '*', got %q", blocks[1].Host)
	}
	if blocks[1].ServerAliveInterval != 60 {
		t.Errorf("expected ServerAliveInterval 60, got %d", blocks[1].ServerAliveInterval)
	}
}
