package main

import (
	"os"
	"path/filepath"
	"testing"
)

// Test_bookmark_restore_proxyJump 验证书签恢复时 CLI -J 标志不被覆盖
func Test_bookmark_restore_proxyJump(t *testing.T) {
	// 创建临时书签目录
	tmpDir := t.TempDir()
	origHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpDir)
	defer os.Setenv("HOME", origHome)

	sshellDir := filepath.Join(tmpDir, ".sshell")
	if err := os.MkdirAll(sshellDir, 0700); err != nil {
		t.Fatal(err)
	}

	// 保存一个没有 proxyJump 的书签
	bookmarks := map[string]bookmark{
		"myserver": {
			Host: "10.0.0.1",
			Port: 22,
			User: "root",
		},
	}
	if err := writeBookmarks(bookmarks); err != nil {
		t.Fatal(err)
	}

	// 测试：CLI -J 应该在书签恢复后保留
	t.Run("CLI_J_preserved_over_bookmark", func(t *testing.T) {
		argv := []string{"-u", "root", "-J", "bastion", "myserver"}
		a, err := parseArgsFrom(argv)
		if err != nil {
			t.Fatal(err)
		}

		// 模拟书签查找过程
		if bk, ok := lookupBookmark(a.host); ok {
			cliProxyJump := a.proxyJump
			cliProxyJumpPort := a.proxyJumpPort
			cliProxyJumpUser := a.proxyJumpUser
			cliProxyJumps := a.proxyJumps

			a = bk

			if cliProxyJump != "" {
				a.proxyJump = cliProxyJump
				a.proxyJumpPort = cliProxyJumpPort
				a.proxyJumpUser = cliProxyJumpUser
				a.proxyJumps = cliProxyJumps
			}
		}

		if a.proxyJump != "bastion" {
			t.Errorf("expected proxyJump='bastion', got '%s'", a.proxyJump)
		}
		if a.host != "10.0.0.1" {
			t.Errorf("expected host='10.0.0.1' (from bookmark), got '%s'", a.host)
		}
	})

	// 测试：书签自己的 proxyJump 应该在 CLI 未指定时生效
	t.Run("bookmark_proxyJump_used_when_no_CLI", func(t *testing.T) {
		// 更新书签，加入 proxyJump
		bookmarks["myserver"] = bookmark{
			Host:      "10.0.0.1",
			Port:      22,
			User:      "root",
			ProxyJump: "bookjump",
		}
		if err := writeBookmarks(bookmarks); err != nil {
			t.Fatal(err)
		}

		argv := []string{"-u", "root", "myserver"}
		a, err := parseArgsFrom(argv)
		if err != nil {
			t.Fatal(err)
		}

		if bk, ok := lookupBookmark(a.host); ok {
			cliProxyJump := a.proxyJump
			cliProxyJumpPort := a.proxyJumpPort
			cliProxyJumpUser := a.proxyJumpUser
			cliProxyJumps := a.proxyJumps

			a = bk

			if cliProxyJump != "" {
				a.proxyJump = cliProxyJump
				a.proxyJumpPort = cliProxyJumpPort
				a.proxyJumpUser = cliProxyJumpUser
				a.proxyJumps = cliProxyJumps
			}
		}

		if a.proxyJump != "bookjump" {
			t.Errorf("expected proxyJump='bookjump' (from bookmark), got '%s'", a.proxyJump)
		}
	})

	// 测试：多跳 -J 应该正确保留
	t.Run("multi_hop_J_preserved", func(t *testing.T) {
		// 清除书签的 proxyJump
		bookmarks["myserver"] = bookmark{
			Host: "10.0.0.1",
			Port: 22,
			User: "root",
		}
		if err := writeBookmarks(bookmarks); err != nil {
			t.Fatal(err)
		}

		argv := []string{"-u", "root", "-J", "j1,j2,j3", "myserver"}
		a, err := parseArgsFrom(argv)
		if err != nil {
			t.Fatal(err)
		}

		if bk, ok := lookupBookmark(a.host); ok {
			cliProxyJump := a.proxyJump
			cliProxyJumpPort := a.proxyJumpPort
			cliProxyJumpUser := a.proxyJumpUser
			cliProxyJumps := a.proxyJumps

			a = bk

			if cliProxyJump != "" {
				a.proxyJump = cliProxyJump
				a.proxyJumpPort = cliProxyJumpPort
				a.proxyJumpUser = cliProxyJumpUser
				a.proxyJumps = cliProxyJumps
			}
		}

		if a.proxyJump != "j1,j2,j3" {
			t.Errorf("expected proxyJump='j1,j2,j3', got '%s'", a.proxyJump)
		}
		if len(a.proxyJumps) != 3 {
			t.Errorf("expected 3 proxy jumps, got %d", len(a.proxyJumps))
		}
	})
}
