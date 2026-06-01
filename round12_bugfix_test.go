package main

import (
	"net"
	"os"
	"path/filepath"
	"testing"
)

// ==================== Bug #43: 目标主机 SSH config 应用 ====================

func TestApplySSHConfigTargetHostName(t *testing.T) {
	// 创建临时 SSH config
	tmpDir := t.TempDir()
	sshDir := filepath.Join(tmpDir, ".ssh")
	os.MkdirAll(sshDir, 0700)
	configContent := `Host myserver
    HostName 192.168.1.100
    Port 2222
    User admin
`
	os.WriteFile(filepath.Join(sshDir, "config"), []byte(configContent), 0600)

	origHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpDir)
	defer os.Setenv("HOME", origHome)

	a := args{host: "myserver", port: 22, user: "root", alive: 30}
	a = applySSHConfigTarget(a, "myserver")

	if a.host != "192.168.1.100" {
		t.Errorf("host = %q, want '192.168.1.100'", a.host)
	}
	if a.port != 2222 {
		t.Errorf("port = %d, want 2222", a.port)
	}
}

func TestApplySSHConfigTargetNoMatch(t *testing.T) {
	tmpDir := t.TempDir()
	sshDir := filepath.Join(tmpDir, ".ssh")
	os.MkdirAll(sshDir, 0700)
	configContent := `Host other
    HostName 10.0.0.1
`
	os.WriteFile(filepath.Join(sshDir, "config"), []byte(configContent), 0600)

	origHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpDir)
	defer os.Setenv("HOME", origHome)

	a := args{host: "myserver", port: 22, user: "root", alive: 30}
	result := applySSHConfigTarget(a, "myserver")

	// 不匹配时不应改变 host
	if result.host != "myserver" {
		t.Errorf("host = %q, want 'myserver'", result.host)
	}
}

func TestApplySSHConfigTargetIdentityFile(t *testing.T) {
	tmpDir := t.TempDir()
	sshDir := filepath.Join(tmpDir, ".ssh")
	os.MkdirAll(sshDir, 0700)
	configContent := `Host myserver
    IdentityFile ~/.ssh/myserver_key
`
	os.WriteFile(filepath.Join(sshDir, "config"), []byte(configContent), 0600)

	origHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpDir)
	defer os.Setenv("HOME", origHome)

	a := args{host: "myserver", port: 22, user: "root", alive: 30, auth: ""}
	result := applySSHConfigTarget(a, "myserver")

	if result.auth == "" {
		t.Error("auth should be set from SSH config IdentityFile")
	}
}

func TestApplySSHConfigTargetAuthNotOverridden(t *testing.T) {
	tmpDir := t.TempDir()
	sshDir := filepath.Join(tmpDir, ".ssh")
	os.MkdirAll(sshDir, 0700)
	configContent := `Host myserver
    IdentityFile ~/.ssh/myserver_key
`
	os.WriteFile(filepath.Join(sshDir, "config"), []byte(configContent), 0600)

	origHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpDir)
	defer os.Setenv("HOME", origHome)

	// auth 已通过命令行指定（cliAuth=true），不应被 SSH config 覆盖
	a := args{host: "myserver", port: 22, user: "root", alive: 30, auth: "/my/key", cliAuth: true}
	result := applySSHConfigTarget(a, "myserver")

	if result.auth != "/my/key" {
		t.Errorf("auth = %q, want '/my/key' (command line should not be overridden)", result.auth)
	}
}

// ==================== Bug #46: SSH config Include 相对路径 ====================

