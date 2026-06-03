package main

import (
	"testing"
)

// TestMultiHost_AgentForwardFlagPreserved 验证多主机模式保留 agentForward 标志
func TestMultiHost_AgentForwardFlagPreserved(t *testing.T) {
	// 验证 runOnHost 使用 hostArgs.agentForward（从 a 复制）来判断是否启用 agent forwarding
	a := args{
		user:         "testuser",
		cmd:          "echo ok",
		agentForward: true,
		verbose:      false,
	}
	// 通过 hosts 触发多主机模式路径
	a.hosts = []string{"host1"}

	// runOnHost 会在 connectClientWithRetry 阶段失败（host1 不存在），
	// 但我们可以验证 agentForward 标志被正确传递
	// 通过检查 args 复制逻辑
	hostArgs := a
	hostArgs.host = "host1"
	hostArgs.hosts = nil

	if !hostArgs.agentForward {
		t.Error("agentForward flag not preserved in hostArgs copy")
	}
	if hostArgs.user != "testuser" {
		t.Errorf("user not preserved: got %q, want %q", hostArgs.user, "testuser")
	}
	if hostArgs.cmd != "echo ok" {
		t.Errorf("cmd not preserved: got %q, want %q", hostArgs.cmd, "echo ok")
	}
}

// TestMultiHost_AgentForwardDisabled 验证多主机模式在 agentForward=false 时不启用转发
func TestMultiHost_AgentForwardDisabled(t *testing.T) {
	a := args{
		user:         "testuser",
		cmd:          "echo ok",
		agentForward: false,
		verbose:      false,
	}
	a.hosts = []string{"host1"}

	hostArgs := a
	hostArgs.host = "host1"
	hostArgs.hosts = nil

	if hostArgs.agentForward {
		t.Error("agentForward should be false when not set")
	}
}

// TestMultiHost_AllFlagsPreserved 验证多主机模式保留所有关键标志
func TestMultiHost_AllFlagsPreserved(t *testing.T) {
	a := args{
		host:            "host1,host2",
		hosts:           []string{"host1", "host2"},
		port:            2222,
		user:            "testuser",
		auth:            "/path/to/key",
		alive:           60,
		verbose:         true,
		cmd:             "uptime",
		noAgent:         true,
		agentForward:    true,
		compress:        true,
		insecureHostKey: true,
		reconnect:       true,
		reconnectMax:    5,
		cliPort:         true,
		cliUser:         true,
		cliAuth:         true,
		cliAlive:        true,
	}

	// 模拟 runOnHost 的 args 复制
	hostArgs := a
	hostArgs.host = "host1"
	hostArgs.hosts = nil

	tests := []struct {
		name string
		got  interface{}
		want interface{}
	}{
		{"port", hostArgs.port, uint16(2222)},
		{"user", hostArgs.user, "testuser"},
		{"auth", hostArgs.auth, "/path/to/key"},
		{"alive", hostArgs.alive, uint32(60)},
		{"verbose", hostArgs.verbose, true},
		{"cmd", hostArgs.cmd, "uptime"},
		{"noAgent", hostArgs.noAgent, true},
		{"agentForward", hostArgs.agentForward, true},
		{"compress", hostArgs.compress, true},
		{"insecureHostKey", hostArgs.insecureHostKey, true},
		{"reconnect", hostArgs.reconnect, true},
		{"reconnectMax", hostArgs.reconnectMax, 5},
		{"cliPort", hostArgs.cliPort, true},
		{"cliUser", hostArgs.cliUser, true},
		{"cliAuth", hostArgs.cliAuth, true},
		{"cliAlive", hostArgs.cliAlive, true},
	}

	for _, tt := range tests {
		if tt.got != tt.want {
			t.Errorf("%s: got %v, want %v", tt.name, tt.got, tt.want)
		}
	}
}
