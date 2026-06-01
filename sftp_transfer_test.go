package main

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"testing"

	"github.com/pkg/sftp"
	"golang.org/x/crypto/ssh"
)

// sftpTestSSHServer is a test SSH server that handles SFTP subsystem.
type sftpTestSSHServer struct {
	listener net.Listener
	config   *ssh.ServerConfig
	addr     string
	cancel   context.CancelFunc
	tmpDir   string
}

// startSFTPServer starts a test SSH server that handles SFTP subsystem.
func startSFTPServer(t *testing.T, tmpDir string) *sftpTestSSHServer {
	t.Helper()

	_, hostPrivKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("生成主机密钥失败: %v", err)
	}

	signer, err := ssh.NewSignerFromKey(hostPrivKey)
	if err != nil {
		t.Fatalf("创建签名器失败: %v", err)
	}

	config := &ssh.ServerConfig{
		PasswordCallback: func(c ssh.ConnMetadata, pass []byte) (*ssh.Permissions, error) {
			return nil, nil // allow any password
		},
		ServerVersion: "SSH-2.0-TestSFTPServer",
	}
	config.AddHostKey(signer)

	ctx, cancel := context.WithCancel(context.Background())

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		cancel()
		t.Fatalf("监听失败: %v", err)
	}

	server := &sftpTestSSHServer{
		listener: listener,
		config:   config,
		addr:     listener.Addr().String(),
		cancel:   cancel,
		tmpDir:   tmpDir,
	}

	go func() {
		for {
			conn, err := listener.Accept()
			if err != nil {
				select {
				case <-ctx.Done():
					return
				default:
					continue
				}
			}
			go server.handleConn(conn)
		}
	}()

	t.Cleanup(func() {
		cancel()
		listener.Close()
	})

	return server
}

func (s *sftpTestSSHServer) handleConn(netConn net.Conn) {
	sshConn, chans, reqs, err := ssh.NewServerConn(netConn, s.config)
	if err != nil {
		netConn.Close()
		return
	}
	defer sshConn.Close()

	go ssh.DiscardRequests(reqs)

	for newChan := range chans {
		if newChan.ChannelType() != "session" {
			newChan.Reject(ssh.UnknownChannelType, "unsupported")
			continue
		}
		channel, requests, err := newChan.Accept()
		if err != nil {
			continue
		}
		go s.handleSession(channel, requests)
	}
}

func (s *sftpTestSSHServer) handleSession(channel ssh.Channel, requests <-chan *ssh.Request) {
	defer channel.Close()

	for req := range requests {
		switch req.Type {
		case "subsystem":
			// Check if it's SFTP subsystem
			if len(req.Payload) >= 4 {
				cmdLen := int(req.Payload[0])<<24 | int(req.Payload[1])<<16 | int(req.Payload[2])<<8 | int(req.Payload[3])
				subsystem := string(req.Payload[4 : 4+cmdLen])
				if subsystem == "sftp" {
					if req.WantReply {
						req.Reply(true, nil)
					}
					s.handleSFTP(channel)
					return
				}
			}
			if req.WantReply {
				req.Reply(false, nil)
			}
		default:
			if req.WantReply {
				req.Reply(false, nil)
			}
		}
	}
}

func (s *sftpTestSSHServer) handleSFTP(channel ssh.Channel) {
	// Create SFTP server from the channel
	server, err := sftp.NewServer(
		struct {
			io.Reader
			io.WriteCloser
		}{channel, channel},
		sftp.WithServerWorkingDirectory(s.tmpDir),
	)
	if err != nil {
		fmt.Fprintf(os.Stderr, "SFTP server error: %v\n", err)
		return
	}
	defer server.Close()

	if err := server.Serve(); err != nil && err != io.EOF {
		// Connection closed, normal for SFTP
	}
}

// connectToSFTPServer creates an SSH client connected to the test SFTP server.
func connectToSFTPServer(t *testing.T, server *sftpTestSSHServer) *ssh.Client {
	t.Helper()

	config := &ssh.ClientConfig{
		User:            "testuser",
		Auth:            []ssh.AuthMethod{ssh.Password("testpass")},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
	}

	client, err := ssh.Dial("tcp", server.addr, config)
	if err != nil {
		t.Fatalf("连接 SFTP 测试服务器失败: %v", err)
	}
	t.Cleanup(func() { client.Close() })
	return client
}

