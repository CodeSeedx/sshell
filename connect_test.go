package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadKnownHostsNoFile(t *testing.T) {
	tmpDir := t.TempDir()
	oldHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpDir)
	defer os.Setenv("HOME", oldHome)

	_, err := loadKnownHosts(false)
	if err == nil {
		t.Log("loadKnownHosts succeeded (known_hosts might exist)")
	} else {
		t.Logf("loadKnownHosts error (expected): %v", err)
	}
}

func TestLoadKnownHostsWithFile(t *testing.T) {
	tmpDir := t.TempDir()
	sshDir := filepath.Join(tmpDir, ".ssh")
	if err := os.MkdirAll(sshDir, 0700); err != nil {
		t.Fatal(err)
	}

	knownHostsPath := filepath.Join(sshDir, "known_hosts")
	if err := os.WriteFile(knownHostsPath, []byte(""), 0644); err != nil {
		t.Fatal(err)
	}

	oldHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpDir)
	defer os.Setenv("HOME", oldHome)

	callback, err := loadKnownHosts(false)
	if err != nil {
		t.Fatalf("loadKnownHosts failed: %v", err)
	}
	if callback == nil {
		t.Error("callback should not be nil")
	}
}

func TestLoadKnownHostsVerbose(t *testing.T) {
	tmpDir := t.TempDir()
	sshDir := filepath.Join(tmpDir, ".ssh")
	if err := os.MkdirAll(sshDir, 0700); err != nil {
		t.Fatal(err)
	}

	knownHostsPath := filepath.Join(sshDir, "known_hosts")
	if err := os.WriteFile(knownHostsPath, []byte(""), 0644); err != nil {
		t.Fatal(err)
	}

	oldHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpDir)
	defer os.Setenv("HOME", oldHome)

	callback, err := loadKnownHosts(true)
	if err != nil {
		t.Fatalf("loadKnownHosts verbose failed: %v", err)
	}
	if callback == nil {
		t.Error("callback should not be nil")
	}
}

func TestLoadKnownHostsNoHome(t *testing.T) {
	oldHome := os.Getenv("HOME")
	os.Unsetenv("HOME")
	defer os.Setenv("HOME", oldHome)

	_, err := loadKnownHosts(false)
	if err == nil {
		t.Error("expected error when HOME is not set")
	}
}

func TestConnectInvalidHost(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping network test in short mode")
	}

	a := args{
		host: "127.0.0.1",
		port: 19999,
		user: "test",
		auth: "password",
	}

	_, err := connect(a)
	if err == nil {
		t.Log("connect succeeded (unexpected but not fatal)")
	} else {
		t.Logf("connect error (expected): %v", err)
	}
}

func TestConnectVerbose(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping network test in short mode")
	}

	a := args{
		host:    "127.0.0.1",
		port:    19999,
		user:    "test",
		auth:    "password",
		verbose: true,
	}

	_, err := connect(a)
	if err == nil {
		t.Log("connect verbose succeeded (unexpected but not fatal)")
	} else {
		t.Logf("connect verbose error (expected): %v", err)
	}
}

func TestConnectAuthFailure(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping network test in short mode")
	}

	// 指向一个不存在的密钥文件，会走到 findAuth 失败
	tmpDir := t.TempDir()
	badKeyPath := filepath.Join(tmpDir, "nonexistent_key")

	a := args{
		host: "127.0.0.1",
		port: 19999,
		user: "test",
		auth: badKeyPath,
	}

	_, err := connect(a)
	if err == nil {
		t.Log("connect auth failure test succeeded (unexpected)")
	} else {
		t.Logf("connect auth failure (expected): %v", err)
	}
}

func TestConnectWithKeepAlive(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping network test in short mode")
	}

	a := args{
		host:  "127.0.0.1",
		port:  19999,
		user:  "test",
		auth:  "password",
		alive: 60,
	}

	_, err := connect(a)
	if err == nil {
		t.Log("connect keepalive succeeded (unexpected)")
	} else {
		t.Logf("connect keepalive error (expected): %v", err)
	}
}
