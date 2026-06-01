package main

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/ed25519"
	"encoding/pem"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"testing"

	"crypto/x509"
	"strings"
	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/agent"
)

// =============================================================================
// 测试辅助：内存 SSH 服务器
// =============================================================================

// testSSHServer 表示一个用于测试的内存 SSH 服务器
type testSSHServer struct {
	listener net.Listener
	config   *ssh.ServerConfig
	addr     string
	stderr   bytes.Buffer
	cancel   context.CancelFunc
}

// startTestSSHServer 启动一个测试用 SSH 服务器
// password: 允许的密码（空字符串表示只允许公钥认证）
// echo: 如果为 true，服务器会将执行的命令输出回客户端（用于命令回显测试）
func startTestSSHServer(t *testing.T, password string, authorizedKeys []ssh.PublicKey, echo bool) *testSSHServer {
	t.Helper()

	// 生成 Ed25519 主机密钥
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
			if password != "" && string(pass) == password {
				return nil, nil
			}
			return nil, fmt.Errorf("密码错误")
		},
		PublicKeyCallback: func(c ssh.ConnMetadata, key ssh.PublicKey) (*ssh.Permissions, error) {
			for _, ak := range authorizedKeys {
				if bytes.Equal(ssh.MarshalAuthorizedKey(key), ssh.MarshalAuthorizedKey(ak)) {
					return nil, nil
				}
			}
			return nil, fmt.Errorf("公钥未授权")
		},
		ServerVersion: "SSH-2.0-TestServer",
	}
	config.AddHostKey(signer)

	ctx, cancel := context.WithCancel(context.Background())

	// 监听随机端口
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		cancel()
		t.Fatalf("监听失败: %v", err)
	}

	server := &testSSHServer{
		listener: listener,
		config:   config,
		addr:     listener.Addr().String(),
		cancel:   cancel,
	}

	// 接受连接循环
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
			go server.handleConn(ctx, conn, echo)
		}
	}()

	t.Cleanup(func() {
		cancel()
		listener.Close()
	})

	return server
}

func (s *testSSHServer) handleConn(ctx context.Context, netConn net.Conn, echo bool) {
	sshConn, chans, reqs, err := ssh.NewServerConn(netConn, s.config)
	if err != nil {
		fmt.Fprintf(&s.stderr, "SSH 握手失败: %v\n", err)
		netConn.Close()
		return
	}
	defer sshConn.Close()

	go ssh.DiscardRequests(reqs)

	for newChan := range chans {
		if newChan.ChannelType() != "session" {
			newChan.Reject(ssh.UnknownChannelType, "不支持的通道类型")
			continue
		}
		channel, requests, err := newChan.Accept()
		if err != nil {
			continue
		}
		go s.handleSession(ctx, channel, requests, echo)
	}
}

func (s *testSSHServer) handleSession(ctx context.Context, channel ssh.Channel, requests <-chan *ssh.Request, echo bool) {
	defer channel.Close()

	for req := range requests {
		switch req.Type {
		case "auth-agent-req@openssh.com":
			// 允许 agent forwarding 请求
			if req.WantReply {
				req.Reply(true, nil)
			}
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

			if echo {
				fmt.Fprintf(channel, "server-echo: %s\n", cmd)
			} else {
				fmt.Fprintf(channel, "ok\n")
			}
			channel.CloseWrite()
			// 发送退出状态 0
			exitMsg := struct{ Status uint32 }{0}
			channel.SendRequest("exit-status", false, ssh.Marshal(&exitMsg))
			return

		default:
			if req.WantReply {
				req.Reply(false, nil)
			}
		}
	}
}

// =============================================================================
// 辅助函数
// =============================================================================

