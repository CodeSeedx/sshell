package main

import (
	"os"
	"path/filepath"
	"testing"
)

// ==================== configDir / configFile 测试 ====================

func TestConfigDir(t *testing.T) {
	origHome := os.Getenv("HOME")
	defer os.Setenv("HOME", origHome)

	t.Run("正常情况", func(t *testing.T) {
		tmpDir := t.TempDir()
		os.Setenv("HOME", tmpDir)
		dir, err := configDir()
		if err != nil {
			t.Fatalf("configDir failed: %v", err)
		}
		expected := filepath.Join(tmpDir, ".sshell")
		if dir != expected {
			t.Errorf("configDir = %q, want %q", dir, expected)
		}
	})

	t.Run("HOME未设置", func(t *testing.T) {
		os.Unsetenv("HOME")
		_, err := configDir()
		if err == nil {
			t.Error("expected error when HOME is not set")
		}
	})
}

func TestConfigFile(t *testing.T) {
	origHome := os.Getenv("HOME")
	defer os.Setenv("HOME", origHome)

	t.Run("正常情况", func(t *testing.T) {
		tmpDir := t.TempDir()
		os.Setenv("HOME", tmpDir)
		path, err := configFile()
		if err != nil {
			t.Fatalf("configFile failed: %v", err)
		}
		expected := filepath.Join(tmpDir, ".sshell", "config")
		if path != expected {
			t.Errorf("configFile = %q, want %q", path, expected)
		}
	})

	t.Run("HOME未设置", func(t *testing.T) {
		os.Unsetenv("HOME")
		_, err := configFile()
		if err == nil {
			t.Error("expected error when HOME is not set")
		}
	})
}

// ==================== loadConfig 深入测试 ====================

func TestLoadConfigMissingFile(t *testing.T) {
	origHome := os.Getenv("HOME")
	defer os.Setenv("HOME", origHome)

	tmpDir := t.TempDir()
	os.Setenv("HOME", tmpDir)

	// 配置文件不存在时应返回空配置
	c := loadConfig()
	if c.DefaultUser != "" {
		t.Errorf("DefaultUser = %q, want empty", c.DefaultUser)
	}
	if c.DefaultPort != 0 {
		t.Errorf("DefaultPort = %d, want 0", c.DefaultPort)
	}
	if c.Verbose {
		t.Error("Verbose should be false")
	}
}

func TestLoadConfigInvalidJSON(t *testing.T) {
	origHome := os.Getenv("HOME")
	defer os.Setenv("HOME", origHome)

	tmpDir := t.TempDir()
	sshDir := filepath.Join(tmpDir, ".sshell")
	os.MkdirAll(sshDir, 0700)
	os.Setenv("HOME", tmpDir)

	// 写入无效 JSON
	configPath := filepath.Join(sshDir, "config")
	os.WriteFile(configPath, []byte("not valid json{{{"), 0600)

	c := loadConfig()
	// JSON 解析失败，应返回空配置
	if c.DefaultUser != "" {
		t.Errorf("DefaultUser = %q, want empty", c.DefaultUser)
	}
}

func TestLoadConfigPartialFields(t *testing.T) {
	origHome := os.Getenv("HOME")
	defer os.Setenv("HOME", origHome)

	tmpDir := t.TempDir()
	sshDir := filepath.Join(tmpDir, ".sshell")
	os.MkdirAll(sshDir, 0700)
	os.Setenv("HOME", tmpDir)

	// 只写部分字段
	configPath := filepath.Join(sshDir, "config")
	os.WriteFile(configPath, []byte(`{"default_user": "partial"}`), 0600)

	c := loadConfig()
	if c.DefaultUser != "partial" {
		t.Errorf("DefaultUser = %q, want %q", c.DefaultUser, "partial")
	}
	if c.DefaultPort != 0 {
		t.Errorf("DefaultPort = %d, want 0", c.DefaultPort)
	}
}

func TestLoadConfigFullFields(t *testing.T) {
	origHome := os.Getenv("HOME")
	defer os.Setenv("HOME", origHome)

	tmpDir := t.TempDir()
	sshDir := filepath.Join(tmpDir, ".sshell")
	os.MkdirAll(sshDir, 0700)
	os.Setenv("HOME", tmpDir)

	configPath := filepath.Join(sshDir, "config")
	jsonData := `{
  "default_user": "admin",
  "default_port": 3322,
  "default_auth": "/path/to/key",
  "default_alive": 120,
  "verbose": true
}`
	os.WriteFile(configPath, []byte(jsonData), 0600)

	c := loadConfig()
	if c.DefaultUser != "admin" {
		t.Errorf("DefaultUser = %q, want %q", c.DefaultUser, "admin")
	}
	if c.DefaultPort != 3322 {
		t.Errorf("DefaultPort = %d, want %d", c.DefaultPort, 3322)
	}
	if c.DefaultAuth != "/path/to/key" {
		t.Errorf("DefaultAuth = %q, want %q", c.DefaultAuth, "/path/to/key")
	}
	if c.DefaultAlive != 120 {
		t.Errorf("DefaultAlive = %d, want %d", c.DefaultAlive, 120)
	}
	if !c.Verbose {
		t.Error("Verbose should be true")
	}
}

