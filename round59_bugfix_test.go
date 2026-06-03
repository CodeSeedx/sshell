package main

import (
	"os"
	"path/filepath"
	"testing"
)

// Test_BookmarkRestore_CLICmdNotLost 验证书签查找时 CLI 命令不被覆盖
func Test_BookmarkRestore_CLICmdNotLost(t *testing.T) {
	// 创建临时书签目录
	tmpHome := t.TempDir()
	t.Setenv("HOME", tmpHome)
	bookmarkDir := filepath.Join(tmpHome, ".sshell")
	if err := os.MkdirAll(bookmarkDir, 0700); err != nil {
		t.Fatal(err)
	}

	// 保存一个带 cmd 的书签
	err := saveBookmark("myserver", bookmark{
		Host: "10.0.0.1",
		Port: 22,
		User: "root",
		Cmd:  "saved-cmd",
	})
	if err != nil {
		t.Fatalf("saveBookmark: %v", err)
	}

	// 模拟 CLI: sshell myserver ls -la
	// a.host = "myserver", a.cmd = "ls -la", a.user = ""
	a := args{
		host: "myserver",
		user: "",
		cmd:  "ls -la",
	}

	// 模拟 main.go 中的书签查找逻辑
	if a.user == "" && a.host != "" {
		if bk, ok := lookupBookmark(a.host); ok {
			cliCmd := a.cmd
			cliVerbose := a.verbose
			cliAgentFwd := a.agentForward
			cliCompress := a.compress
			cliLocalFwd := a.localFwd
			cliRemoteFwd := a.remoteFwd
			cliSocks := a.socksPort
			cliLog := a.logFile
			cliSftp := a.sftp
			cliNoAgent := a.noAgent
			cliReconnect := a.reconnect
			cliReconnectMax := a.reconnectMax
			cliInsecureHostKey := a.insecureHostKey
			cliScpPut := a.scpPut
			cliScpGet := a.scpGet
			cliEditFile := a.editFile
			cliPort := a.cliPort
			cliUser := a.cliUser
			cliAuth := a.cliAuth
			cliAlive := a.cliAlive
			cliPortVal := a.port
			cliUserVal := a.user
			cliAuthVal := a.auth
			cliAliveVal := a.alive

			a = bk

			if cliVerbose {
				a.verbose = true
			}
			if cliAgentFwd {
				a.agentForward = true
			}
			if cliCompress {
				a.compress = true
			}
			if len(cliLocalFwd) > 0 {
				a.localFwd = cliLocalFwd
			}
			if len(cliRemoteFwd) > 0 {
				a.remoteFwd = cliRemoteFwd
			}
			if cliSocks != "" {
				a.socksPort = cliSocks
			}
			if cliLog != "" {
				a.logFile = cliLog
			}
			if cliSftp {
				a.sftp = true
			}
			if cliNoAgent {
				a.noAgent = true
			}
			if cliReconnect {
				a.reconnect = true
			}
			if cliReconnectMax > 0 {
				a.reconnectMax = cliReconnectMax
			}
			if cliInsecureHostKey {
				a.insecureHostKey = true
			}
			if cliScpPut != "" {
				a.scpPut = cliScpPut
			}
			if cliScpGet != "" {
				a.scpGet = cliScpGet
			}
			if cliEditFile != "" {
				a.editFile = cliEditFile
			}
			if cliCmd != "" {
				a.cmd = cliCmd
			}
			if cliPort {
				a.port = cliPortVal
				a.cliPort = true
			}
			if cliUser {
				a.user = cliUserVal
				a.cliUser = true
			}
			if cliAuth {
				a.auth = cliAuthVal
				a.cliAuth = true
			}
			if cliAlive {
				a.alive = cliAliveVal
				a.cliAlive = true
			}
			if a.port == 0 {
				a.port = 22
			}
			if a.alive == 0 {
				a.alive = 30
			}
		}
	}

	// 验证 CLI cmd 未被覆盖
	if a.cmd != "ls -la" {
		t.Errorf("CLI cmd lost after bookmark lookup: got %q, want %q", a.cmd, "ls -la")
	}

	// 验证书签的 host 和 user 正确应用
	if a.host != "10.0.0.1" {
		t.Errorf("bookmark host not applied: got %q, want %q", a.host, "10.0.0.1")
	}
	if a.user != "root" {
		t.Errorf("bookmark user not applied: got %q, want %q", a.user, "root")
	}
}

// Test_BookmarkRestore_CLICmdEmpty_UsesBookmarkCmd 验证 CLI 无命令时使用书签命令
func Test_BookmarkRestore_CLICmdEmpty_UsesBookmarkCmd(t *testing.T) {
	tmpHome := t.TempDir()
	t.Setenv("HOME", tmpHome)
	bookmarkDir := filepath.Join(tmpHome, ".sshell")
	if err := os.MkdirAll(bookmarkDir, 0700); err != nil {
		t.Fatal(err)
	}

	err := saveBookmark("myserver", bookmark{
		Host: "10.0.0.1",
		Port: 22,
		User: "root",
		Cmd:  "uptime",
	})
	if err != nil {
		t.Fatalf("saveBookmark: %v", err)
	}

	// 模拟 CLI: sshell myserver（无命令）
	a := args{
		host: "myserver",
		user: "",
		cmd:  "",
	}

	if a.user == "" && a.host != "" {
		if bk, ok := lookupBookmark(a.host); ok {
			cliCmd := a.cmd
			a = bk
			if cliCmd != "" {
				a.cmd = cliCmd
			}
		}
	}

	// 验证书签的 cmd 被保留
	if a.cmd != "uptime" {
		t.Errorf("bookmark cmd not preserved: got %q, want %q", a.cmd, "uptime")
	}
}

