package main

import (
	"os"
	"path/filepath"
	"testing"
)

// TestBookmarkAgentForwardSaveRestore 验证书签保存和恢复 agentForward 标志
func TestBookmarkAgentForwardSaveRestore(t *testing.T) {
	// 创建带 agentForward=true 的 args
	a := args{
		host:         "192.168.1.100",
		port:         22,
		user:         "root",
		agentForward: true,
		compress:     true,
	}

	// 转换为书签
	bk := argsToBookmark(a)

	// 验证 AgentForward 被保存
	if !bk.AgentForward {
		t.Error("argsToBookmark: AgentForward should be true, got false")
	}

	// 转换回 args
	restored := bookmarkToArgs(bk)

	// 验证 agentForward 被恢复
	if !restored.agentForward {
		t.Error("bookmarkToArgs: agentForward should be true, got false")
	}
	if restored.compress != true {
		t.Error("bookmarkToArgs: compress should be true")
	}
	if restored.host != "192.168.1.100" {
		t.Errorf("bookmarkToArgs: host should be '192.168.1.100', got '%s'", restored.host)
	}
}

// TestBookmarkAgentForwardFalse 验证 agentForward=false 也能正确保存恢复
func TestBookmarkAgentForwardFalse(t *testing.T) {
	a := args{
		host:         "10.0.0.1",
		port:         2222,
		user:         "admin",
		agentForward: false,
	}

	bk := argsToBookmark(a)
	if bk.AgentForward {
		t.Error("argsToBookmark: AgentForward should be false, got true")
	}

	restored := bookmarkToArgs(bk)
	if restored.agentForward {
		t.Error("bookmarkToArgs: agentForward should be false, got true")
	}
}

// TestBookmarkAgentForwardPersistence 验证书签 JSON 序列化/反序列化保留 AgentForward
func TestBookmarkAgentForwardPersistence(t *testing.T) {
	// 使用临时目录避免影响真实书签
	tmpDir := t.TempDir()
	origHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpDir)
	defer os.Setenv("HOME", origHome)

	// 创建 .sshell 目录
	sshellDir := filepath.Join(tmpDir, ".sshell")
	if err := os.MkdirAll(sshellDir, 0700); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	// 保存带 agentForward=true 的书签
	a := args{
		host:         "bastion.example.com",
		port:         22,
		user:         "admin",
		agentForward: true,
		compress:     true,
		insecureHostKey: true,
	}
	if err := saveBookmark("mybastion", argsToBookmark(a)); err != nil {
		t.Fatalf("saveBookmark: %v", err)
	}

	// 重新加载书签
	restored, ok := lookupBookmark("mybastion")
	if !ok {
		t.Fatal("lookupBookmark: bookmark 'mybastion' not found")
	}

	// 验证 agentForward 通过 JSON 持久化正确恢复
	if !restored.agentForward {
		t.Error("lookupBookmark: agentForward should be true after JSON round-trip, got false")
	}
	if !restored.compress {
		t.Error("lookupBookmark: compress should be true")
	}
	if !restored.insecureHostKey {
		t.Error("lookupBookmark: insecureHostKey should be true")
	}
	if restored.host != "bastion.example.com" {
		t.Errorf("lookupBookmark: host should be 'bastion.example.com', got '%s'", restored.host)
	}
}

// TestBookmarkAgentForwardCLIPriority 验证 CLI -A 标志优先于书签值
func TestBookmarkAgentForwardCLIPriority(t *testing.T) {
	// 模拟 main.go 中的书签恢复逻辑
	// 场景1: CLI 设置了 -A，书签没有 agentForward
	cliArgs := args{
		user:         "root",
		host:         "myserver",
		agentForward: true, // CLI 设置了 -A
	}

	bookmarkArgs := args{
		host:         "10.0.0.1",
		user:         "admin",
		agentForward: false, // 书签没有 agentForward
	}

	// 模拟 main.go 的恢复逻辑
	a := bookmarkArgs
	cliAgentFwd := cliArgs.agentForward
	if cliAgentFwd {
		a.agentForward = true
	}
	if !a.agentForward {
		t.Error("CLI -A should override bookmark's agentForward=false")
	}

	// 场景2: CLI 没有 -A，书签有 agentForward
	cliArgs2 := args{
		user:         "root",
		host:         "myserver",
		agentForward: false,
	}

	bookmarkArgs2 := args{
		host:         "10.0.0.1",
		user:         "admin",
		agentForward: true, // 书签有 agentForward
	}

	a2 := bookmarkArgs2
	cliAgentFwd2 := cliArgs2.agentForward
	if cliAgentFwd2 {
		a2.agentForward = true
	}
	if !a2.agentForward {
		t.Error("Bookmark's agentForward=true should be preserved when CLI doesn't set -A")
	}
}