// =============================================================================
// SFTP Tests
// =============================================================================

func TestSFTPPutBasic(t *testing.T) {
	serverDir := t.TempDir()
	clientDir := t.TempDir()

	server := startSFTPServer(t, serverDir)
	client := connectToSFTPServer(t, server)

	// Create a local test file
	localPath := filepath.Join(clientDir, "upload.txt")
	content := []byte("hello sftp put test\n")
	if err := os.WriteFile(localPath, content, 0644); err != nil {
		t.Fatalf("写入本地文件失败: %v", err)
	}

	remotePath := "upload.txt"
	err := sftpPut(client, localPath, remotePath, false)
	if err != nil {
		t.Fatalf("sftpPut 失败: %v", err)
	}

	// Verify file was written on the server side
	uploadedPath := filepath.Join(serverDir, "upload.txt")
	data, err := os.ReadFile(uploadedPath)
	if err != nil {
		t.Fatalf("读取上传文件失败: %v", err)
	}

	if !bytes.Equal(data, content) {
		t.Fatalf("文件内容不匹配: got %q, want %q", data, content)
	}
	t.Log("SFTP put 基本测试通过")
}

func TestSFTPPutPreservesPermissions(t *testing.T) {
	serverDir := t.TempDir()
	clientDir := t.TempDir()

	server := startSFTPServer(t, serverDir)
	client := connectToSFTPServer(t, server)

	localPath := filepath.Join(clientDir, "script.sh")
	if err := os.WriteFile(localPath, []byte("#!/bin/bash\necho hello\n"), 0755); err != nil {
		t.Fatalf("写入文件失败: %v", err)
	}

	err := sftpPut(client, localPath, "script.sh", false)
	if err != nil {
		t.Fatalf("sftpPut 失败: %v", err)
	}

	uploadedPath := filepath.Join(serverDir, "script.sh")
	info, err := os.Stat(uploadedPath)
	if err != nil {
		t.Fatalf("stat 失败: %v", err)
	}

	if info.Mode().Perm() != 0755 {
		t.Fatalf("权限不匹配: got %o, want 0755", info.Mode().Perm())
	}
	t.Log("SFTP put 权限保持测试通过")
}

func TestSFTPPutTrailingSlash(t *testing.T) {
	serverDir := t.TempDir()
	clientDir := t.TempDir()

	server := startSFTPServer(t, serverDir)
	client := connectToSFTPServer(t, server)

	localPath := filepath.Join(clientDir, "mydata.dat")
	content := []byte("data content")
	if err := os.WriteFile(localPath, content, 0644); err != nil {
		t.Fatalf("写入文件失败: %v", err)
	}

	// Remote path ends with / — should use original filename
	remotePath := "subdir/"
	err := sftpPut(client, localPath, remotePath, false)
	if err != nil {
		t.Fatalf("sftpPut 失败: %v", err)
	}

	uploadedPath := filepath.Join(serverDir, "subdir", "mydata.dat")
	data, err := os.ReadFile(uploadedPath)
	if err != nil {
		t.Fatalf("读取上传文件失败: %v", err)
	}

	if !bytes.Equal(data, content) {
		t.Fatalf("文件内容不匹配: got %q, want %q", data, content)
	}
	t.Log("SFTP put 尾部斜杠测试通过")
}

func TestSFTPPutVerbose(t *testing.T) {
	serverDir := t.TempDir()
	clientDir := t.TempDir()

	server := startSFTPServer(t, serverDir)
	client := connectToSFTPServer(t, server)

	// Create a 2MB file to trigger progress output
	localPath := filepath.Join(clientDir, "large.bin")
	data := make([]byte, 2*1024*1024)
	rand.Read(data)
	if err := os.WriteFile(localPath, data, 0644); err != nil {
		t.Fatalf("写入文件失败: %v", err)
	}

	err := sftpPut(client, localPath, "large.bin", true)
	if err != nil {
		t.Fatalf("sftpPut verbose 失败: %v", err)
	}

	uploadedPath := filepath.Join(serverDir, "large.bin")
	got, err := os.ReadFile(uploadedPath)
	if err != nil {
		t.Fatalf("读取上传文件失败: %v", err)
	}
	if !bytes.Equal(got, data) {
		t.Fatalf("文件内容不匹配")
	}
	t.Log("SFTP put verbose 测试通过")
}