// Test_BookmarkRestore_AllFlagsRoundtrip 验证所有 CLI 标志在书签查找后正确保留
func Test_BookmarkRestore_AllFlagsRoundtrip(t *testing.T) {
	tmpHome := t.TempDir()
	t.Setenv("HOME", tmpHome)
	bookmarkDir := filepath.Join(tmpHome, ".sshell")
	if err := os.MkdirAll(bookmarkDir, 0700); err != nil {
		t.Fatal(err)
	}

	// 保存一个空书签
	err := saveBookmark("test", bookmark{
		Host: "1.2.3.4",
		Port: 22,
		User: "testuser",
	})
	if err != nil {
		t.Fatalf("saveBookmark: %v", err)
	}

	a := args{
		host:            "test",
		user:            "", // 未指定用户，触发书签查找
		verbose:         true,
		agentForward:    true,
		compress:        true,
		localFwd:        []string{"8080:localhost:80"},
		remoteFwd:       []string{"9090:localhost:90"},
		socksPort:       "1080",
		logFile:         "/tmp/test.log",
		sftp:            true,
		noAgent:         true,
		reconnect:       true,
		reconnectMax:    5,
		insecureHostKey: true,
		scpPut:          "local:remote",
		scpGet:          "remote:local",
		editFile:        "/etc/config",
	}

	// 先设置 cmd（不能在 struct literal 中重复字段）
	a.cmd = "whoami"

	// 模拟书签查找
	if a.user == "" && a.host != "" {
		if bk, ok := lookupBookmark(a.host); ok {
			cliVerbose := a.verbose
			cliAgentFwd := a.agentForward
			cliCompress := a.compress
			cliLocalFwd := a.localFwd
			cliRemoteFwd := a.remoteFwd
			cliSocks := a.socksPort
			cliLog := a.logFile
			cliSftp := a.sftp
			cliNoAgent := a.noAgent
			cliReconnect := a.reconnect
			cliReconnectMax := a.reconnectMax
			cliInsecureHostKey := a.insecureHostKey
			cliScpPut := a.scpPut
			cliScpGet := a.scpGet
			cliEditFile := a.editFile
			cliCmd := a.cmd
			cliPort := a.cliPort
			cliUser := a.cliUser
			cliAuth := a.cliAuth
			cliAlive := a.cliAlive
			cliPortVal := a.port
			cliUserVal := a.user
			cliAuthVal := a.auth
			cliAliveVal := a.alive

			a = bk

			if cliVerbose {
				a.verbose = true
			}
			if cliAgentFwd {
				a.agentForward = true
			}
			if cliCompress {
				a.compress = true
			}
			if len(cliLocalFwd) > 0 {
				a.localFwd = cliLocalFwd
			}
			if len(cliRemoteFwd) > 0 {
				a.remoteFwd = cliRemoteFwd
			}
			if cliSocks != "" {
				a.socksPort = cliSocks
			}
			if cliLog != "" {
				a.logFile = cliLog
			}
			if cliSftp {
				a.sftp = true
			}
			if cliNoAgent {
				a.noAgent = true
			}
			if cliReconnect {
				a.reconnect = true
			}
			if cliReconnectMax > 0 {
				a.reconnectMax = cliReconnectMax
			}
			if cliInsecureHostKey {
				a.insecureHostKey = true
			}
			if cliScpPut != "" {
				a.scpPut = cliScpPut
			}
			if cliScpGet != "" {
				a.scpGet = cliScpGet
			}
			if cliEditFile != "" {
				a.editFile = cliEditFile
			}
			if cliCmd != "" {
				a.cmd = cliCmd
			}
			if cliPort {
				a.port = cliPortVal
				a.cliPort = true
			}
			if cliUser {
				a.user = cliUserVal
				a.cliUser = true
			}
			if cliAuth {
				a.auth = cliAuthVal
				a.cliAuth = true
			}
			if cliAlive {
				a.alive = cliAliveVal
				a.cliAlive = true
			}
			if a.port == 0 {
				a.port = 22
			}
			if a.alive == 0 {
				a.alive = 30
			}
		}
	}

	// 验证所有 CLI 标志被正确保留
	checks := []struct {
		name string
		got  interface{}
		want interface{}
	}{
		{"cmd", a.cmd, "whoami"},
		{"verbose", a.verbose, true},
		{"agentForward", a.agentForward, true},
		{"compress", a.compress, true},
		{"localFwd len", len(a.localFwd), 1},
		{"remoteFwd len", len(a.remoteFwd), 1},
		{"socksPort", a.socksPort, "1080"},
		{"logFile", a.logFile, "/tmp/test.log"},
		{"sftp", a.sftp, true},
		{"noAgent", a.noAgent, true},
		{"reconnect", a.reconnect, true},
		{"reconnectMax", a.reconnectMax, 5},
		{"insecureHostKey", a.insecureHostKey, true},
		{"scpPut", a.scpPut, "local:remote"},
		{"scpGet", a.scpGet, "remote:local"},
		{"editFile", a.editFile, "/etc/config"},
		{"host", a.host, "1.2.3.4"},     // 书签值
		{"user", a.user, "testuser"},     // 书签值
	}

	for _, c := range checks {
		if c.got != c.want {
			t.Errorf("%s: got %v, want %v", c.name, c.got, c.want)
		}
	}
}