// generateTestKey 生成 Ed25519 测试密钥对，写入临时目录
func generateTestKey(t *testing.T, dir string) (privPath, pubPath string) {
	t.Helper()

	pubKey, privKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("生成密钥失败: %v", err)
	}

	// 写私钥 (PKCS8 格式)
	privPath = filepath.Join(dir, "id_ed25519_test")
	pkcs8Bytes, err := x509.MarshalPKCS8PrivateKey(privKey)
	if err != nil {
		t.Fatalf("PKCS8 编码失败: %v", err)
	}
	privPEM := &pem.Block{
		Type:  "PRIVATE KEY",
		Bytes: pkcs8Bytes,
	}
	privData := pem.EncodeToMemory(privPEM)
	if err := os.WriteFile(privPath, privData, 0600); err != nil {
		t.Fatalf("写入私钥失败: %v", err)
	}

	// 写公钥
	pubPath = filepath.Join(dir, "id_ed25519_test.pub")
	sshPubKey, err := ssh.NewPublicKey(pubKey)
	if err != nil {
		t.Fatalf("创建公钥签名器失败: %v", err)
	}
	pubBytes := ssh.MarshalAuthorizedKey(sshPubKey)
	if err := os.WriteFile(pubPath, pubBytes, 0644); err != nil {
		t.Fatalf("写入公钥失败: %v", err)
	}

	return privPath, pubPath
}

// makeTestArgs 构造测试用的 args
func makeTestArgs(server *testSSHServer, user, password string) args {
	return args{
		host:  "127.0.0.1",
		port:  uint16(server.listener.Addr().(*net.TCPAddr).Port),
		user:  user,
		auth:  password,
		alive: 30,
	}
}

// =============================================================================
// 集成测试
// =============================================================================

func TestIntegrationPasswordAuth(t *testing.T) {
	server := startTestSSHServer(t, "testpass", nil, false)
	a := makeTestArgs(server, "testuser", "testpass")

	session, err := connect(a)
	if err != nil {
		t.Fatalf("连接失败: %v", err)
	}
	defer session.Close()

	// 执行简单命令验证会话可用
	if err := session.Run("echo hello"); err != nil {
		t.Fatalf("执行命令失败: %v", err)
	}

	t.Log("密码认证连接成功")
}

func TestIntegrationWrongPassword(t *testing.T) {
	server := startTestSSHServer(t, "correctpass", nil, false)
	a := makeTestArgs(server, "testuser", "wrongpass")

	_, err := connect(a)
	if err == nil {
		t.Fatal("错误密码应该连接失败")
	}
	t.Logf("预期的认证失败: %v", err)
}

func TestIntegrationCommandEcho(t *testing.T) {
	server := startTestSSHServer(t, "testpass", nil, true)
	a := makeTestArgs(server, "testuser", "testpass")

	session, err := connect(a)
	if err != nil {
		t.Fatalf("连接失败: %v", err)
	}
	defer session.Close()

	// 执行命令，捕获输出
	var stdout bytes.Buffer
	session.Stdout = &stdout

	if err := session.Run("ls -la"); err != nil {
		t.Fatalf("执行命令失败: %v", err)
	}

	output := stdout.String()
	if output == "" {
		t.Fatal("命令输出为空")
	}
	t.Logf("命令输出: %q", output)
}

func TestIntegrationKeyAuth(t *testing.T) {
	dir := t.TempDir()
	privPath, pubPath := generateTestKey(t, dir)

	// 加载公钥作为服务器的授权密钥
	pubKeyData, err := os.ReadFile(pubPath)
	if err != nil {
		t.Fatalf("读取公钥失败: %v", err)
	}
	authorizedKey, _, _, _, err := ssh.ParseAuthorizedKey(pubKeyData)
	if err != nil {
		t.Fatalf("解析公钥失败: %v", err)
	}

	server := startTestSSHServer(t, "", []ssh.PublicKey{authorizedKey}, false)

	// 用私钥文件作为认证
	a := makeTestArgs(server, "testuser", "")
	a.auth = privPath

	session, err := connect(a)
	if err != nil {
		t.Fatalf("密钥认证连接失败: %v", err)
	}
	defer session.Close()

	if err := session.Run("echo key-auth-ok"); err != nil {
		t.Fatalf("执行命令失败: %v", err)
	}

	t.Log("密钥认证连接成功")
}