func TestSFTPGetBasic(t *testing.T) {
	serverDir := t.TempDir()
	clientDir := t.TempDir()

	// Create a file on the "server"
	serverFilePath := filepath.Join(serverDir, "download.txt")
	content := []byte("hello sftp get test\n")
	if err := os.WriteFile(serverFilePath, content, 0644); err != nil {
		t.Fatalf("写入服务器文件失败: %v", err)
	}

	server := startSFTPServer(t, serverDir)
	client := connectToSFTPServer(t, server)

	localPath := filepath.Join(clientDir, "download.txt")
	err := sftpGet(client, "download.txt", localPath, false)
	if err != nil {
		t.Fatalf("sftpGet 失败: %v", err)
	}

	data, err := os.ReadFile(localPath)
	if err != nil {
		t.Fatalf("读取下载文件失败: %v", err)
	}

	if !bytes.Equal(data, content) {
		t.Fatalf("文件内容不匹配: got %q, want %q", data, content)
	}
	t.Log("SFTP get 基本测试通过")
}

func TestSFTPGetPreservesPermissions(t *testing.T) {
	serverDir := t.TempDir()
	clientDir := t.TempDir()

	// Create executable file on server
	serverFilePath := filepath.Join(serverDir, "run.sh")
	if err := os.WriteFile(serverFilePath, []byte("#!/bin/bash\necho ok\n"), 0755); err != nil {
		t.Fatalf("写入文件失败: %v", err)
	}

	server := startSFTPServer(t, serverDir)
	client := connectToSFTPServer(t, server)

	localPath := filepath.Join(clientDir, "run.sh")
	err := sftpGet(client, "run.sh", localPath, false)
	if err != nil {
		t.Fatalf("sftpGet 失败: %v", err)
	}

	info, err := os.Stat(localPath)
	if err != nil {
		t.Fatalf("stat 失败: %v", err)
	}

	if info.Mode().Perm() != 0755 {
		t.Fatalf("权限不匹配: got %o, want 0755", info.Mode().Perm())
	}
	t.Log("SFTP get 权限保持测试通过")
}

func TestSFTPGetToLocalDir(t *testing.T) {
	serverDir := t.TempDir()
	clientDir := t.TempDir()

	serverFilePath := filepath.Join(serverDir, "hello.txt")
	content := []byte("hello world")
	if err := os.WriteFile(serverFilePath, content, 0644); err != nil {
		t.Fatalf("写入服务器文件失败: %v", err)
	}

	server := startSFTPServer(t, serverDir)
	client := connectToSFTPServer(t, server)

	// localPath is an existing directory
	err := sftpGet(client, "hello.txt", clientDir, false)
	if err != nil {
		t.Fatalf("sftpGet 到目录失败: %v", err)
	}

	downloaded := filepath.Join(clientDir, "hello.txt")
	data, err := os.ReadFile(downloaded)
	if err != nil {
		t.Fatalf("读取下载文件失败: %v", err)
	}

	if !bytes.Equal(data, content) {
		t.Fatalf("文件内容不匹配: got %q, want %q", data, content)
	}
	t.Log("SFTP get 到目录测试通过")
}

func TestSFTPGetCreatesLocalDir(t *testing.T) {
	serverDir := t.TempDir()
	clientDir := t.TempDir()

	serverFilePath := filepath.Join(serverDir, "data.bin")
	content := []byte{0x00, 0x01, 0x02, 0xFF, 0xFE, 0xFD}
	if err := os.WriteFile(serverFilePath, content, 0644); err != nil {
		t.Fatalf("写入服务器文件失败: %v", err)
	}

	server := startSFTPServer(t, serverDir)
	client := connectToSFTPServer(t, server)

	localPath := filepath.Join(clientDir, "newdir", "sub", "data.bin")
	err := sftpGet(client, "data.bin", localPath, false)
	if err != nil {
		t.Fatalf("sftpGet 创建目录失败: %v", err)
	}

	data, err := os.ReadFile(localPath)
	if err != nil {
		t.Fatalf("读取下载文件失败: %v", err)
	}

	if !bytes.Equal(data, content) {
		t.Fatalf("文件内容不匹配")
	}
	t.Log("SFTP get 创建本地目录测试通过")
}

