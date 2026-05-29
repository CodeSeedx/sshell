package main

import (
	"os"
	"path/filepath"
	"testing"
)

// ==================== loadKnownHosts 更多测试 ====================

func TestLoadKnownHostsEmptyFile(t *testing.T) {
	tmpDir := t.TempDir()
	sshDir := filepath.Join(tmpDir, ".ssh")
	os.MkdirAll(sshDir, 0700)

	// 创建空的 known_hosts 文件
	knownHostsPath := filepath.Join(sshDir, "known_hosts")
	os.WriteFile(knownHostsPath, []byte(""), 0644)

	origHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpDir)
	defer os.Setenv("HOME", origHome)

	callback, err := loadKnownHosts(false)
	if err != nil {
		t.Fatalf("loadKnownHosts with empty file failed: %v", err)
	}
	if callback == nil {
		t.Error("callback should not be nil")
	}
}

func TestLoadKnownHostsInvalidFormat(t *testing.T) {
	tmpDir := t.TempDir()
	sshDir := filepath.Join(tmpDir, ".ssh")
	os.MkdirAll(sshDir, 0700)

	// 写入一些无效格式的数据
	knownHostsPath := filepath.Join(sshDir, "known_hosts")
	os.WriteFile(knownHostsPath, []byte("this is not valid known_hosts format\n"), 0644)

	origHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpDir)
	defer os.Setenv("HOME", origHome)

	// knownhosts.New 应该能处理无效格式（只是解析慢一点）
	callback, err := loadKnownHosts(false)
	if err != nil {
		t.Logf("loadKnownHosts with invalid format: %v (acceptable)", err)
	} else if callback == nil {
		t.Error("callback should not be nil")
	}
}

func TestLoadKnownHostsNoSSHDir(t *testing.T) {
	tmpDir := t.TempDir()
	// 不创建 .ssh 目录
	origHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpDir)
	defer os.Setenv("HOME", origHome)

	_, err := loadKnownHosts(false)
	if err == nil {
		t.Error("expected error when .ssh directory doesn't exist")
	}
}

func TestLoadKnownHostsVerboseNoFile(t *testing.T) {
	tmpDir := t.TempDir()
	origHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpDir)
	defer os.Setenv("HOME", origHome)

	_, err := loadKnownHosts(true)
	if err == nil {
		t.Log("loadKnownHosts verbose succeeded (unexpected)")
	} else {
		t.Logf("loadKnownHosts verbose error (expected): %v", err)
	}
}

// ==================== connect 更多测试 ====================

func TestConnectLocalhostRefused(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping network test in short mode")
	}

	a := args{
		host:  "127.0.0.1",
		port:  19998,
		user:  "test",
		auth:  "password",
		alive: 10,
	}

	_, err := connect(a)
	if err == nil {
		t.Log("connect succeeded (unexpected)")
	} else {
		t.Logf("connect error (expected): %v", err)
	}
}

func TestConnectLocalhostVerbose(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping network test in short mode")
	}

	a := args{
		host:    "127.0.0.1",
		port:    19997,
		user:    "test",
		auth:    "password",
		alive:   30,
		verbose: true,
	}

	_, err := connect(a)
	if err == nil {
		t.Log("connect verbose succeeded (unexpected)")
	} else {
		t.Logf("connect verbose error (expected): %v", err)
	}
}

func TestConnectWithKeyAuth(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping network test in short mode")
	}

	keyPath := generateTestEd25519Key(t)
	a := args{
		host:  "127.0.0.1",
		port:  19996,
		user:  "test",
		auth:  keyPath,
		alive: 30,
	}

	_, err := connect(a)
	if err == nil {
		t.Log("connect with key auth succeeded (unexpected)")
	} else {
		t.Logf("connect with key auth error (expected): %v", err)
	}
}

func TestConnectVerboseWithKeyAuth(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping network test in short mode")
	}

	keyPath := generateTestEd25519Key(t)
	a := args{
		host:    "127.0.0.1",
		port:    19995,
		user:    "test",
		auth:    keyPath,
		alive:   30,
		verbose: true,
	}

	_, err := connect(a)
	if err == nil {
		t.Log("connect verbose key auth succeeded (unexpected)")
	} else {
		t.Logf("connect verbose key auth error (expected): %v", err)
	}
}

func TestConnectWithKeepAliveZero(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping network test in short mode")
	}

	a := args{
		host:  "127.0.0.1",
		port:  19994,
		user:  "test",
		auth:  "password",
		alive: 0,
	}

	_, err := connect(a)
	if err == nil {
		t.Log("connect with keepalive=0 succeeded (unexpected)")
	} else {
		t.Logf("connect with keepalive=0 error (expected): %v", err)
	}
}