func TestIntegrationKeyAuthFailure(t *testing.T) {
	dir := t.TempDir()
	_, pubPath := generateTestKey(t, dir)

	// 服务器只授权这个密钥
	pubKeyData, _ := os.ReadFile(pubPath)
	authorizedKey, _, _, _, _ := ssh.ParseAuthorizedKey(pubKeyData)
	server := startTestSSHServer(t, "", []ssh.PublicKey{authorizedKey}, false)

	// 客户端用另一个密钥
	dir2 := t.TempDir()
	otherPrivPath, _ := generateTestKey(t, dir2)

	a := makeTestArgs(server, "testuser", "")
	a.auth = otherPrivPath

	_, err := connect(a)
	if err == nil {
		t.Fatal("不同密钥应该认证失败")
	}
	t.Logf("预期的公钥认证失败: %v", err)
}

func TestIntegrationConnectionRefused(t *testing.T) {
	// 连一个没人监听的端口
	a := args{
		host:  "127.0.0.1",
		port:  19999,
		user:  "testuser",
		auth:  "testpass",
		alive: 30,
	}

	_, err := connect(a)
	if err == nil {
		t.Fatal("连接不存在的端口应该失败")
	}
	t.Logf("预期的连接失败: %v", err)
}

func TestIntegrationVerboseOutput(t *testing.T) {
	server := startTestSSHServer(t, "testpass", nil, false)
	a := makeTestArgs(server, "testuser", "testpass")
	a.verbose = true

	// verbose 会往 stderr 打日志，这里验证不会崩溃
	session, err := connect(a)
	if err != nil {
		t.Fatalf("verbose 模式连接失败: %v", err)
	}
	defer session.Close()

	if err := session.Run("echo verbose-test"); err != nil {
		t.Fatalf("verbose 模式执行命令失败: %v", err)
	}

	t.Log("verbose 模式连接成功")
}

func TestIntegrationMultipleSessions(t *testing.T) {
	server := startTestSSHServer(t, "testpass", nil, true)

	// SSH session 只能执行一条命令，多条命令需要多个 session（多次连接）
	commands := []string{"echo first", "echo second", "echo third"}
	for _, cmd := range commands {
		a := makeTestArgs(server, "testuser", "testpass")
		session, err := connect(a)
		if err != nil {
			t.Errorf("连接失败 (%s): %v", cmd, err)
			continue
		}

		var stdout bytes.Buffer
		session.Stdout = &stdout

		if err := session.Run(cmd); err != nil {
			t.Errorf("执行命令 %q 失败: %v", cmd, err)
		} else {
			t.Logf("命令 %q 输出: %q", cmd, stdout.String())
		}
		session.Close()
	}
}

func TestIntegrationSessionClose(t *testing.T) {
	server := startTestSSHServer(t, "testpass", nil, false)
	a := makeTestArgs(server, "testuser", "testpass")

	session, err := connect(a)
	if err != nil {
		t.Fatalf("连接失败: %v", err)
	}

	// 不执行命令直接关闭
	if err := session.Close(); err != nil {
		t.Fatalf("关闭会话失败: %v", err)
	}

	t.Log("会话正常关闭")
}

func TestIntegrationInvalidSSHServer(t *testing.T) {
	// 测试连接一个接受 TCP 但立即关闭的服务器
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("监听失败: %v", err)
	}
	defer listener.Close()

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
			// 立即关闭连接，模拟非 SSH 服务
			conn.Close()
		}
	}()

	a := args{
		host:  "127.0.0.1",
		port:  uint16(listener.Addr().(*net.TCPAddr).Port),
		user:  "testuser",
		auth:  "testpass",
		alive: 5,
	}

	_, err = connect(a)
	if err == nil {
		t.Fatal("连接非 SSH 服务器应该失败")
	}
	t.Logf("预期的握手失败: %v", err)
}

func TestIntegrationLoadKnownHosts(t *testing.T) {
	// 测试 loadKnownHosts 函数在没有 known_hosts 文件时的行为
	// 使用不存在的 home 目录来模拟
	dir := t.TempDir()

	// 临时修改 HOME 环境变量
	origHome := os.Getenv("HOME")
	os.Setenv("HOME", dir)
	defer os.Setenv("HOME", origHome)

	// 没有 .ssh/known_hosts 文件
	_, err := loadKnownHosts(false)
	if err == nil {
		t.Log("loadKnownHosts 在没有 known_hosts 时返回了错误")
	} else {
		t.Logf("预期的错误: %v", err)
	}
}

