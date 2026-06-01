package main

import (
	"os"
	"path/filepath"
	"testing"
)

// ==================== Bug #9: config.go 配置加载 ====================

func TestLoadConfigDefaultValuesExtended(t *testing.T) {
	// 测试默认值
	c := config{}
	a := args{}
	applyConfig(&a, c)
	
	if a.port != 22 {
		t.Errorf("default port = %d, want 22", a.port)
	}
	if a.alive != 30 {
		t.Errorf("default alive = %d, want 30", a.alive)
	}
}

func TestLoadConfigOverrideExtended(t *testing.T) {
	// 测试配置覆盖
	c := config{
		DefaultUser:  "admin",
		DefaultPort:  2222,
		DefaultAuth:  "/path/to/key",
		DefaultAlive: 60,
		Verbose:      true,
	}
	
	a := args{}
	applyConfig(&a, c)
	
	if a.user != "admin" {
		t.Errorf("user = %q, want 'admin'", a.user)
	}
	if a.port != 2222 {
		t.Errorf("port = %d, want 2222", a.port)
	}
	if a.auth != "/path/to/key" {
		t.Errorf("auth = %q, want '/path/to/key'", a.auth)
	}
	if a.alive != 60 {
		t.Errorf("alive = %d, want 60", a.alive)
	}
	if !a.verbose {
		t.Error("verbose should be true")
	}
}

func TestLoadConfigArgsPriorityExtended(t *testing.T) {
	// 测试命令行参数优先级
	c := config{
		DefaultUser: "configuser",
		DefaultPort: 2222,
	}
	
	a := args{
		user: "cmduser",
		port: 22,
	}
	applyConfig(&a, c)
	
	if a.user != "cmduser" {
		t.Errorf("user = %q, want 'cmduser' (args priority)", a.user)
	}
	if a.port != 22 {
		t.Errorf("port = %d, want 22 (args priority)", a.port)
	}
}

func TestSaveAndLoadConfigRoundTripExtended(t *testing.T) {
	// 测试保存和加载的往返
	tmpDir := t.TempDir()
	origHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpDir)
	defer os.Setenv("HOME", origHome)
	
	c := config{
		DefaultUser:  "testuser",
		DefaultPort:  3322,
		DefaultAuth:  "/test/key",
		DefaultAlive: 45,
		Verbose:      true,
	}
	
	err := saveConfig(c)
	if err != nil {
		t.Fatalf("saveConfig failed: %v", err)
	}
	
	loaded := loadConfig()
	if loaded.DefaultUser != "testuser" {
		t.Errorf("DefaultUser = %q, want 'testuser'", loaded.DefaultUser)
	}
	if loaded.DefaultPort != 3322 {
		t.Errorf("DefaultPort = %d, want 3322", loaded.DefaultPort)
	}
	if loaded.DefaultAuth != "/test/key" {
		t.Errorf("DefaultAuth = %q, want '/test/key'", loaded.DefaultAuth)
	}
	if loaded.DefaultAlive != 45 {
		t.Errorf("DefaultAlive = %d, want 45", loaded.DefaultAlive)
	}
	if !loaded.Verbose {
		t.Error("Verbose should be true")
	}
}

func TestSaveConfigDirectoryCreationExtended(t *testing.T) {
	// 测试目录创建
	tmpDir := t.TempDir()
	origHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpDir)
	defer os.Setenv("HOME", origHome)
	
	c := config{DefaultUser: "test"}
	err := saveConfig(c)
	if err != nil {
		t.Fatalf("saveConfig failed: %v", err)
	}
	
	// 验证目录已创建
	sshellDir := filepath.Join(tmpDir, ".sshell")
	info, err := os.Stat(sshellDir)
	if err != nil {
		t.Fatalf("~/.sshell/ directory not created: %v", err)
	}
	if !info.IsDir() {
		t.Error("~/.sshell/ should be a directory")
	}
}

func TestSaveConfigFilePermissionsExtended(t *testing.T) {
	// 测试文件权限
	tmpDir := t.TempDir()
	origHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpDir)
	defer os.Setenv("HOME", origHome)
	
	c := config{DefaultUser: "test"}
	err := saveConfig(c)
	if err != nil {
		t.Fatalf("saveConfig failed: %v", err)
	}
	
	configPath := filepath.Join(tmpDir, ".sshell", "config")
	info, err := os.Stat(configPath)
	if err != nil {
		t.Fatalf("config file not found: %v", err)
	}
	
	// 验证权限为 0600
	if info.Mode().Perm() != 0600 {
		t.Errorf("config file permissions = %o, want 0600", info.Mode().Perm())
	}
}