func TestSFTPGetBinaryData(t *testing.T) {
	serverDir := t.TempDir()
	clientDir := t.TempDir()

	// Create binary file with all byte values
	serverFilePath := filepath.Join(serverDir, "binary.dat")
	binaryContent := make([]byte, 256)
	for i := 0; i < 256; i++ {
		binaryContent[i] = byte(i)
	}
	if err := os.WriteFile(serverFilePath, binaryContent, 0644); err != nil {
		t.Fatalf("写入二进制文件失败: %v", err)
	}

	server := startSFTPServer(t, serverDir)
	client := connectToSFTPServer(t, server)

	localPath := filepath.Join(clientDir, "binary.dat")
	err := sftpGet(client, "binary.dat", localPath, false)
	if err != nil {
		t.Fatalf("sftpGet 二进制文件失败: %v", err)
	}

	data, err := os.ReadFile(localPath)
	if err != nil {
		t.Fatalf("读取二进制文件失败: %v", err)
	}

	if !bytes.Equal(data, binaryContent) {
		t.Fatalf("二进制文件内容不匹配")
	}
	t.Log("SFTP get 二进制数据测试通过")
}

func TestSFTPGetVerbose(t *testing.T) {
	serverDir := t.TempDir()
	clientDir := t.TempDir()

	// Create a 2MB file
	serverFilePath := filepath.Join(serverDir, "large.bin")
	largeData := make([]byte, 2*1024*1024)
	rand.Read(largeData)
	if err := os.WriteFile(serverFilePath, largeData, 0644); err != nil {
		t.Fatalf("写入大文件失败: %v", err)
	}

	server := startSFTPServer(t, serverDir)
	client := connectToSFTPServer(t, server)

	localPath := filepath.Join(clientDir, "large.bin")
	err := sftpGet(client, "large.bin", localPath, true)
	if err != nil {
		t.Fatalf("sftpGet verbose 失败: %v", err)
	}

	data, err := os.ReadFile(localPath)
	if err != nil {
		t.Fatalf("读取大文件失败: %v", err)
	}

	if !bytes.Equal(data, largeData) {
		t.Fatalf("大文件内容不匹配")
	}
	t.Log("SFTP get verbose 测试通过")
}

func TestSFTPPutNonExistentFile(t *testing.T) {
	serverDir := t.TempDir()
	server := startSFTPServer(t, serverDir)
	client := connectToSFTPServer(t, server)

	err := sftpPut(client, "/nonexistent/path/file.txt", "/file.txt", false)
	if err == nil {
		t.Fatal("上传不存在的文件应该失败")
	}
	t.Logf("预期的错误: %v", err)
}

func TestSFTPGetNonExistentFile(t *testing.T) {
	serverDir := t.TempDir()
	clientDir := t.TempDir()
	server := startSFTPServer(t, serverDir)
	client := connectToSFTPServer(t, server)

	localPath := filepath.Join(clientDir, "wont_exist.txt")
	err := sftpGet(client, "nonexistent.txt", localPath, false)
	if err == nil {
		t.Fatal("下载不存在的文件应该失败")
	}
	t.Logf("预期的错误: %v", err)
}

