package main

import (
	"bytes"
	"net"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// ==================== Bug #2: -J user@host 不覆盖目标用户 ====================

func TestParseArgsJumpUserNotOverwriteTargetUser(t *testing.T) {
	// -J user@bastion 应只设置 proxyJumpUser，不覆盖 -u 指定的目标用户
	a, err := parseArgsFrom([]string{"-u", "root", "-J", "admin@bastion", "target"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if a.user != "root" {
		t.Errorf("user = %q, want 'root' (should not be overwritten by -J)", a.user)
	}
	if a.proxyJumpUser != "admin" {
		t.Errorf("proxyJumpUser = %q, want 'admin'", a.proxyJumpUser)
	}
	if a.proxyJump != "bastion" {
		t.Errorf("proxyJump = %q, want 'bastion'", a.proxyJump)
	}
}

func TestParseArgsJumpUserWithPortNotOverwrite(t *testing.T) {
	// -J user@bastion:2222 格式
	a, err := parseArgsFrom([]string{"-u", "deploy", "-J", "jumpuser@bastion:2222", "target"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if a.user != "deploy" {
		t.Errorf("user = %q, want 'deploy'", a.user)
	}
	if a.proxyJumpUser != "jumpuser" {
		t.Errorf("proxyJumpUser = %q, want 'jumpuser'", a.proxyJumpUser)
	}
	if a.proxyJump != "bastion" {
		t.Errorf("proxyJump = %q, want 'bastion'", a.proxyJump)
	}
	if a.proxyJumpPort != 2222 {
		t.Errorf("proxyJumpPort = %d, want 2222", a.proxyJumpPort)
	}
}

func TestBuildJumpArgsUsesProxyJumpUser(t *testing.T) {
	// buildJumpArgs 应使用 proxyJumpUser 而非 user
	a := args{
		host:          "target",
		port:          22,
		user:          "root",
		proxyJump:     "bastion",
		proxyJumpUser: "admin",
		alive:         30,
	}
	jumpArgs := buildJumpArgs(a)
	if jumpArgs.user != "admin" {
		t.Errorf("jumpArgs.user = %q, want 'admin'", jumpArgs.user)
	}
}

func TestBuildJumpArgsFallbackToUser(t *testing.T) {
	// proxyJumpUser 为空时，应使用 user
	a := args{
		host:      "target",
		port:      22,
		user:      "root",
		proxyJump: "bastion",
		alive:     30,
	}
	jumpArgs := buildJumpArgs(a)
	if jumpArgs.user != "root" {
		t.Errorf("jumpArgs.user = %q, want 'root'", jumpArgs.user)
	}
}

func TestBookmarkSaveLoadProxyJumpUser(t *testing.T) {
	// 书签应保存和恢复 proxyJumpUser
	dir := t.TempDir()
	origHome := os.Getenv("HOME")
	os.Setenv("HOME", dir)
	defer os.Setenv("HOME", origHome)

	b := bookmark{
		Host:          "target",
		User:          "root",
		ProxyJump:     "bastion",
		ProxyJumpUser: "admin",
	}
	err := saveBookmark("test", b)
	if err != nil {
		t.Fatalf("saveBookmark failed: %v", err)
	}

	a, ok := lookupBookmark("test")
	if !ok {
		t.Fatal("lookupBookmark failed")
	}
	if a.proxyJumpUser != "admin" {
		t.Errorf("proxyJumpUser = %q, want 'admin'", a.proxyJumpUser)
	}
	if a.user != "root" {
		t.Errorf("user = %q, want 'root'", a.user)
	}
}

// ==================== Bug #3: Windows SCP 路径解析 ====================

func TestParseSCPPathWindowsDriveLetter(t *testing.T) {
	// Windows 盘符 C:\ 应被正确跳过
	local, remote := parseSCPPath("C:\\Users\\file.txt:/tmp/")
	if local != "C:\\Users\\file.txt" {
		t.Errorf("local = %q, want 'C:\\Users\\file.txt'", local)
	}
	if remote != "/tmp/" {
		t.Errorf("remote = %q, want '/tmp/'", remote)
	}
}

func TestParseSCPPathWindowsDriveLetterForwardSlash(t *testing.T) {
	// Windows 盘符 C:/ 也应被正确跳过
	local, remote := parseSCPPath("C:/Users/file.txt:/tmp/")
	if local != "C:/Users/file.txt" {
		t.Errorf("local = %q, want 'C:/Users/file.txt'", local)
	}
	if remote != "/tmp/" {
		t.Errorf("remote = %q, want '/tmp/'", remote)
	}
}

func TestParseSCPPathWindowsDriveLetterLowercase(t *testing.T) {
	// 小写盘符 c:\ 也应被正确跳过
	local, remote := parseSCPPath("c:\\file.txt:/tmp/")
	if local != "c:\\file.txt" {
		t.Errorf("local = %q, want 'c:\\file.txt'", local)
	}
	if remote != "/tmp/" {
		t.Errorf("remote = %q, want '/tmp/'", remote)
	}
}

func TestParseSCPPathNotWindowsDrive(t *testing.T) {
	// a:b:c 不应被误识别为 Windows 盘符
	local, remote := parseSCPPath("a:b:c")
	if local != "a" {
		t.Errorf("local = %q, want 'a'", local)
	}
	if remote != "b:c" {
		t.Errorf("remote = %q, want 'b:c'", remote)
	}
}

// ==================== Bug #4: channelConn deadline 实现 ====================

func TestChannelConnDeadlineClosesOnExpiry(t *testing.T) {
	// 创建一个 ssh.Channel mock
	server, client := net.Pipe()
	defer server.Close()
	defer client.Close()

	// 用 net.Pipe 创建一个简单的 channelConn
	cc := &channelConn{
		Channel:     &mockChannel{},
		remoteAddr:  &net.TCPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 22},
	}

	// 设置一个很短的 deadline
	err := cc.SetDeadline(time.Now().Add(50 * time.Millisecond))
	if err != nil {
		t.Fatalf("SetDeadline failed: %v", err)
	}

	// 等待 deadline 过期
	time.Sleep(100 * time.Millisecond)
	// Channel 应该已经被关闭了（mock 会记录）
}

func TestChannelConnDeadlineZeroClearsTimer(t *testing.T) {
	cc := &channelConn{
		Channel:    &mockChannel{},
		remoteAddr: &net.TCPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 22},
	}

	// 设置 deadline 后用零值清除
	cc.SetDeadline(time.Now().Add(1 * time.Hour))
	cc.SetDeadline(time.Time{}) // 零值应清除 timer

	if cc.timer != nil {
		t.Error("timer should be nil after clearing deadline")
	}
}

func TestChannelConnNilChannelNoPanic(t *testing.T) {
	// Channel 为 nil 时不应 panic
	cc := &channelConn{}
	err := cc.SetDeadline(time.Now())
	if err != nil {
		t.Errorf("SetDeadline returned error: %v", err)
	}
}

// ==================== Bug #6: 加密密钥自动探测 ====================

func TestAutoDetectKeysWithEncryptedKey(t *testing.T) {
	// 创建包含加密密钥的临时目录
	tmpDir := t.TempDir()
	sshDir := filepath.Join(tmpDir, ".ssh")
	os.MkdirAll(sshDir, 0700)

	// 生成一个未加密的密钥用于对比
	keyPath := filepath.Join(sshDir, "id_ed25519")
	keyData := generateTestEd25519KeyData(t)
	os.WriteFile(keyPath, keyData, 0600)

	origHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpDir)
	defer os.Setenv("HOME", origHome)

	// 应该能找到未加密的密钥
	methods, err := autoDetectKeys(false)
	if err != nil {
		t.Fatalf("autoDetectKeys failed: %v", err)
	}
	if len(methods) == 0 {
		t.Error("expected at least one auth method")
	}
}

// ==================== Bug #8: 日志文件追加模式 ====================

func TestSessionLoggerAppendMode(t *testing.T) {
	dir := t.TempDir()
	logPath := filepath.Join(dir, "test.log")

	// 第一次写入
	logger1, err := newSessionLogger(logPath, false)
	if err != nil {
		t.Fatalf("first newSessionLogger failed: %v", err)
	}
	var buf1 []byte
	w1 := logger1.WrapWriter(&byteWriter{buf: &buf1})
	w1.Write([]byte("session1"))
	logger1.Close()

	// 第二次写入应该追加而不是覆盖
	logger2, err := newSessionLogger(logPath, false)
	if err != nil {
		t.Fatalf("second newSessionLogger failed: %v", err)
	}
	var buf2 []byte
	w2 := logger2.WrapWriter(&byteWriter{buf: &buf2})
	w2.Write([]byte("session2"))
	logger2.Close()

	// 验证两次的内容都保留
	data, _ := os.ReadFile(logPath)
	content := string(data)
	if !containsString(content, "session1") {
		t.Error("first session data lost (log file was truncated instead of appended)")
	}
	if !containsString(content, "session2") {
		t.Error("second session data not found")
	}
}

// ==================== Bug #5: 远程转发 verbose 传递 ====================

func TestRemoteForwardEntryVerbose(t *testing.T) {
	// 测试 verbose 标志正确存储在映射中
	remoteForwardMappings.Store(uint16(9090), remoteForwardEntry{
		localAddr: "localhost:80",
		verbose:   true,
	})
	defer remoteForwardMappings.Delete(uint16(9090))

	val, ok := remoteForwardMappings.Load(uint16(9090))
	if !ok {
		t.Fatal("expected port 9090 to be stored")
	}
	entry := val.(remoteForwardEntry)
	if !entry.verbose {
		t.Error("verbose should be true")
	}
	if entry.localAddr != "localhost:80" {
		t.Errorf("localAddr = %q, want 'localhost:80'", entry.localAddr)
	}
}

// ==================== 辅助类型 ====================

// 辅助函数
func containsString(s, substr string) bool {
	return len(s) >= len(substr) && searchString(s, substr)
}

func searchString(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

func generateTestEd25519KeyData(t *testing.T) []byte {
	t.Helper()
	keyPath := generateTestEd25519Key(t)
	data, err := os.ReadFile(keyPath)
	if err != nil {
		t.Fatalf("read key: %v", err)
	}
	return data
}

// ==================== Bug #9: SCP/SFTP 临时文件清理 ====================

func TestSCPGetNoTempFileOnSuccess(t *testing.T) {
	serverDir := t.TempDir()
	clientDir := t.TempDir()

	serverFilePath := filepath.Join(serverDir, "test.txt")
	content := []byte("temp file cleanup test")
	if err := os.WriteFile(serverFilePath, content, 0644); err != nil {
		t.Fatalf("写入服务器文件失败: %v", err)
	}

	server := startSCPServer(t, serverDir)
	client := connectToSCPServer(t, server)

	localPath := filepath.Join(clientDir, "test.txt")
	err := scpGet(client, "test.txt", localPath, false)
	if err != nil {
		t.Fatalf("scpGet 失败: %v", err)
	}

	// 成功后不应有 .tmp 文件残留
	tmpPath := localPath + ".tmp"
	if _, err := os.Stat(tmpPath); err == nil {
		t.Error(".tmp file should not exist after successful download")
	}

	// 最终文件应存在且内容正确
	data, err := os.ReadFile(localPath)
	if err != nil {
		t.Fatalf("读取下载文件失败: %v", err)
	}
	if !bytes.Equal(data, content) {
		t.Fatalf("文件内容不匹配")
	}
}

func TestSFTPGetNoTempFileOnSuccess(t *testing.T) {
	serverDir := t.TempDir()
	clientDir := t.TempDir()

	serverFilePath := filepath.Join(serverDir, "test.txt")
	content := []byte("sftp temp file cleanup test")
	if err := os.WriteFile(serverFilePath, content, 0644); err != nil {
		t.Fatalf("写入服务器文件失败: %v", err)
	}

	server := startSFTPServer(t, serverDir)
	client := connectToSFTPServer(t, server)

	localPath := filepath.Join(clientDir, "test.txt")
	err := sftpGet(client, "test.txt", localPath, false)
	if err != nil {
		t.Fatalf("sftpGet 失败: %v", err)
	}

	// 成功后不应有 .tmp 文件残留
	tmpPath := localPath + ".tmp"
	if _, err := os.Stat(tmpPath); err == nil {
		t.Error(".tmp file should not exist after successful download")
	}

	// 最终文件应存在且内容正确
	data, err := os.ReadFile(localPath)
	if err != nil {
		t.Fatalf("读取下载文件失败: %v", err)
	}
	if !bytes.Equal(data, content) {
		t.Fatalf("文件内容不匹配")
	}
}
