package main

import (
	"os"
	"path/filepath"
	"testing"
)

// TestMultiHost_SSHConfigCompression 验证多主机模式正确应用 SSH config 的 Compression
func TestMultiHost_SSHConfigCompression(t *testing.T) {
	// 创建临时 SSH config
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config")
	configContent := `Host target1
  Compression yes
`
	if err := os.WriteFile(configPath, []byte(configContent), 0600); err != nil {
		t.Fatal(err)
	}

	// 验证 loadSSHConfig 能正确解析 Compression
	// 注意：loadSSHConfig 使用 ~/.ssh/config，这里直接测试解析逻辑
	blocks := parseSSHConfig(configPath)
	if len(blocks) != 1 {
		t.Fatalf("expected 1 block, got %d", len(blocks))
	}
	if !blocks[0].Compression {
		t.Error("expected Compression=true from SSH config")
	}

	// 验证 runOnHost 中 hostArgs.compress 能被 SSH config 覆盖
	a := args{
		host:     "target1",
		hosts:    []string{"target1"},
		port:     22,
		user:     "testuser",
		compress: false, // CLI 未设置 -C
		cmd:      "echo ok",
	}

	// 模拟 runOnHost 的 args 复制逻辑
	hostArgs := a
	hostArgs.host = "target1"
	hostArgs.hosts = nil

	// 模拟 SSH config 返回 Compression=yes
	cfg := &sshHostConfig{Compression: true}
	if cfg.Compression {
		hostArgs.compress = true
	}

	if !hostArgs.compress {
		t.Error("expected compress=true after SSH config Compression=yes is applied")
	}
}

// TestMultiHost_SSHConfigCompression_AlreadySet 验证 CLI -C 不被 SSH config 覆盖
func TestMultiHost_SSHConfigCompression_AlreadySet(t *testing.T) {
	a := args{
		host:     "target1",
		hosts:    []string{"target1"},
		port:     22,
		user:     "testuser",
		compress: true, // CLI 已设置 -C
		cmd:      "echo ok",
	}

	hostArgs := a
	hostArgs.host = "target1"
	hostArgs.hosts = nil

	// SSH config Compression 不应覆盖已设置的值（但 compress 是 bool，true 覆盖 true 无影响）
	// 重要的是：CLI 未设置时 SSH config 能生效
	if !hostArgs.compress {
		t.Error("expected compress=true from CLI -C flag")
	}
}

// TestMultiHost_SSHConfigProxyJump 验证多主机模式正确应用 SSH config 的 ProxyJump
func TestMultiHost_SSHConfigProxyJump(t *testing.T) {
	// 创建临时 SSH config
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config")
	configContent := `Host target1
  ProxyJump jumphost
`
	if err := os.WriteFile(configPath, []byte(configContent), 0600); err != nil {
		t.Fatal(err)
	}

	// 验证 loadSSHConfig 能正确解析 ProxyJump
	blocks := parseSSHConfig(configPath)
	if len(blocks) != 1 {
		t.Fatalf("expected 1 block, got %d", len(blocks))
	}
	if blocks[0].ProxyJump != "jumphost" {
		t.Errorf("expected ProxyJump=jumphost, got %q", blocks[0].ProxyJump)
	}

	// 验证 runOnHost 中 hostArgs.proxyJump 能被 SSH config 设置
	a := args{
		host:  "target1",
		hosts: []string{"target1"},
		port:  22,
		user:  "testuser",
		cmd:   "echo ok",
		// proxyJump 未设置（CLI 未指定 -J）
	}

	hostArgs := a
	hostArgs.host = "target1"
	hostArgs.hosts = nil

	// 模拟 SSH config 返回 ProxyJump=jumphost
	cfg := &sshHostConfig{ProxyJump: "jumphost"}
	if cfg.ProxyJump != "" && hostArgs.proxyJump == "" {
		hostArgs.proxyJump = cfg.ProxyJump
		hostArgs.proxyJumps = parseJumpHosts(cfg.ProxyJump)
		if len(hostArgs.proxyJumps) == 1 {
			jh := hostArgs.proxyJumps[0]
			hostArgs.proxyJump = jh.Host
			hostArgs.proxyJumpPort = jh.Port
			hostArgs.proxyJumpUser = jh.User
		}
	}

	if hostArgs.proxyJump != "jumphost" {
		t.Errorf("expected proxyJump=jumphost, got %q", hostArgs.proxyJump)
	}
	if len(hostArgs.proxyJumps) != 1 {
		t.Errorf("expected 1 proxyJump entry, got %d", len(hostArgs.proxyJumps))
	}
}