func TestSFTPPutRoundTrip(t *testing.T) {
	serverDir := t.TempDir()
	clientDir := t.TempDir()
	downloadDir := t.TempDir()

	server := startSFTPServer(t, serverDir)
	client := connectToSFTPServer(t, server)

	// Create original file
	originalPath := filepath.Join(clientDir, "roundtrip.txt")
	originalContent := []byte("round trip test content 12345\nwith multiple lines\n")
	if err := os.WriteFile(originalPath, originalContent, 0644); err != nil {
		t.Fatalf("写入原始文件失败: %v", err)
	}

	// Upload via sftpPut
	err := sftpPut(client, originalPath, "roundtrip.txt", false)
	if err != nil {
		t.Fatalf("sftpPut 失败: %v", err)
	}

	// Download via sftpGet
	downloadedPath := filepath.Join(downloadDir, "roundtrip.txt")
	err = sftpGet(client, "roundtrip.txt", downloadedPath, false)
	if err != nil {
		t.Fatalf("sftpGet 失败: %v", err)
	}

	// Compare
	downloadedContent, err := os.ReadFile(downloadedPath)
	if err != nil {
		t.Fatalf("读取下载文件失败: %v", err)
	}

	if !bytes.Equal(originalContent, downloadedContent) {
		t.Fatalf("往返测试失败: 内容不匹配\n原始: %q\n下载: %q", originalContent, downloadedContent)
	}
	t.Log("SFTP 往返测试通过")
}

func TestSFTPPutEmptyFile(t *testing.T) {
	serverDir := t.TempDir()
	clientDir := t.TempDir()

	server := startSFTPServer(t, serverDir)
	client := connectToSFTPServer(t, server)

	localPath := filepath.Join(clientDir, "empty.txt")
	if err := os.WriteFile(localPath, []byte{}, 0644); err != nil {
		t.Fatalf("写入空文件失败: %v", err)
	}

	err := sftpPut(client, localPath, "empty.txt", false)
	if err != nil {
		t.Fatalf("上传空文件失败: %v", err)
	}

	uploadedPath := filepath.Join(serverDir, "empty.txt")
	info, err := os.Stat(uploadedPath)
	if err != nil {
		t.Fatalf("stat 失败: %v", err)
	}

	if info.Size() != 0 {
		t.Fatalf("空文件大小应为 0, got %d", info.Size())
	}
	t.Log("SFTP put 空文件测试通过")
}

func TestSFTPGetEmptyFile(t *testing.T) {
	serverDir := t.TempDir()
	clientDir := t.TempDir()

	serverFilePath := filepath.Join(serverDir, "empty.txt")
	if err := os.WriteFile(serverFilePath, []byte{}, 0644); err != nil {
		t.Fatalf("写入空文件失败: %v", err)
	}

	server := startSFTPServer(t, serverDir)
	client := connectToSFTPServer(t, server)

	localPath := filepath.Join(clientDir, "empty.txt")
	err := sftpGet(client, "empty.txt", localPath, false)
	if err != nil {
		t.Fatalf("下载空文件失败: %v", err)
	}

	info, err := os.Stat(localPath)
	if err != nil {
		t.Fatalf("stat 失败: %v", err)
	}

	if info.Size() != 0 {
		t.Fatalf("空文件大小应为 0, got %d", info.Size())
	}
	t.Log("SFTP get 空文件测试通过")
}

func TestSFTPLargeFileTransfer(t *testing.T) {
	if testing.Short() {
		t.Skip("跳过大文件测试 (short mode)")
	}

	serverDir := t.TempDir()
	clientDir := t.TempDir()
	downloadDir := t.TempDir()

	server := startSFTPServer(t, serverDir)
	client := connectToSFTPServer(t, server)

	// Create a 10MB file
	localPath := filepath.Join(clientDir, "large.bin")
	size := 10 * 1024 * 1024
	data := make([]byte, size)
	for i := range data {
		data[i] = byte(i % 251)
	}
	if err := os.WriteFile(localPath, data, 0644); err != nil {
		t.Fatalf("写入大文件失败: %v", err)
	}

	// Upload
	err := sftpPut(client, localPath, "large.bin", false)
	if err != nil {
		t.Fatalf("上传大文件失败: %v", err)
	}

	// Download
	downloadedPath := filepath.Join(downloadDir, "large.bin")
	err = sftpGet(client, "large.bin", downloadedPath, false)
	if err != nil {
		t.Fatalf("下载大文件失败: %v", err)
	}

	downloaded, err := os.ReadFile(downloadedPath)
	if err != nil {
		t.Fatalf("读取下载文件失败: %v", err)
	}

	if !bytes.Equal(data, downloaded) {
		t.Fatalf("大文件内容不匹配 (size: uploaded=%d, downloaded=%d)", len(data), len(downloaded))
	}
	t.Logf("SFTP 大文件 (%d MB) 往返测试通过", size/(1024*1024))
}

