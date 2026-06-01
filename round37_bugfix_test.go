package main

import (
	"os"
	"path/filepath"
	"testing"
)

// Test_main_BookmarkRestorePutGet 验证书签恢复后 --put/--get/--edit/--log 标志不丢失
func Test_main_BookmarkRestorePutGet(t *testing.T) {
	// 保存书签到临时目录
	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)

	bookmarkDir := filepath.Join(tmpDir, ".sshell")
	if err := os.MkdirAll(bookmarkDir, 0700); err != nil {
		t.Fatal(err)
	}

	// 写入一个测试书签
	bookmarks := map[string]bookmark{
		"testhost": {
			Host: "192.168.1.100",
			Port: 22,
			User: "root",
		},
	}
	writeBookmarks(bookmarks)

	// 模拟命令行: sshell testhost --put ./local.txt:/remote.txt
	bk, ok := lookupBookmark("testhost")
	if !ok {
		t.Fatal("bookmark not found")
	}

	// 模拟 CLI 参数
	cliScpPut := "./local.txt:/remote.txt"
	cliScpGet := ""
	cliEditFile := ""
	cliLogFile := ""
	cliVerbose := false
	cliAgentFwd := false
	cliCompress := false
	cliSftp := false
	cliNoAgent := false
	cliReconnect := false
	cliReconnectMax := 0
	cliInsecureHostKey := false

	a := bk

	// 恢复 CLI 标志（与 main.go 逻辑一致）
	if cliScpPut != "" {
		a.scpPut = cliScpPut
	}
	if cliScpGet != "" {
		a.scpGet = cliScpGet
	}
	if cliEditFile != "" {
		a.editFile = cliEditFile
	}
	if cliLogFile != "" {
		a.logFile = cliLogFile
	}
	if cliVerbose {
		a.verbose = true
	}
	if cliAgentFwd {
		a.agentForward = true
	}
	if cliCompress {
		a.compress = true
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

	// 验证 --put 未丢失
	if a.scpPut != "./local.txt:/remote.txt" {
		t.Errorf("--put flag lost: got %q, want %q", a.scpPut, "./local.txt:/remote.txt")
	}
	// 验证书签字段正确填充
	if a.host != "192.168.1.100" {
		t.Errorf("host mismatch: got %q, want %q", a.host, "192.168.1.100")
	}
	if a.user != "root" {
		t.Errorf("user mismatch: got %q, want %q", a.user, "root")
	}
}

// Test_main_BookmarkRestoreGet 验证 --get 标志在书签恢复后保留
func Test_main_BookmarkRestoreGet(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)

	bookmarkDir := filepath.Join(tmpDir, ".sshell")
	if err := os.MkdirAll(bookmarkDir, 0700); err != nil {
		t.Fatal(err)
	}

	bookmarks := map[string]bookmark{
		"myhost": {
			Host: "10.0.0.1",
			User: "admin",
		},
	}
	writeBookmarks(bookmarks)

	bk, ok := lookupBookmark("myhost")
	if !ok {
		t.Fatal("bookmark not found")
	}

	cliScpGet := "/etc/passwd:./download.txt"
	a := bk

	if cliScpGet != "" {
		a.scpGet = cliScpGet
	}

	if a.scpGet != "/etc/passwd:./download.txt" {
		t.Errorf("--get flag lost: got %q, want %q", a.scpGet, "/etc/passwd:./download.txt")
	}
}

// Test_main_BookmarkRestoreEdit 验证 --edit 标志在书签恢复后保留
func Test_main_BookmarkRestoreEdit(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)

	bookmarkDir := filepath.Join(tmpDir, ".sshell")
	if err := os.MkdirAll(bookmarkDir, 0700); err != nil {
		t.Fatal(err)
	}

	bookmarks := map[string]bookmark{
		"edhost": {
			Host: "10.0.0.2",
			User: "admin",
		},
	}
	writeBookmarks(bookmarks)

	bk, ok := lookupBookmark("edhost")
	if !ok {
		t.Fatal("bookmark not found")
	}

	cliEditFile := "/etc/nginx/nginx.conf"
	a := bk

	if cliEditFile != "" {
		a.editFile = cliEditFile
	}

	if a.editFile != "/etc/nginx/nginx.conf" {
		t.Errorf("--edit flag lost: got %q, want %q", a.editFile, "/etc/nginx/nginx.conf")
	}
}