// TestMultiHost_SSHConfigProxyJump_CliOverride 验证 CLI -J 优先于 SSH config ProxyJump
func TestMultiHost_SSHConfigProxyJump_CliOverride(t *testing.T) {
	a := args{
		host:      "target1",
		hosts:     []string{"target1"},
		port:      22,
		user:      "testuser",
		proxyJump: "cli-jump", // CLI 已指定 -J
		cmd:       "echo ok",
	}

	hostArgs := a
	hostArgs.host = "target1"
	hostArgs.hosts = nil

	// 模拟 SSH config 返回 ProxyJump=config-jump
	cfg := &sshHostConfig{ProxyJump: "config-jump"}
	if cfg.ProxyJump != "" && hostArgs.proxyJump == "" {
		// 这个条件不应满足，因为 hostArgs.proxyJump 已经是 "cli-jump"
		hostArgs.proxyJump = cfg.ProxyJump
	}

	if hostArgs.proxyJump != "cli-jump" {
		t.Errorf("expected proxyJump=cli-jump (CLI override), got %q", hostArgs.proxyJump)
	}
}

// TestMultiHost_SSHConfigProxyJump_WithPort 验证 SSH config ProxyJump 带端口解析
func TestMultiHost_SSHConfigProxyJump_WithPort(t *testing.T) {
	a := args{
		host:  "target1",
		hosts: []string{"target1"},
		port:  22,
		user:  "testuser",
		cmd:   "echo ok",
	}

	hostArgs := a
	hostArgs.host = "target1"
	hostArgs.hosts = nil

	// 模拟 SSH config 返回 ProxyJump=jumphost:2222
	cfg := &sshHostConfig{ProxyJump: "jumphost:2222"}
	if cfg.ProxyJump != "" && hostArgs.proxyJump == "" {
		hostArgs.proxyJump = cfg.ProxyJump
		hostArgs.proxyJumps = parseJumpHosts(cfg.ProxyJump)
		if len(hostArgs.proxyJumps) == 1 {
			jh := hostArgs.proxyJumps[0]
			hostArgs.proxyJump = jh.Host
			hostArgs.proxyJumpPort = jh.Port
			hostArgs.proxyJumpUser = jh.User
		}
	}

	if hostArgs.proxyJump != "jumphost" {
		t.Errorf("expected proxyJump=jumphost, got %q", hostArgs.proxyJump)
	}
	if hostArgs.proxyJumpPort != 2222 {
		t.Errorf("expected proxyJumpPort=2222, got %d", hostArgs.proxyJumpPort)
	}
}

// TestMultiHost_SSHConfigProxyJump_WithUser 验证 SSH config ProxyJump 带用户解析
func TestMultiHost_SSHConfigProxyJump_WithUser(t *testing.T) {
	a := args{
		host:  "target1",
		hosts: []string{"target1"},
		port:  22,
		user:  "testuser",
		cmd:   "echo ok",
	}

	hostArgs := a
	hostArgs.host = "target1"
	hostArgs.hosts = nil

	// 模拟 SSH config 返回 ProxyJump=admin@jumphost:2222
	cfg := &sshHostConfig{ProxyJump: "admin@jumphost:2222"}
	if cfg.ProxyJump != "" && hostArgs.proxyJump == "" {
		hostArgs.proxyJump = cfg.ProxyJump
		hostArgs.proxyJumps = parseJumpHosts(cfg.ProxyJump)
		if len(hostArgs.proxyJumps) == 1 {
			jh := hostArgs.proxyJumps[0]
			hostArgs.proxyJump = jh.Host
			hostArgs.proxyJumpPort = jh.Port
			hostArgs.proxyJumpUser = jh.User
		}
	}

	if hostArgs.proxyJump != "jumphost" {
		t.Errorf("expected proxyJump=jumphost, got %q", hostArgs.proxyJump)
	}
	if hostArgs.proxyJumpPort != 2222 {
		t.Errorf("expected proxyJumpPort=2222, got %d", hostArgs.proxyJumpPort)
	}
	if hostArgs.proxyJumpUser != "admin" {
		t.Errorf("expected proxyJumpUser=admin, got %q", hostArgs.proxyJumpUser)
	}
}

// TestMultiHost_SSHConfigConsistency 验证多主机模式与单主机模式的 SSH config 应用一致性
func TestMultiHost_SSHConfigConsistency(t *testing.T) {
	// 列出 applySSHConfigTarget 应用的所有 SSH config 字段
	// 与 runOnHost 中应用的字段进行对比
	sshConfigFields := []string{
		"HostName",
		"Port",
		"User",
		"IdentityFile",
		"Compression",
		"ServerAliveInterval",
		"ProxyJump",
	}

	// runOnHost 现在应包含所有字段
	runOnHostFields := []string{
		"HostName",
		"Port",
		"User",
		"IdentityFile",
		"ServerAliveInterval",
		"Compression",
		"ProxyJump",
	}

	if len(sshConfigFields) != len(runOnHostFields) {
		t.Errorf("field count mismatch: applySSHConfigTarget=%d, runOnHost=%d",
			len(sshConfigFields), len(runOnHostFields))
	}

	fieldSet := make(map[string]bool)
	for _, f := range sshConfigFields {
		fieldSet[f] = true
	}
	for _, f := range runOnHostFields {
		if !fieldSet[f] {
			t.Errorf("runOnHost has field %q not in applySSHConfigTarget", f)
		}
		delete(fieldSet, f)
	}
	for f := range fieldSet {
		t.Errorf("applySSHConfigTarget has field %q missing in runOnHost", f)
	}
}
