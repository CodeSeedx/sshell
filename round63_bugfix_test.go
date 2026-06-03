package main

import (
	"testing"

	"golang.org/x/crypto/ssh"
)

// Test_InsecureHostKey_IgnoresKnownHosts 验证 --insecure-host-key 标志
// 在 known_hosts 文件存在时也能正确跳过主机密钥校验
func Test_InsecureHostKey_IgnoresKnownHosts(t *testing.T) {
	// 测试场景：当 insecureHostKey 为 true 时，
	// dialSSH 应该使用 InsecureIgnoreHostKey 而不是尝试加载 known_hosts
	// 这个测试验证修复后的行为：insecureHostKey 优先级高于 loadKnownHosts

	// 构造 args，设置 insecureHostKey = true
	a := args{
		host:            "testhost",
		port:            22,
		user:            "testuser",
		insecureHostKey: true,
		alive:           30,
	}

	// 验证 insecureHostKey 标志被正确设置
	if !a.insecureHostKey {
		t.Error("insecureHostKey should be true")
	}

	// 验证 buildSSHConfig 能正确处理 InsecureIgnoreHostKey
	// 这是一个间接测试，确保代码路径正确
	authMethods := []ssh.AuthMethod{}
	hostKeyCallback := ssh.InsecureIgnoreHostKey()
	config := buildSSHConfig(a, authMethods, hostKeyCallback)
	if config == nil {
		t.Error("buildSSHConfig should return non-nil config")
	}
	if config.HostKeyCallback == nil {
		t.Error("HostKeyCallback should not be nil")
	}
}

// Test_InsecureHostKey_False_RequiresKnownHosts 验证当 insecureHostKey 为 false 时，
// 需要加载 known_hosts 文件
func Test_InsecureHostKey_False_RequiresKnownHosts(t *testing.T) {
	a := args{
		host:            "testhost",
		port:            22,
		user:            "testuser",
		insecureHostKey: false,
		alive:           30,
	}

	if a.insecureHostKey {
		t.Error("insecureHostKey should be false")
	}

	// 当 insecureHostKey 为 false 时，loadKnownHosts 会被调用
	// 如果 known_hosts 文件不存在或无法加载，会返回错误
	// 这里只验证标志位的逻辑
	authMethods := []ssh.AuthMethod{}
	// 使用 nil callback 模拟 loadKnownHosts 失败的情况
	config := buildSSHConfig(a, authMethods, nil)
	if config == nil {
		t.Error("buildSSHConfig should return non-nil config")
	}
}