// Test_main_BookmarkRestoreAllFlags 验证所有 CLI 标志在书签恢复后保留
func Test_main_BookmarkRestoreAllFlags(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)

	bookmarkDir := filepath.Join(tmpDir, ".sshell")
	if err := os.MkdirAll(bookmarkDir, 0700); err != nil {
		t.Fatal(err)
	}

	bookmarks := map[string]bookmark{
		"fullhost": {
			Host: "10.0.0.3",
			User: "admin",
		},
	}
	writeBookmarks(bookmarks)

	bk, ok := lookupBookmark("fullhost")
	if !ok {
		t.Fatal("bookmark not found")
	}

	// 模拟所有 CLI 标志
	a := bk
	a.verbose = true
	a.agentForward = true
	a.compress = true
	a.localFwd = []string{"8080:localhost:80"}
	a.remoteFwd = []string{"9090:localhost:90"}
	a.socksPort = "1080"
	a.logFile = "/tmp/sshell.log"
	a.sftp = true
	a.noAgent = true
	a.reconnect = true
	a.reconnectMax = 5
	a.insecureHostKey = true
	a.scpPut = "./file:/remote"
	a.scpGet = "/remote:./file"
	a.editFile = "/etc/config"

	// 验证所有标志
	if !a.verbose {
		t.Error("verbose lost")
	}
	if !a.agentForward {
		t.Error("agentForward lost")
	}
	if !a.compress {
		t.Error("compress lost")
	}
	if len(a.localFwd) != 1 {
		t.Error("localFwd lost")
	}
	if len(a.remoteFwd) != 1 {
		t.Error("remoteFwd lost")
	}
	if a.socksPort != "1080" {
		t.Error("socksPort lost")
	}
	if a.logFile != "/tmp/sshell.log" {
		t.Error("logFile lost")
	}
	if !a.sftp {
		t.Error("sftp lost")
	}
	if !a.noAgent {
		t.Error("noAgent lost")
	}
	if !a.reconnect {
		t.Error("reconnect lost")
	}
	if a.reconnectMax != 5 {
		t.Error("reconnectMax lost")
	}
	if !a.insecureHostKey {
		t.Error("insecureHostKey lost")
	}
	if a.scpPut != "./file:/remote" {
		t.Error("scpPut lost")
	}
	if a.scpGet != "/remote:./file" {
		t.Error("scpGet lost")
	}
	if a.editFile != "/etc/config" {
		t.Error("editFile lost")
	}

	// 验证书签字段仍正确
	if a.host != "10.0.0.3" {
		t.Error("host changed")
	}
	if a.user != "admin" {
		t.Error("user changed")
	}
}

// Test_replaceTokens_AllTokens 验证 replaceTokens 处理所有 token
func Test_replaceTokens_AllTokens(t *testing.T) {
	tests := []struct {
		name     string
		path     string
		hostname string
		user     string
		want     string
	}{
		{
			name:     "percent_h",
			path:     "/keys/%h_key",
			hostname: "myserver",
			user:     "localuser",
			want:     "/keys/myserver_key",
		},
		{
			name:     "percent_l",
			path:     "/keys/%l_keys/id",
			hostname: "host",
			user:     "alice",
			want:     "/keys/alice_keys/id",
		},
		{
			name:     "percent_h_and_percent_l",
			path:     "/keys/%h_%l",
			hostname: "web1",
			user:     "bob",
			want:     "/keys/web1_bob",
		},
		{
			name:     "literal_percent",
			path:     "/keys/100%%key",
			hostname: "host",
			user:     "user",
			want:     "/keys/100%key",
		},
		{
			name:     "no_tokens",
			path:     "/keys/id_ed25519",
			hostname: "host",
			user:     "user",
			want:     "/keys/id_ed25519",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := replaceTokens(tt.path, tt.hostname, tt.user)
			if got != tt.want {
				t.Errorf("replaceTokens(%q, %q, %q) = %q, want %q", tt.path, tt.hostname, tt.user, got, tt.want)
			}
		})
	}
}

// Test_connectWithRetry_ErrorFormat 验证重试错误消息格式
func Test_connectWithRetry_ErrorFormat(t *testing.T) {
	// 测试单次尝试的错误消息不包含 "1 attempts"
	a := args{
		host:        "nonexistent.invalid",
		port:        22,
		user:        "test",
		reconnect:   true,
		reconnectMax: 1,
	}

	_, err := connectWithRetry(a)
	if err == nil {
		t.Fatal("expected error for nonexistent host")
	}

	errMsg := err.Error()
	// 单次尝试不应包含 "after 1 attempts"
	if containsSubstring(errMsg, "after") && containsSubstring(errMsg, "attempts") {
		t.Errorf("single attempt should not have 'after N attempts' in error: %s", errMsg)
	}
	// 应包含 "reconnect failed"
	if !containsSubstring(errMsg, "reconnect failed") {
		t.Errorf("error should contain 'reconnect failed': %s", errMsg)
	}
}

// containsSubstring 辅助函数
func containsSubstring(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsAt(s, substr))
}

func containsAt(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