func TestIntegrationAuthErrorMessages(t *testing.T) {
	server := startTestSSHServer(t, "realpass", nil, false)

	tests := []struct {
		name     string
		password string
		wantErr  bool
	}{
		{"正确密码", "realpass", false},
		{"空密码", "", true},
		{"错误密码", "wrongpass", true},
		{"超长密码", "a" + string(make([]byte, 10000)), true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a := makeTestArgs(server, "testuser", tt.password)
			_, err := connect(a)
			if tt.wantErr && err == nil {
				t.Error("应该返回错误")
			}
			if !tt.wantErr && err != nil {
				t.Errorf("不应该返回错误: %v", err)
			}
		})
	}
}

func TestIntegrationSessionAfterClose(t *testing.T) {
	// 测试关闭会话后不能再执行命令
	server := startTestSSHServer(t, "testpass", nil, true)
	a := makeTestArgs(server, "testuser", "testpass")

	session, err := connect(a)
	if err != nil {
		t.Fatalf("连接失败: %v", err)
	}

	// 关闭前执行一条命令
	if err := session.Run("echo before-close"); err != nil {
		t.Fatalf("关闭前执行命令失败: %v", err)
	}

	// 关闭会话
	session.Close()

	// 关闭后执行命令应该失败
	err = session.Run("echo after-close")
	if err == nil {
		t.Error("关闭会话后执行命令应该失败")
	} else {
		t.Logf("预期的关闭后错误: %v", err)
	}
}

func TestIntegrationConcurrentConnections(t *testing.T) {
	server := startTestSSHServer(t, "testpass", nil, false)

	const numClients = 3
	errCh := make(chan error, numClients)

	for i := 0; i < numClients; i++ {
		go func() {
			a := makeTestArgs(server, "testuser", "testpass")
			session, err := connect(a)
			if err != nil {
				errCh <- fmt.Errorf("连接失败: %w", err)
				return
			}
			defer session.Close()

			var stdout bytes.Buffer
			session.Stdout = &stdout
			if err := session.Run("echo concurrent"); err != nil {
				errCh <- fmt.Errorf("执行命令失败: %w", err)
				return
			}
			errCh <- nil
		}()
	}

	for i := 0; i < numClients; i++ {
		if err := <-errCh; err != nil {
			t.Errorf("客户端 %d 失败: %v", i, err)
		}
	}

	t.Log("并发连接测试通过")
}

func TestIntegrationCommandStderr(t *testing.T) {
	// 测试通过 stderr 接收输出
	server := startTestSSHServer(t, "testpass", nil, false)
	a := makeTestArgs(server, "testuser", "testpass")

	session, err := connect(a)
	if err != nil {
		t.Fatalf("连接失败: %v", err)
	}
	defer session.Close()

	var stdout, stderr bytes.Buffer
	session.Stdout = &stdout
	session.Stderr = &stderr

	// 即使服务器只输出到 stdout，stderr 应该为空但不崩溃
	if err := session.Run("echo test-output"); err != nil {
		t.Fatalf("执行命令失败: %v", err)
	}

	t.Logf("stdout: %q, stderr: %q", stdout.String(), stderr.String())
}

func TestIntegrationReadFromSession(t *testing.T) {
	server := startTestSSHServer(t, "testpass", nil, true)
	a := makeTestArgs(server, "testuser", "testpass")

	session, err := connect(a)
	if err != nil {
		t.Fatalf("连接失败: %v", err)
	}
	defer session.Close()

	// 使用 StdoutPipe 读取输出
	stdout, err := session.StdoutPipe()
	if err != nil {
		t.Fatalf("获取 stdout pipe 失败: %v", err)
	}

	if err := session.Run("echo pipe-test"); err != nil {
		t.Fatalf("执行命令失败: %v", err)
	}

	data, err := io.ReadAll(stdout)
	if err != nil {
		t.Fatalf("读取输出失败: %v", err)
	}

	if len(data) == 0 {
		t.Fatal("从 stdout pipe 读取的数据为空")
	}
	t.Logf("pipe 输出: %q", string(data))
}

