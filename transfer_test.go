package main

import (
	"bufio"
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"golang.org/x/crypto/ssh"
)

// scpTestSSHServer is a test SSH server that handles SCP protocol commands.
type scpTestSSHServer struct {
	listener net.Listener
	config   *ssh.ServerConfig
	addr     string
	cancel   context.CancelFunc
	tmpDir   string
}

// startSCPServer starts a test SSH server that handles SCP commands.
// It writes uploaded files to tmpDir and serves files from tmpDir for downloads.
func startSCPServer(t *testing.T, tmpDir string) *scpTestSSHServer {
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
		ServerVersion: "SSH-2.0-TestSCPServer",
	}
	config.AddHostKey(signer)

	ctx, cancel := context.WithCancel(context.Background())

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		cancel()
		t.Fatalf("监听失败: %v", err)
	}

	server := &scpTestSSHServer{
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

func (s *scpTestSSHServer) handleConn(netConn net.Conn) {
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

func (s *scpTestSSHServer) handleSession(channel ssh.Channel, requests <-chan *ssh.Request) {
	defer channel.Close()

	for req := range requests {
		switch req.Type {
		case "exec":
			if len(req.Payload) < 4 {
				if req.WantReply {
					req.Reply(false, nil)
				}
				continue
			}
			cmdLen := int(req.Payload[0])<<24 | int(req.Payload[1])<<16 | int(req.Payload[2])<<8 | int(req.Payload[3])
			cmd := string(req.Payload[4 : 4+cmdLen])

			if req.WantReply {
				req.Reply(true, nil)
			}

			// Route SCP commands to protocol handler
			if strings.HasPrefix(cmd, "scp -t ") {
				s.handleSCPPut(channel, cmd)
			} else if strings.HasPrefix(cmd, "scp -f ") {
				s.handleSCPGet(channel, cmd)
			} else {
				fmt.Fprintf(channel, "echo: %s\n", cmd)
				channel.CloseWrite()
				exitMsg := struct{ Status uint32 }{0}
				channel.SendRequest("exit-status", false, ssh.Marshal(&exitMsg))
			}
			return

		default:
			if req.WantReply {
				req.Reply(false, nil)
			}
		}
	}
}

// handleSCPPut handles "scp -t <dir>" — receives a file upload from the client.
func (s *scpTestSSHServer) handleSCPPut(channel ssh.Channel, cmd string) {
	dir := strings.TrimSpace(strings.TrimPrefix(cmd, "scp -t "))
	dir = stripShellQuotes(dir)
	if dir == "" {
		dir = "."
	}
	// Make absolute under tmpDir
	dir = filepath.Join(s.tmpDir, dir)
	os.MkdirAll(dir, 0755)

	r := bufio.NewReader(channel)
	w := bufio.NewWriter(channel)

	// Send OK to start
	w.WriteByte(0x00)
	w.Flush()

	// Read "C<mode> <size> <name>\n"
	header, err := r.ReadString('\n')
	if err != nil {
		w.WriteByte(0x01)
		w.WriteString("failed to read header\n")
		w.Flush()
		exitMsg := struct{ Status uint32 }{1}
		channel.SendRequest("exit-status", false, ssh.Marshal(&exitMsg))
		return
	}

	if header[0] != 'C' {
		w.WriteByte(0x01)
		w.WriteString("expected C header\n")
		w.Flush()
		exitMsg := struct{ Status uint32 }{1}
		channel.SendRequest("exit-status", false, ssh.Marshal(&exitMsg))
		return
	}

	parts := strings.Fields(header[1:])
	if len(parts) < 3 {
		w.WriteByte(0x01)
		w.WriteString("malformed header\n")
		w.Flush()
		exitMsg := struct{ Status uint32 }{1}
		channel.SendRequest("exit-status", false, ssh.Marshal(&exitMsg))
		return
	}

	modeStr := parts[0]
	size, _ := strconv.ParseInt(parts[1], 10, 64)
	name := strings.TrimSpace(parts[2])

	// Parse octal mode
	perm, _ := strconv.ParseUint(modeStr, 8, 32)
	if perm == 0 {
		perm = 0644
	}

	outPath := filepath.Join(dir, name)

	// Ack header
	w.WriteByte(0x00)
	w.Flush()

	// Read file data
	f, err := os.OpenFile(outPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, os.FileMode(perm))
	if err != nil {
		w.WriteByte(0x01)
		w.WriteString("failed to create file\n")
		w.Flush()
		exitMsg := struct{ Status uint32 }{1}
		channel.SendRequest("exit-status", false, ssh.Marshal(&exitMsg))
		return
	}

	received := int64(0)
	for received < size {
		remaining := size - received
		n, err := io.CopyN(f, r, remaining)
		received += n
		if err != nil {
			f.Close()
			w.WriteByte(0x01)
			w.WriteString("read error\n")
			w.Flush()
			exitMsg := struct{ Status uint32 }{1}
			channel.SendRequest("exit-status", false, ssh.Marshal(&exitMsg))
			return
		}
	}
	f.Close()

	// Read trailing \x00 from client
	ack := make([]byte, 1)
	r.Read(ack)

	// Send OK
	w.WriteByte(0x00)
	w.Flush()

	exitMsg := struct{ Status uint32 }{0}
	channel.SendRequest("exit-status", false, ssh.Marshal(&exitMsg))
}

// handleSCPGet handles "scp -f <path>" — sends a file download to the client.
func (s *scpTestSSHServer) handleSCPGet(channel ssh.Channel, cmd string) {
	remotePath := strings.TrimSpace(strings.TrimPrefix(cmd, "scp -f "))
	remotePath = stripShellQuotes(remotePath)
	fullPath := filepath.Join(s.tmpDir, remotePath)

	r := bufio.NewReader(channel)
	w := bufio.NewWriter(channel)

	// Read initial \x00 from client
	ack := make([]byte, 1)
	if _, err := r.Read(ack); err != nil {
		exitMsg := struct{ Status uint32 }{1}
		channel.SendRequest("exit-status", false, ssh.Marshal(&exitMsg))
		return
	}

	// Open file
	f, err := os.Open(fullPath)
	if err != nil {
		w.WriteByte(0x02)
		fmt.Fprintf(w, "scp: %s: No such file or directory\n", remotePath)
		w.Flush()
		exitMsg := struct{ Status uint32 }{1}
		channel.SendRequest("exit-status", false, ssh.Marshal(&exitMsg))
		return
	}
	defer f.Close()

	stat, err := f.Stat()
	if err != nil {
		w.WriteByte(0x02)
		fmt.Fprintf(w, "scp: cannot stat\n")
		w.Flush()
		exitMsg := struct{ Status uint32 }{1}
		channel.SendRequest("exit-status", false, ssh.Marshal(&exitMsg))
		return
	}

	// Send header "C<mode> <size> <name>\n"
	mode := fmt.Sprintf("%04o", stat.Mode().Perm())
	name := filepath.Base(remotePath)
	fmt.Fprintf(w, "C%s %d %s\n", mode, stat.Size(), name)
	w.Flush()

	// Wait for ack
	if _, err := r.Read(ack); err != nil {
		exitMsg := struct{ Status uint32 }{1}
		channel.SendRequest("exit-status", false, ssh.Marshal(&exitMsg))
		return
	}
	if ack[0] != 0x00 {
		exitMsg := struct{ Status uint32 }{1}
		channel.SendRequest("exit-status", false, ssh.Marshal(&exitMsg))
		return
	}

	// Send file data
	if _, err := io.Copy(w, f); err != nil {
		exitMsg := struct{ Status uint32 }{1}
		channel.SendRequest("exit-status", false, ssh.Marshal(&exitMsg))
		return
	}

	// Send end-of-file marker
	w.WriteByte(0x00)
	w.Flush()

	// Wait for ack
	if _, err := r.Read(ack); err != nil {
		exitMsg := struct{ Status uint32 }{1}
		channel.SendRequest("exit-status", false, ssh.Marshal(&exitMsg))
		return
	}

	// Send final OK
	w.WriteByte(0x00)
	w.Flush()

	exitMsg := struct{ Status uint32 }{0}
	channel.SendRequest("exit-status", false, ssh.Marshal(&exitMsg))
}

// connectToSCPServer creates an SSH client connected to the test SCP server.
func connectToSCPServer(t *testing.T, server *scpTestSSHServer) *ssh.Client {
	t.Helper()

	config := &ssh.ClientConfig{
		User:            "testuser",
		Auth:            []ssh.AuthMethod{ssh.Password("testpass")},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
	}

	client, err := ssh.Dial("tcp", server.addr, config)
	if err != nil {
		t.Fatalf("连接 SCP 测试服务器失败: %v", err)
	}
	t.Cleanup(func() { client.Close() })
	return client
}

// generateTestKeyForTransfer generates Ed25519 key pair in temp dir for tests.
func generateTestKeyForTransfer(t *testing.T, dir string) (privPath, pubPath string) {
	t.Helper()

	pubKey, privKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("生成密钥失败: %v", err)
	}

	privPath = filepath.Join(dir, "id_ed25519_test")
	pkcs8Bytes, err := x509.MarshalPKCS8PrivateKey(privKey)
	if err != nil {
		t.Fatalf("PKCS8 编码失败: %v", err)
	}
	privPEM := &pem.Block{Type: "PRIVATE KEY", Bytes: pkcs8Bytes}
	if err := os.WriteFile(privPath, pem.EncodeToMemory(privPEM), 0600); err != nil {
		t.Fatalf("写入私钥失败: %v", err)
	}

	pubPath = filepath.Join(dir, "id_ed25519_test.pub")
	sshPubKey, err := ssh.NewPublicKey(pubKey)
	if err != nil {
		t.Fatalf("创建公钥失败: %v", err)
	}
	if err := os.WriteFile(pubPath, ssh.MarshalAuthorizedKey(sshPubKey), 0644); err != nil {
		t.Fatalf("写入公钥失败: %v", err)
	}

	return privPath, pubPath
}

// =============================================================================
// Tests
// =============================================================================

func TestSCPPutBasic(t *testing.T) {
	serverDir := t.TempDir()
	clientDir := t.TempDir()

	server := startSCPServer(t, serverDir)
	client := connectToSCPServer(t, server)

	// Create a local test file
	localPath := filepath.Join(clientDir, "upload.txt")
	content := []byte("hello scp put test\n")
	if err := os.WriteFile(localPath, content, 0644); err != nil {
		t.Fatalf("写入本地文件失败: %v", err)
	}

	remotePath := "/upload.txt"
	err := scpPut(client, localPath, remotePath, false)
	if err != nil {
		t.Fatalf("scpPut 失败: %v", err)
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
	t.Log("SCP put 基本测试通过")
}

func TestSCPPutPreservesPermissions(t *testing.T) {
	serverDir := t.TempDir()
	clientDir := t.TempDir()

	server := startSCPServer(t, serverDir)
	client := connectToSCPServer(t, server)

	localPath := filepath.Join(clientDir, "script.sh")
	if err := os.WriteFile(localPath, []byte("#!/bin/bash\necho hello\n"), 0755); err != nil {
		t.Fatalf("写入文件失败: %v", err)
	}

	err := scpPut(client, localPath, "/script.sh", false)
	if err != nil {
		t.Fatalf("scpPut 失败: %v", err)
	}

	uploadedPath := filepath.Join(serverDir, "script.sh")
	info, err := os.Stat(uploadedPath)
	if err != nil {
		t.Fatalf("stat 失败: %v", err)
	}

	if info.Mode().Perm() != 0755 {
		t.Fatalf("权限不匹配: got %o, want 0755", info.Mode().Perm())
	}
	t.Log("SCP put 权限保持测试通过")
}

func TestSCPPutTrailingSlash(t *testing.T) {
	serverDir := t.TempDir()
	clientDir := t.TempDir()

	server := startSCPServer(t, serverDir)
	client := connectToSCPServer(t, server)

	// Create local file
	localPath := filepath.Join(clientDir, "mydata.dat")
	content := []byte("data content")
	if err := os.WriteFile(localPath, content, 0644); err != nil {
		t.Fatalf("写入文件失败: %v", err)
	}

	// Remote path ends with / — should use original filename
	remotePath := "/subdir/"
	err := scpPut(client, localPath, remotePath, false)
	if err != nil {
		t.Fatalf("scpPut 失败: %v", err)
	}

	// Verify file was placed with original name under remotePath
	uploadedPath := filepath.Join(serverDir, "subdir", "mydata.dat")
	data, err := os.ReadFile(uploadedPath)
	if err != nil {
		t.Fatalf("读取上传文件失败: %v", err)
	}

	if !bytes.Equal(data, content) {
		t.Fatalf("文件内容不匹配: got %q, want %q", data, content)
	}
	t.Log("SCP put 尾部斜杠测试通过")
}

func TestSCPPutVerbose(t *testing.T) {
	serverDir := t.TempDir()
	clientDir := t.TempDir()

	server := startSCPServer(t, serverDir)
	client := connectToSCPServer(t, server)

	// Create a larger file to trigger progress output (but still small enough for fast test)
	localPath := filepath.Join(clientDir, "large.bin")
	// 2MB file to trigger verbose progress
	data := make([]byte, 2*1024*1024)
	rand.Read(data)
	if err := os.WriteFile(localPath, data, 0644); err != nil {
		t.Fatalf("写入文件失败: %v", err)
	}

	err := scpPut(client, localPath, "/large.bin", true)
	if err != nil {
		t.Fatalf("scpPut verbose 失败: %v", err)
	}

	// Verify
	uploadedPath := filepath.Join(serverDir, "large.bin")
	got, err := os.ReadFile(uploadedPath)
	if err != nil {
		t.Fatalf("读取上传文件失败: %v", err)
	}
	if !bytes.Equal(got, data) {
		t.Fatalf("文件内容不匹配")
	}
	t.Log("SCP put verbose 测试通过")
}

func TestSCPGetBasic(t *testing.T) {
	serverDir := t.TempDir()
	clientDir := t.TempDir()

	// Create a file on the "server"
	serverFilePath := filepath.Join(serverDir, "download.txt")
	content := []byte("hello scp get test\n")
	if err := os.WriteFile(serverFilePath, content, 0644); err != nil {
		t.Fatalf("写入服务器文件失败: %v", err)
	}

	server := startSCPServer(t, serverDir)
	client := connectToSCPServer(t, server)

	localPath := filepath.Join(clientDir, "download.txt")
	err := scpGet(client, "download.txt", localPath, false)
	if err != nil {
		t.Fatalf("scpGet 失败: %v", err)
	}

	// Verify downloaded content
	data, err := os.ReadFile(localPath)
	if err != nil {
		t.Fatalf("读取下载文件失败: %v", err)
	}

	if !bytes.Equal(data, content) {
		t.Fatalf("文件内容不匹配: got %q, want %q", data, content)
	}
	t.Log("SCP get 基本测试通过")
}

func TestSCPGetToLocalDir(t *testing.T) {
	serverDir := t.TempDir()
	clientDir := t.TempDir()

	// Create a file on the server
	serverFilePath := filepath.Join(serverDir, "hello.txt")
	content := []byte("hello world")
	if err := os.WriteFile(serverFilePath, content, 0644); err != nil {
		t.Fatalf("写入服务器文件失败: %v", err)
	}

	server := startSCPServer(t, serverDir)
	client := connectToSCPServer(t, server)

	// localPath is an existing directory — file should be saved as hello.txt inside it
	err := scpGet(client, "hello.txt", clientDir, false)
	if err != nil {
		t.Fatalf("scpGet 到目录失败: %v", err)
	}

	downloaded := filepath.Join(clientDir, "hello.txt")
	data, err := os.ReadFile(downloaded)
	if err != nil {
		t.Fatalf("读取下载文件失败: %v", err)
	}

	if !bytes.Equal(data, content) {
		t.Fatalf("文件内容不匹配: got %q, want %q", data, content)
	}
	t.Log("SCP get 到目录测试通过")
}

func TestSCPGetCreatesLocalDir(t *testing.T) {
	serverDir := t.TempDir()
	clientDir := t.TempDir()

	// Create a file on the server
	serverFilePath := filepath.Join(serverDir, "data.bin")
	content := []byte{0x00, 0x01, 0x02, 0xFF, 0xFE, 0xFD}
	if err := os.WriteFile(serverFilePath, content, 0644); err != nil {
		t.Fatalf("写入服务器文件失败: %v", err)
	}

	server := startSCPServer(t, serverDir)
	client := connectToSCPServer(t, server)

	// localPath with non-existent parent directory
	localPath := filepath.Join(clientDir, "newdir", "sub", "data.bin")
	err := scpGet(client, "data.bin", localPath, false)
	if err != nil {
		t.Fatalf("scpGet 创建目录失败: %v", err)
	}

	data, err := os.ReadFile(localPath)
	if err != nil {
		t.Fatalf("读取下载文件失败: %v", err)
	}

	if !bytes.Equal(data, content) {
		t.Fatalf("文件内容不匹配")
	}
	t.Log("SCP get 创建本地目录测试通过")
}

func TestSCPGetBinaryData(t *testing.T) {
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

	server := startSCPServer(t, serverDir)
	client := connectToSCPServer(t, server)

	localPath := filepath.Join(clientDir, "binary.dat")
	err := scpGet(client, "binary.dat", localPath, false)
	if err != nil {
		t.Fatalf("scpGet 二进制文件失败: %v", err)
	}

	data, err := os.ReadFile(localPath)
	if err != nil {
		t.Fatalf("读取二进制文件失败: %v", err)
	}

	if !bytes.Equal(data, binaryContent) {
		t.Fatalf("二进制文件内容不匹配")
	}
	t.Log("SCP get 二进制数据测试通过")
}

func TestSCPGetVerbose(t *testing.T) {
	serverDir := t.TempDir()
	clientDir := t.TempDir()

	// Create a 2MB file
	serverFilePath := filepath.Join(serverDir, "large.bin")
	largeData := make([]byte, 2*1024*1024)
	rand.Read(largeData)
	if err := os.WriteFile(serverFilePath, largeData, 0644); err != nil {
		t.Fatalf("写入大文件失败: %v", err)
	}

	server := startSCPServer(t, serverDir)
	client := connectToSCPServer(t, server)

	localPath := filepath.Join(clientDir, "large.bin")
	err := scpGet(client, "large.bin", localPath, true)
	if err != nil {
		t.Fatalf("scpGet verbose 失败: %v", err)
	}

	data, err := os.ReadFile(localPath)
	if err != nil {
		t.Fatalf("读取大文件失败: %v", err)
	}

	if !bytes.Equal(data, largeData) {
		t.Fatalf("大文件内容不匹配")
	}
	t.Log("SCP get verbose 测试通过")
}

func TestSCPGetTrailingSlash(t *testing.T) {
	serverDir := t.TempDir()
	clientDir := t.TempDir()

	// Create file on server
	serverFilePath := filepath.Join(serverDir, "myfile.txt")
	content := []byte("trailing slash test")
	if err := os.WriteFile(serverFilePath, content, 0644); err != nil {
		t.Fatalf("写入文件失败: %v", err)
	}

	server := startSCPServer(t, serverDir)
	client := connectToSCPServer(t, server)

	// localPath ends with / — should create dir and save with original filename
	localPath := filepath.Join(clientDir, "dest") + string(filepath.Separator)
	err := scpGet(client, "myfile.txt", localPath, false)
	if err != nil {
		t.Fatalf("scpGet trailing slash 失败: %v", err)
	}

	downloaded := filepath.Join(clientDir, "dest", "myfile.txt")
	data, err := os.ReadFile(downloaded)
	if err != nil {
		t.Fatalf("读取下载文件失败: %v", err)
	}

	if !bytes.Equal(data, content) {
		t.Fatalf("文件内容不匹配")
	}
	t.Log("SCP get 尾部斜杠测试通过")
}

func TestSCPPutNonExistentFile(t *testing.T) {
	serverDir := t.TempDir()
	server := startSCPServer(t, serverDir)
	client := connectToSCPServer(t, server)

	err := scpPut(client, "/nonexistent/path/file.txt", "/file.txt", false)
	if err == nil {
		t.Fatal("上传不存在的文件应该失败")
	}
	t.Logf("预期的错误: %v", err)
}

func TestSCPGetNonExistentFile(t *testing.T) {
	serverDir := t.TempDir()
	clientDir := t.TempDir()
	server := startSCPServer(t, serverDir)
	client := connectToSCPServer(t, server)

	localPath := filepath.Join(clientDir, "wont_exist.txt")
	err := scpGet(client, "nonexistent.txt", localPath, false)
	if err == nil {
		t.Fatal("下载不存在的文件应该失败")
	}
	t.Logf("预期的错误: %v", err)
}

func TestSCPPutRoundTrip(t *testing.T) {
	serverDir := t.TempDir()
	clientDir := t.TempDir()
	downloadDir := t.TempDir()

	server := startSCPServer(t, serverDir)
	client := connectToSCPServer(t, server)

	// Create original file
	originalPath := filepath.Join(clientDir, "roundtrip.txt")
	originalContent := []byte("round trip test content 12345\nwith multiple lines\n")
	if err := os.WriteFile(originalPath, originalContent, 0644); err != nil {
		t.Fatalf("写入原始文件失败: %v", err)
	}

	// Upload via scpPut
	err := scpPut(client, originalPath, "/roundtrip.txt", false)
	if err != nil {
		t.Fatalf("scpPut 失败: %v", err)
	}

	// Download via scpGet
	downloadedPath := filepath.Join(downloadDir, "roundtrip.txt")
	err = scpGet(client, "roundtrip.txt", downloadedPath, false)
	if err != nil {
		t.Fatalf("scpGet 失败: %v", err)
	}

	// Compare
	downloadedContent, err := os.ReadFile(downloadedPath)
	if err != nil {
		t.Fatalf("读取下载文件失败: %v", err)
	}

	if !bytes.Equal(originalContent, downloadedContent) {
		t.Fatalf("往返测试失败: 内容不匹配\n原始: %q\n下载: %q", originalContent, downloadedContent)
	}
	t.Log("SCP 往返测试通过")
}

func TestSCPPutEmptyFile(t *testing.T) {
	serverDir := t.TempDir()
	clientDir := t.TempDir()

	server := startSCPServer(t, serverDir)
	client := connectToSCPServer(t, server)

	// Empty file
	localPath := filepath.Join(clientDir, "empty.txt")
	if err := os.WriteFile(localPath, []byte{}, 0644); err != nil {
		t.Fatalf("写入空文件失败: %v", err)
	}

	err := scpPut(client, localPath, "/empty.txt", false)
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
	t.Log("SCP put 空文件测试通过")
}

func TestSCPGetEmptyFile(t *testing.T) {
	serverDir := t.TempDir()
	clientDir := t.TempDir()

	// Empty file on server
	serverFilePath := filepath.Join(serverDir, "empty.txt")
	if err := os.WriteFile(serverFilePath, []byte{}, 0644); err != nil {
		t.Fatalf("写入空文件失败: %v", err)
	}

	server := startSCPServer(t, serverDir)
	client := connectToSCPServer(t, server)

	localPath := filepath.Join(clientDir, "empty.txt")
	err := scpGet(client, "empty.txt", localPath, false)
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
	t.Log("SCP get 空文件测试通过")
}

func TestSCPLargeFileTransfer(t *testing.T) {
	if testing.Short() {
		t.Skip("跳过大文件测试 (short mode)")
	}

	serverDir := t.TempDir()
	clientDir := t.TempDir()
	downloadDir := t.TempDir()

	server := startSCPServer(t, serverDir)
	client := connectToSCPServer(t, server)

	// Create a 10MB file
	localPath := filepath.Join(clientDir, "large.bin")
	size := 10 * 1024 * 1024
	data := make([]byte, size)
	// Fill with predictable pattern
	for i := range data {
		data[i] = byte(i % 251) // prime to avoid obvious patterns
	}
	if err := os.WriteFile(localPath, data, 0644); err != nil {
		t.Fatalf("写入大文件失败: %v", err)
	}

	// Upload
	err := scpPut(client, localPath, "/large.bin", false)
	if err != nil {
		t.Fatalf("上传大文件失败: %v", err)
	}

	// Download
	downloadedPath := filepath.Join(downloadDir, "large.bin")
	err = scpGet(client, "large.bin", downloadedPath, false)
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
	t.Logf("SCP 大文件 (%d MB) 往返测试通过", size/(1024*1024))
}

// stripShellQuotes 去除 shell 引号（单引号或双引号包裹的字符串）
func stripShellQuotes(s string) string {
	if len(s) >= 2 {
		if (s[0] == '\'' && s[len(s)-1] == '\'') || (s[0] == '"' && s[len(s)-1] == '"') {
			return s[1 : len(s)-1]
		}
	}
	return s
}
