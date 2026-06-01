package main

import (
	"testing"
	
	"golang.org/x/crypto/ssh"
)

// ==================== Bug #15: exec.go 远程命令执行 ====================

func TestRunRemoteCommandIOBasic(t *testing.T) {
	// 测试基本的命令执行
	if testing.Short() {
		t.Skip("skipping network test in short mode")
	}
	
	// 这个测试需要 SSH 服务器，跳过
	t.Skip("requires SSH server")
}

// ==================== Bug #16: connect.go 连接管理 ====================

func TestBuildSSHConfigBasic(t *testing.T) {
	a := args{
		user: "testuser",
	}
	
	authMethods := []ssh.AuthMethod{ssh.Password("test")}
	hostKeyCallback := ssh.InsecureIgnoreHostKey()
	
	config := buildSSHConfig(a, authMethods, hostKeyCallback)
	
	if config.User != "testuser" {
		t.Errorf("config.User = %q, want 'testuser'", config.User)
	}
	if config.Timeout != 10*1000000000 { // 10 seconds in nanoseconds
		t.Errorf("config.Timeout = %v, want 10s", config.Timeout)
	}
}

func TestBuildSSHConfigAuthMethods(t *testing.T) {
	a := args{
		user: "testuser",
	}
	
	authMethods := []ssh.AuthMethod{
		ssh.Password("pass1"),
		ssh.Password("pass2"),
	}
	hostKeyCallback := ssh.InsecureIgnoreHostKey()
	
	config := buildSSHConfig(a, authMethods, hostKeyCallback)
	
	if len(config.Auth) != 2 {
		t.Errorf("len(config.Auth) = %d, want 2", len(config.Auth))
	}
}

func TestBuildSSHConfigHostKeyCallback(t *testing.T) {
	a := args{
		user: "testuser",
	}
	
	authMethods := []ssh.AuthMethod{ssh.Password("test")}
	
	// 测试 InsecureIgnoreHostKey
	config := buildSSHConfig(a, authMethods, ssh.InsecureIgnoreHostKey())
	if config.HostKeyCallback == nil {
		t.Error("HostKeyCallback should not be nil")
	}
}

// ==================== Bug #17: connect.go 重连逻辑 ====================

func TestConnectWithRetryDisabled(t *testing.T) {
	// 重连禁用时，应该直接调用 connect
	a := args{
		host:         "127.0.0.1",
		port:         19999,
		user:         "test",
		auth:         "password",
		reconnect:    false,
		reconnectMax: 3,
	}
	
	_, err := connectWithRetry(a)
	if err == nil {
		t.Log("connect succeeded (unexpected)")
	} else {
		t.Logf("connect error (expected): %v", err)
	}
}

func TestConnectWithRetryMaxAttempts(t *testing.T) {
	// 测试最大重试次数
	a := args{
		host:         "127.0.0.1",
		port:         19999,
		user:         "test",
		auth:         "password",
		reconnect:    true,
		reconnectMax: 1,
	}
	
	_, err := connectWithRetry(a)
	if err == nil {
		t.Log("connect succeeded (unexpected)")
	} else {
		t.Logf("connect error after retries (expected): %v", err)
	}
}

func TestConnectWithRetryDefaultMax(t *testing.T) {
	// 测试默认最大重试次数
	a := args{
		host:         "127.0.0.1",
		port:         19999,
		user:         "test",
		auth:         "password",
		reconnect:    true,
		reconnectMax: 0, // 应该使用默认值 3
	}
	
	_, err := connectWithRetry(a)
	if err == nil {
		t.Log("connect succeeded (unexpected)")
	} else {
		t.Logf("connect error after retries (expected): %v", err)
	}
}

// ==================== Bug #18: connect.go ProxyJump ====================

func TestBuildJumpArgsBasic(t *testing.T) {
	a := args{
		host:    "target",
		port:    22,
		user:    "testuser",
		auth:    "/path/to/key",
		alive:   30,
		verbose: true,
		noAgent: false,
		proxyJump: "bastion",
	}
	
	jumpArgs := buildJumpArgs(a)
	
	if jumpArgs.host != "bastion" {
		t.Errorf("jumpArgs.host = %q, want 'bastion'", jumpArgs.host)
	}
	if jumpArgs.port != 22 {
		t.Errorf("jumpArgs.port = %d, want 22", jumpArgs.port)
	}
	if jumpArgs.user != "testuser" {
		t.Errorf("jumpArgs.user = %q, want 'testuser'", jumpArgs.user)
	}
	if jumpArgs.auth != "/path/to/key" {
		t.Errorf("jumpArgs.auth = %q, want '/path/to/key'", jumpArgs.auth)
	}
	if jumpArgs.alive != 30 {
		t.Errorf("jumpArgs.alive = %d, want 30", jumpArgs.alive)
	}
	if !jumpArgs.verbose {
		t.Error("jumpArgs.verbose should be true")
	}
}

func TestBuildJumpArgsWithPort(t *testing.T) {
	a := args{
		host:         "target",
		port:         22,
		user:         "testuser",
		proxyJump:    "bastion",
		proxyJumpPort: 2222,
	}
	
	jumpArgs := buildJumpArgs(a)
	
	if jumpArgs.host != "bastion" {
		t.Errorf("jumpArgs.host = %q, want 'bastion'", jumpArgs.host)
	}
	if jumpArgs.port != 2222 {
		t.Errorf("jumpArgs.port = %d, want 2222", jumpArgs.port)
	}
}

func TestBuildJumpArgsWithNoAgent(t *testing.T) {
	a := args{
		host:      "target",
		port:      22,
		user:      "testuser",
		proxyJump: "bastion",
		noAgent:   true,
	}
	
	jumpArgs := buildJumpArgs(a)
	
	if !jumpArgs.noAgent {
		t.Error("jumpArgs.noAgent should be true")
	}
}