func TestSFTPPutCreatesNestedDirs(t *testing.T) {
	serverDir := t.TempDir()
	clientDir := t.TempDir()

	server := startSFTPServer(t, serverDir)
	client := connectToSFTPServer(t, server)

	localPath := filepath.Join(clientDir, "test.txt")
	content := []byte("nested dirs test")
	if err := os.WriteFile(localPath, content, 0644); err != nil {
		t.Fatalf("写入文件失败: %v", err)
	}

	// Upload to deeply nested path
	remotePath := "a/b/c/test.txt"
	err := sftpPut(client, localPath, remotePath, false)
	if err != nil {
		t.Fatalf("sftpPut 嵌套目录失败: %v", err)
	}

	uploadedPath := filepath.Join(serverDir, "a", "b", "c", "test.txt")
	data, err := os.ReadFile(uploadedPath)
	if err != nil {
		t.Fatalf("读取上传文件失败: %v", err)
	}

	if !bytes.Equal(data, content) {
		t.Fatalf("文件内容不匹配")
	}
	t.Log("SFTP put 嵌套目录创建测试通过")
}

func TestSFTPGetTrailingSlash(t *testing.T) {
	serverDir := t.TempDir()
	clientDir := t.TempDir()

	serverFilePath := filepath.Join(serverDir, "myfile.txt")
	content := []byte("trailing slash test")
	if err := os.WriteFile(serverFilePath, content, 0644); err != nil {
		t.Fatalf("写入文件失败: %v", err)
	}

	server := startSFTPServer(t, serverDir)
	client := connectToSFTPServer(t, server)

	localPath := filepath.Join(clientDir, "dest") + string(filepath.Separator)
	err := sftpGet(client, "myfile.txt", localPath, false)
	if err != nil {
		t.Fatalf("sftpGet trailing slash 失败: %v", err)
	}

	downloaded := filepath.Join(clientDir, "dest", "myfile.txt")
	data, err := os.ReadFile(downloaded)
	if err != nil {
		t.Fatalf("读取下载文件失败: %v", err)
	}

	if !bytes.Equal(data, content) {
		t.Fatalf("文件内容不匹配")
	}
	t.Log("SFTP get 尾部斜杠测试通过")
}

func TestSFTPArgsFlagParsing(t *testing.T) {
	tests := []struct {
		name    string
		argv    []string
		wantSFTP bool
		wantPut  string
		wantGet  string
	}{
		{
			name:     "sftp with put",
			argv:     []string{"--sftp", "--put", "local.txt:/remote.txt", "-u", "root", "host"},
			wantSFTP: true,
			wantPut:  "local.txt:/remote.txt",
		},
		{
			name:     "sftp with get",
			argv:     []string{"--sftp", "--get", "/remote.txt:local.txt", "-u", "root", "host"},
			wantSFTP: true,
			wantGet:  "/remote.txt:local.txt",
		},
		{
			name:     "no sftp flag",
			argv:     []string{"--put", "local.txt:/remote.txt", "-u", "root", "host"},
			wantSFTP: false,
			wantPut:  "local.txt:/remote.txt",
		},
		{
			name:     "sftp flag alone",
			argv:     []string{"--sftp", "-u", "root", "host"},
			wantSFTP: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a, err := parseArgsFrom(tt.argv)
			if err != nil {
				t.Fatalf("parseArgsFrom 失败: %v", err)
			}
			if a.sftp != tt.wantSFTP {
				t.Errorf("sftp = %v, want %v", a.sftp, tt.wantSFTP)
			}
			if tt.wantPut != "" && a.scpPut != tt.wantPut {
				t.Errorf("scpPut = %q, want %q", a.scpPut, tt.wantPut)
			}
			if tt.wantGet != "" && a.scpGet != tt.wantGet {
				t.Errorf("scpGet = %q, want %q", a.scpGet, tt.wantGet)
			}
		})
	}
}