func TestSSHConfigIncludeRelativePath(t *testing.T) {
	tmpDir := t.TempDir()
	sshDir := filepath.Join(tmpDir, ".ssh")
	subDir := filepath.Join(sshDir, "config.d")
	os.MkdirAll(subDir, 0700)

	// 主配置文件引用相对路径的 Include
	mainConfig := `Host included-host
    HostName 10.0.0.1

Include config.d/extra
`
	os.WriteFile(filepath.Join(sshDir, "config"), []byte(mainConfig), 0600)

	// Include 的文件
	extraConfig := `Host extra-host
    HostName 10.0.0.2
`
	os.WriteFile(filepath.Join(subDir, "extra"), []byte(extraConfig), 0600)

	origHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpDir)
	defer os.Setenv("HOME", origHome)

	cfg := loadSSHConfig("extra-host")
	if cfg == nil {
		t.Fatal("loadSSHConfig should find extra-host via Include")
	}
	if cfg.HostName != "10.0.0.2" {
		t.Errorf("HostName = %q, want '10.0.0.2'", cfg.HostName)
	}
}

// ==================== Bug #47: JSON 解析错误警告 ====================

func TestLoadConfigInvalidJSONWarning(t *testing.T) {
	dir := t.TempDir()
	origHome := os.Getenv("HOME")
	os.Setenv("HOME", dir)
	defer os.Setenv("HOME", origHome)

	configDirPath := filepath.Join(dir, ".sshell")
	os.MkdirAll(configDirPath, 0700)
	os.WriteFile(filepath.Join(configDirPath, "config"), []byte("not json"), 0600)

	// 应该不会 panic，只是打印警告并返回空配置
	c := loadConfig()
	if c.DefaultUser != "" {
		t.Errorf("DefaultUser should be empty for invalid JSON, got %q", c.DefaultUser)
	}
}

// ==================== Bug #48: default_auth ~ 展开 ====================

func TestApplyConfigAuthTildeExpansion(t *testing.T) {
	dir := t.TempDir()
	origHome := os.Getenv("HOME")
	os.Setenv("HOME", dir)
	defer os.Setenv("HOME", origHome)

	c := config{DefaultAuth: "~/.ssh/id_rsa"}
	a := args{}
	applyConfig(&a, c)

	expected := filepath.Join(dir, ".ssh", "id_rsa")
	if a.auth != expected {
		t.Errorf("auth = %q, want %q", a.auth, expected)
	}
}

func TestApplyConfigAuthAbsolute(t *testing.T) {
	c := config{DefaultAuth: "/absolute/path/key"}
	a := args{}
	applyConfig(&a, c)

	if a.auth != "/absolute/path/key" {
		t.Errorf("auth = %q, want '/absolute/path/key'", a.auth)
	}
}

func TestApplyConfigAuthNotOverridden(t *testing.T) {
	c := config{DefaultAuth: "~/.ssh/id_rsa"}
	a := args{auth: "/my/key"}
	applyConfig(&a, c)

	if a.auth != "/my/key" {
		t.Errorf("auth = %q, want '/my/key' (args should take priority)", a.auth)
	}
}

// ==================== Bug #44: 多主机 SSH config 应用 ====================

func TestMultiHostSSHConfigApplied(t *testing.T) {
	tmpDir := t.TempDir()
	sshDir := filepath.Join(tmpDir, ".ssh")
	os.MkdirAll(sshDir, 0700)
	configContent := `Host server1
    HostName 192.168.1.100
    Port 2222
`
	os.WriteFile(filepath.Join(sshDir, "config"), []byte(configContent), 0600)

	origHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpDir)
	defer os.Setenv("HOME", origHome)

	// loadSSHConfig 应该能解析 server1
	cfg := loadSSHConfig("server1")
	if cfg == nil {
		t.Fatal("loadSSHConfig should find server1")
	}
	if cfg.HostName != "192.168.1.100" {
		t.Errorf("HostName = %q, want '192.168.1.100'", cfg.HostName)
	}
	if cfg.Port != 2222 {
		t.Errorf("Port = %d, want 2222", cfg.Port)
	}
}

// ==================== Bug #45: closeWriter 接口 ====================

func TestCloseWriterInterfaceImplemented(t *testing.T) {
	// 验证 *net.TCPConn 实现了 closeWriter
	var _ closeWriter = (*net.TCPConn)(nil)
	// 验证 *channelConn 实现了 closeWriter
	var _ closeWriter = (*channelConn)(nil)
}
