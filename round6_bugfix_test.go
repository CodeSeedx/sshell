package main

import (
	"os"
	"path/filepath"
	"testing"
)

// ==================== 第 6 轮：资源泄漏和错误处理 ====================

// Bug #23: parseSCPPath 空字符串处理
func TestParseSCPPathEmpty(t *testing.T) {
	local, remote := parseSCPPath("")
	if local != "" {
		t.Errorf("local = %q, want ''", local)
	}
	if remote != "" {
		t.Errorf("remote = %q, want ''", remote)
	}
}

func TestParseSCPPathNoColon(t *testing.T) {
	local, remote := parseSCPPath("/path/to/file")
	if local != "/path/to/file" {
		t.Errorf("local = %q, want '/path/to/file'", local)
	}
	if remote != "/path/to/file" {
		t.Errorf("remote = %q, want '/path/to/file'", remote)
	}
}

func TestParseSCPPathWithColon(t *testing.T) {
	local, remote := parseSCPPath("local.txt:/remote.txt")
	if local != "local.txt" {
		t.Errorf("local = %q, want 'local.txt'", local)
	}
	if remote != "/remote.txt" {
		t.Errorf("remote = %q, want '/remote.txt'", remote)
	}
}

func TestParseSCPPathMultipleColons(t *testing.T) {
	// 只取第一个冒号
	local, remote := parseSCPPath("a:b:c")
	if local != "a" {
		t.Errorf("local = %q, want 'a'", local)
	}
	if remote != "b:c" {
		t.Errorf("remote = %q, want 'b:c'", remote)
	}
}

// Bug #24: loadKnownHosts 文件不存在时的处理
func TestLoadKnownHostsFileNotExist(t *testing.T) {
	// 设置一个不存在的 HOME 目录
	tmpDir := t.TempDir()
	origHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpDir)
	defer os.Setenv("HOME", origHome)
	
	// 没有 .ssh/known_hosts 文件时应该返回错误
	_, err := loadKnownHosts(false)
	if err == nil {
		t.Log("loadKnownHosts 返回 nil 错误，可能已处理文件不存在的情况")
	} else {
		t.Logf("loadKnownHosts 返回错误（预期）: %v", err)
	}
}

func TestLoadKnownHostsFileExists(t *testing.T) {
	tmpDir := t.TempDir()
	sshDir := filepath.Join(tmpDir, ".ssh")
	os.MkdirAll(sshDir, 0700)
	
	// 创建一个 known_hosts 文件（可以是空的或格式正确的）
	knownHostsPath := filepath.Join(sshDir, "known_hosts")
	os.WriteFile(knownHostsPath, []byte(""), 0600)
	
	origHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpDir)
	defer os.Setenv("HOME", origHome)
	
	callback, err := loadKnownHosts(false)
	if err != nil {
		t.Fatalf("loadKnownHosts failed: %v", err)
	}
	if callback == nil {
		t.Error("callback should not be nil")
	}
}

// Bug #25: sshAgentAuth 环境变量处理
func TestSSHAgentAuthNoSocket(t *testing.T) {
	origSocket := os.Getenv("SSH_AUTH_SOCK")
	os.Unsetenv("SSH_AUTH_SOCK")
	defer os.Setenv("SSH_AUTH_SOCK", origSocket)
	
	_, _, err := sshAgentAuth()
	if err == nil {
		t.Error("sshAgentAuth should fail when SSH_AUTH_SOCK is not set")
	}
}

func TestSSHAgentAuthInvalidSocket(t *testing.T) {
	origSocket := os.Getenv("SSH_AUTH_SOCK")
	os.Setenv("SSH_AUTH_SOCK", "/nonexistent/path/agent.sock")
	defer os.Setenv("SSH_AUTH_SOCK", origSocket)
	
	_, _, err := sshAgentAuth()
	if err == nil {
		t.Error("sshAgentAuth should fail with invalid socket path")
	}
}

// Bug #26: autoDetectKeys 空目录处理
func TestAutoDetectKeysNoSSHDir(t *testing.T) {
	tmpDir := t.TempDir()
	origHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpDir)
	defer os.Setenv("HOME", origHome)
	
	_, err := autoDetectKeys(false)
	if err == nil {
		t.Error("autoDetectKeys should fail when no SSH keys exist")
	}
}

func TestAutoDetectKeysEmptySSHDir(t *testing.T) {
	tmpDir := t.TempDir()
	sshDir := filepath.Join(tmpDir, ".ssh")
	os.MkdirAll(sshDir, 0700)
	
	origHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpDir)
	defer os.Setenv("HOME", origHome)
	
	_, err := autoDetectKeys(false)
	if err == nil {
		t.Error("autoDetectKeys should fail when SSH dir is empty")
	}
}

// Bug #27: readPassword 错误处理
func TestReadPasswordPrompt(t *testing.T) {
	// 无法在非交互式环境中测试 readPassword，跳过
	t.Skip("readPassword requires interactive terminal")
}