// ==================== saveConfig 深入测试 ====================

func TestSaveAndLoadConfig(t *testing.T) {
	origHome := os.Getenv("HOME")
	defer os.Setenv("HOME", origHome)

	tmpDir := t.TempDir()
	os.Setenv("HOME", tmpDir)

	c := config{
		DefaultUser:  "saveduser",
		DefaultPort:  9922,
		DefaultAuth:  "/saved/key",
		DefaultAlive: 45,
		Verbose:      false,
	}

	err := saveConfig(c)
	if err != nil {
		t.Fatalf("saveConfig failed: %v", err)
	}

	// 验证文件已创建
	path, _ := configFile()
	if _, err := os.Stat(path); os.IsNotExist(err) {
		t.Fatal("config file was not created")
	}

	// 验证能正确加载
	loaded := loadConfig()
	if loaded.DefaultUser != "saveduser" {
		t.Errorf("loaded DefaultUser = %q, want %q", loaded.DefaultUser, "saveduser")
	}
	if loaded.DefaultPort != 9922 {
		t.Errorf("loaded DefaultPort = %d, want %d", loaded.DefaultPort, 9922)
	}
	if loaded.DefaultAuth != "/saved/key" {
		t.Errorf("loaded DefaultAuth = %q, want %q", loaded.DefaultAuth, "/saved/key")
	}
	if loaded.DefaultAlive != 45 {
		t.Errorf("loaded DefaultAlive = %d, want %d", loaded.DefaultAlive, 45)
	}
	if loaded.Verbose {
		t.Error("loaded Verbose should be false")
	}
}

func TestSaveConfigOverwrite(t *testing.T) {
	origHome := os.Getenv("HOME")
	defer os.Setenv("HOME", origHome)

	tmpDir := t.TempDir()
	os.Setenv("HOME", tmpDir)

	// 第一次保存
	c1 := config{DefaultUser: "user1", DefaultPort: 1111}
	if err := saveConfig(c1); err != nil {
		t.Fatalf("saveConfig c1 failed: %v", err)
	}

	// 第二次保存（覆盖）
	c2 := config{DefaultUser: "user2", DefaultPort: 2222}
	if err := saveConfig(c2); err != nil {
		t.Fatalf("saveConfig c2 failed: %v", err)
	}

	loaded := loadConfig()
	if loaded.DefaultUser != "user2" {
		t.Errorf("DefaultUser = %q, want %q (should be overwritten)", loaded.DefaultUser, "user2")
	}
	if loaded.DefaultPort != 2222 {
		t.Errorf("DefaultPort = %d, want %d (should be overwritten)", loaded.DefaultPort, 2222)
	}
}

// ==================== applyConfig 更多边界测试 ====================

func TestApplyConfigVerboseInteraction(t *testing.T) {
	c := config{Verbose: true}

	// 命令行没设 verbose，配置文件设了 → 应用
	a := args{verbose: false}
	applyConfig(&a, c)
	if !a.verbose {
		t.Error("verbose should be applied from config when not set in args")
	}

	// 命令行设了 verbose，配置文件没设 → 保持
	a2 := args{verbose: true}
	c2 := config{Verbose: false}
	applyConfig(&a2, c2)
	if !a2.verbose {
		t.Error("verbose should remain true when set in args")
	}

	// 都设了 → 命令行优先
	a3 := args{verbose: true}
	c3 := config{Verbose: false}
	applyConfig(&a3, c3)
	if !a3.verbose {
		t.Error("verbose should remain true (args priority)")
	}
}

func TestApplyConfigDefaultValues(t *testing.T) {
	// 空配置文件（全零值）→ 默认值应被设置
	c := config{}
	a := args{}
	applyConfig(&a, c)

	if a.port != 22 {
		t.Errorf("port = %d, want 22 (default)", a.port)
	}
	if a.alive != 30 {
		t.Errorf("alive = %d, want 30 (default)", a.alive)
	}
}

func TestApplyConfigZeroPortOverride(t *testing.T) {
	// 配置文件设了 port=0（用户显式设为0），不应覆盖命令行
	c := config{DefaultPort: 0}
	a := args{port: 22}
	applyConfig(&a, c)
	if a.port != 22 {
		t.Errorf("port = %d, want 22 (zero config should not override)", a.port)
	}
}