// =============================================================================
// Agent Forwarding 集成测试
// =============================================================================

func TestIntegrationAgentForwardVerbose(t *testing.T) {
	// 创建 fake SSH agent socket
	tmpDir := t.TempDir()
	socketPath := filepath.Join(tmpDir, "agent.sock")

	agentListener, err := net.Listen("unix", socketPath)
	if err != nil {
		t.Fatalf("创建 agent socket 失败: %v", err)
	}
	defer agentListener.Close()

	go func() {
		conn, _ := agentListener.Accept()
		if conn != nil {
			agent.ServeAgent(agent.NewKeyring(), conn)
		}
	}()

	origSock := os.Getenv("SSH_AUTH_SOCK")
	os.Setenv("SSH_AUTH_SOCK", socketPath)
	defer func() {
		if origSock != "" {
			os.Setenv("SSH_AUTH_SOCK", origSock)
		} else {
			os.Unsetenv("SSH_AUTH_SOCK")
		}
	}()

	// 启动测试 SSH 服务器
	server := startTestSSHServer(t, "testpass", nil, true)
	a := makeTestArgs(server, "testuser", "testpass")
	a.agentForward = true
	a.verbose = true

	// 捕获 stderr
	origStderr := os.Stderr
	r, w, _ := os.Pipe()
	os.Stderr = w

	// 建立连接并启用 agent forwarding
	session, err := connect(a)
	if err != nil {
		w.Close()
		os.Stderr = origStderr
		t.Fatalf("连接失败: %v", err)
	}

	// setupAgentForwarding 需要在 session 上调用
	// 这里直接调用来验证 verbose 输出
	af, cleanup, err := setupAgentForwarding(session.client, session.Session, a.verbose)
	if err != nil {
		t.Logf("setupAgentForwarding 返回错误（可能因为测试服务器不支持 agent 转发）: %v", err)
	}
	if cleanup != nil {
		defer cleanup()
	}
	if af != nil {
		t.Log("agent forwarding 成功建立")
	}

	w.Close()
	os.Stderr = origStderr

	var buf bytes.Buffer
	buf.ReadFrom(r)
	output := buf.String()

	if !strings.Contains(output, "Agent forwarding enabled.") {
		t.Errorf("verbose 输出应包含 'Agent forwarding enabled.'，实际: %q", output)
	} else {
		t.Logf("verbose 输出正确: %q", output)
	}

	session.Close()
}

func TestIntegrationAgentForwardEqualsSyntax(t *testing.T) {
	// --agent-forward=true 通过 parseArgsWithConfig 应该报错
	_, err := parseArgsWithConfig([]string{"-u", "root", "--agent-forward=true", "host"})
	if err == nil {
		t.Fatal("--agent-forward=true 应该报错")
	}
	if !strings.Contains(err.Error(), "unknown option") {
		t.Errorf("error = %q, 应该包含 'unknown option'", err.Error())
	}
}

func TestIntegrationAgentForwardDuplicate(t *testing.T) {
	// -A -A 通过 parseArgsWithConfig 不应报错
	a, err := parseArgsWithConfig([]string{"-u", "root", "-A", "-A", "host"})
	if err != nil {
		t.Fatalf("重复 -A 不应报错: %v", err)
	}
	if !a.agentForward {
		t.Error("agentForward should be true")
	}
}

func TestIntegrationAgentForwardDuplicateMixed(t *testing.T) {
	// -A 和 --agent-forward 混合通过 parseArgsWithConfig 不应报错
	a, err := parseArgsWithConfig([]string{"-u", "root", "-A", "--agent-forward", "host"})
	if err != nil {
		t.Fatalf("混合使用不应报错: %v", err)
	}
	if !a.agentForward {
		t.Error("agentForward should be true")
	}
}
