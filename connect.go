package main

import (
	"fmt"
	"net"
	"os"
	"path/filepath"
	"time"

	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/knownhosts"
)

// sshConn 封装 session 和 client，确保两者都能被正确关闭
type sshConn struct {
	*ssh.Session
	client  *ssh.Client
	onClose func() error // 可选的额外清理回调
}

func (c *sshConn) Close() error {
	var firstErr error

	if c.Session != nil {
		if err := c.Session.Close(); err != nil {
			firstErr = err
		}
	}
	if err := c.client.Close(); err != nil && firstErr == nil {
		firstErr = err
	}
	if c.onClose != nil {
		if err := c.onClose(); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	return firstErr
}

// connectClient 只建立 SSH 客户端连接，不创建 session
// 用于 SCP、端口转发等不需要 session 的场景
func connectClient(a args) (*sshConn, error) {
	if a.proxyJump != "" {
		if len(a.proxyJumps) > 1 {
			return connectViaMultiJumpClient(a)
		}
		return connectViaJumpClient(a)
	}
	return connectDirectClient(a)
}

// dialSSH 建立 TCP 连接并完成 SSH 握手，返回 ssh.Client
func dialSSH(a args, addr string) (*ssh.Client, error) {
	if a.verbose {
		fmt.Fprintf(os.Stderr, "[sshell] Connecting to %s...\n", addr)
	}

	// TCP 连接
	dialer := net.Dialer{Timeout: 30 * time.Second}
	conn, err := dialer.Dial("tcp", addr)
	if err != nil {
		return nil, fmt.Errorf("connect: %w", err)
	}

	// TCP Keep-Alive
	if tcpConn, ok := conn.(*net.TCPConn); ok {
		tcpConn.SetKeepAlive(true)
		tcpConn.SetKeepAlivePeriod(time.Duration(a.alive) * time.Second)
	}

	// 认证
	authMethods, authCleanup, err := findAuth(a)
	if err != nil {
		conn.Close()
		return nil, fmt.Errorf("auth: %w", err)
	}
	defer authCleanup()

	// 主机密钥校验
	hostKeyCallback, err := loadKnownHosts(a.verbose)
	if err != nil {
		if a.insecureHostKey {
			fmt.Fprintf(os.Stderr, "[sshell] Warning: %v\n", err)
			fmt.Fprintln(os.Stderr, "[sshell] WARNING: Using INSECURE host key check. Connections are vulnerable to MITM attacks.")
			hostKeyCallback = ssh.InsecureIgnoreHostKey()
		} else {
			conn.Close()
			return nil, fmt.Errorf("host key verification failed: %w (use --insecure-host-key to override)", err)
		}
	}

	config := buildSSHConfig(a, authMethods, hostKeyCallback)

	// SSH 握手
	sshConnRaw, chans, reqs, err := ssh.NewClientConn(conn, addr, config)
	if err != nil {
		conn.Close()
		return nil, fmt.Errorf("ssh handshake: %w", err)
	}

	client := ssh.NewClient(sshConnRaw, chans, reqs)
	if a.verbose {
		fmt.Fprintln(os.Stderr, "[sshell] Handshake OK.")
	}
	return client, nil
}

// connectDirectClient 直连目标主机，不创建 session
func connectDirectClient(a args) (*sshConn, error) {
	addr := net.JoinHostPort(a.host, fmt.Sprintf("%d", a.port))
	client, err := dialSSH(a, addr)
	if err != nil {
		return nil, err
	}
	return &sshConn{client: client}, nil
}

// connectViaJumpClient 通过跳板机连接，不创建 session
func connectViaJumpClient(a args) (*sshConn, error) {
	if a.verbose {
		fmt.Fprintf(os.Stderr, "[sshell] Connecting via jump host %s...\n", a.proxyJump)
	}

	jumpArgs := buildJumpArgs(a)
	jumpConn, err := connectDirectClient(jumpArgs)
	if err != nil {
		return nil, fmt.Errorf("jump host: %w", err)
	}

	destAddr := net.JoinHostPort(a.host, fmt.Sprintf("%d", a.port))
	client, err := dialSSHVia(jumpConn.client, a, destAddr)
	if err != nil {
		jumpConn.Close()
		return nil, err
	}

	jumpClient := jumpConn.client
	return &sshConn{
		client:  client,
		onClose: func() error { return jumpClient.Close() },
	}, nil
}

func connect(a args) (*sshConn, error) {
	// 如果有 ProxyJump，走跳板机连接
	if a.proxyJump != "" {
		if len(a.proxyJumps) > 1 {
			return connectViaMultiJump(a)
		}
		return connectViaJump(a)
	}

	return connectDirect(a)
}

// connectDirect 直连目标主机
func connectDirect(a args) (*sshConn, error) {
	addr := net.JoinHostPort(a.host, fmt.Sprintf("%d", a.port))
	client, err := dialSSH(a, addr)
	if err != nil {
		return nil, err
	}
	return newSession(client, a)
}

// connectViaJump 通过 ProxyJump 跳板机连接（带 session）
func connectViaJump(a args) (*sshConn, error) {
	if a.verbose {
		fmt.Fprintf(os.Stderr, "[sshell] Connecting via jump host %s...\n", a.proxyJump)
	}

	jumpArgs := buildJumpArgs(a)
	jumpConn, err := connectDirectClient(jumpArgs)
	if err != nil {
		return nil, fmt.Errorf("jump host: %w", err)
	}

	destAddr := net.JoinHostPort(a.host, fmt.Sprintf("%d", a.port))
	client, err := dialSSHVia(jumpConn.client, a, destAddr)
	if err != nil {
		jumpConn.Close()
		return nil, err
	}

	if a.verbose {
		fmt.Fprintln(os.Stderr, "[sshell] Connected via jump host.")
	}

	session, err := client.NewSession()
	if err != nil {
		// 关闭目标 client（会关闭底层连接）
		client.Close()
		// 显式关闭 jumpConn，确保跳板机连接被清理
		jumpConn.Close()
		return nil, fmt.Errorf("session: %w", err)
	}
	jumpClient := jumpConn.client
	return &sshConn{
		Session: session,
		client:  client,
		onClose: func() error { return jumpClient.Close() },
	}, nil
}

// buildJumpArgs 从当前 args 构建跳板机连接参数
func buildJumpArgs(a args) args {
	jumpUser := a.user
	if a.proxyJumpUser != "" {
		jumpUser = a.proxyJumpUser
	}

	jumpArgs := args{
		host:            a.proxyJump,
		port:            22,
		user:            jumpUser,
		auth:            a.auth,
		alive:           a.alive,
		verbose:         a.verbose,
		noAgent:         a.noAgent,
		compress:        a.compress,
		agentForward:    a.agentForward,
		insecureHostKey: a.insecureHostKey,
	}

	// 如果命令行指定了端口（-J host:port），优先使用
	if a.proxyJumpPort != 0 {
		jumpArgs.port = a.proxyJumpPort
	}

	if cfg := loadSSHConfig(a.proxyJump); cfg != nil {
		if cfg.HostName != "" {
			jumpArgs.host = cfg.HostName
		}
		if cfg.Port != 0 && jumpArgs.port == 22 {
			jumpArgs.port = cfg.Port
		}
		if cfg.User != "" {
			jumpArgs.user = cfg.User
		}
		if cfg.IdentityFile != "" {
			jumpArgs.auth = cfg.IdentityFile
		}
	}
	return jumpArgs
}

// dialSSHVia 通过已有的 SSH client 连接目标主机
func dialSSHVia(via *ssh.Client, a args, destAddr string) (*ssh.Client, error) {
	if a.verbose {
		fmt.Fprintf(os.Stderr, "[sshell] Dialing %s via jump host...\n", destAddr)
	}

	conn, err := via.Dial("tcp", destAddr)
	if err != nil {
		return nil, fmt.Errorf("dial through jump: %w", err)
	}

	authMethods, authCleanup, err := findAuth(a)
	if err != nil {
		conn.Close()
		return nil, fmt.Errorf("auth: %w", err)
	}
	defer authCleanup()

	hostKeyCallback, err := loadKnownHosts(a.verbose)
	if err != nil {
		if a.insecureHostKey {
			fmt.Fprintf(os.Stderr, "[sshell] Warning: %v\n", err)
			fmt.Fprintln(os.Stderr, "[sshell] WARNING: Using INSECURE host key check.")
			hostKeyCallback = ssh.InsecureIgnoreHostKey()
		} else {
			conn.Close()
			return nil, fmt.Errorf("host key verification failed: %w (use --insecure-host-key to override)", err)
		}
	}

	config := buildSSHConfig(a, authMethods, hostKeyCallback)

	sshConnRaw, chans, reqs, err := ssh.NewClientConn(conn, destAddr, config)
	if err != nil {
		conn.Close()
		return nil, fmt.Errorf("ssh handshake: %w", err)
	}

	client := ssh.NewClient(sshConnRaw, chans, reqs)
	if a.verbose {
		fmt.Fprintln(os.Stderr, "[sshell] Handshake OK.")
	}
	return client, nil
}

// connectViaMultiJump 通过多个跳板机链式连接到目标主机（带 session）
func connectViaMultiJump(a args) (*sshConn, error) {
	if a.verbose {
		fmt.Fprintf(os.Stderr, "[sshell] Connecting via %d jump hosts...\n", len(a.proxyJumps))
	}

	// 建立跳板机链
	clients, err := buildJumpChain(a)
	if err != nil {
		return nil, err
	}

	// 通过最后一个跳板机连接到目标主机
	lastClient := clients[len(clients)-1]
	destAddr := net.JoinHostPort(a.host, fmt.Sprintf("%d", a.port))
	client, err := dialSSHVia(lastClient, a, destAddr)
	if err != nil {
		closeJumpChain(clients)
		return nil, err
	}

	if a.verbose {
		fmt.Fprintln(os.Stderr, "[sshell] Connected via multi-jump chain.")
	}

	session, err := client.NewSession()
	if err != nil {
		client.Close()
		closeJumpChain(clients)
		return nil, fmt.Errorf("session: %w", err)
	}

	return &sshConn{
		Session: session,
		client:  client,
		onClose: func() error { closeJumpChain(clients); return nil },
	}, nil
}

// connectViaMultiJumpClient 通过多个跳板机链式连接到目标主机（不带 session）
func connectViaMultiJumpClient(a args) (*sshConn, error) {
	if a.verbose {
		fmt.Fprintf(os.Stderr, "[sshell] Connecting via %d jump hosts...\n", len(a.proxyJumps))
	}

	// 建立跳板机链
	clients, err := buildJumpChain(a)
	if err != nil {
		return nil, err
	}

	// 通过最后一个跳板机连接到目标主机
	lastClient := clients[len(clients)-1]
	destAddr := net.JoinHostPort(a.host, fmt.Sprintf("%d", a.port))
	client, err := dialSSHVia(lastClient, a, destAddr)
	if err != nil {
		closeJumpChain(clients)
		return nil, err
	}

	return &sshConn{
		client:  client,
		onClose: func() error { closeJumpChain(clients); return nil },
	}, nil
}

// buildJumpChain 建立跳板机链，返回所有跳板机的 ssh.Client
func buildJumpChain(a args) ([]*ssh.Client, error) {
	var clients []*ssh.Client

	for i, jh := range a.proxyJumps {
		jumpArgs := buildJumpHostArgs(a, jh)

		var client *ssh.Client
		port := jh.Port
		if port == 0 {
			port = 22
		}
		if i == 0 {
			// 第一个跳板机：直连
			addr := net.JoinHostPort(jh.Host, fmt.Sprintf("%d", port))
			var err error
			client, err = dialSSH(jumpArgs, addr)
			if err != nil {
				return nil, fmt.Errorf("jump host %d (%s): %w", i+1, jh.Host, err)
			}
		} else {
			// 后续跳板机：通过前一个跳板机连接
			prevClient := clients[i-1]
			addr := net.JoinHostPort(jh.Host, fmt.Sprintf("%d", port))
			var err error
			client, err = dialSSHVia(prevClient, jumpArgs, addr)
			if err != nil {
				closeJumpChain(clients)
				return nil, fmt.Errorf("jump host %d (%s): %w", i+1, jh.Host, err)
			}
		}

		clients = append(clients, client)
		if a.verbose {
			fmt.Fprintf(os.Stderr, "[sshell] Jump %d/%d: connected to %s\n", i+1, len(a.proxyJumps), jh.Host)
		}
	}

	return clients, nil
}

// closeJumpChain 关闭跳板机链中的所有连接
func closeJumpChain(clients []*ssh.Client) {
	// 从后往前关闭，避免中间节点断开导致后续关闭失败
	for i := len(clients) - 1; i >= 0; i-- {
		clients[i].Close()
	}
}

// buildJumpHostArgs 为单个跳板机构建连接参数
func buildJumpHostArgs(a args, jh jumpHost) args {
	jumpUser := a.user
	if jh.User != "" {
		jumpUser = jh.User
	}

	port := jh.Port
	if port == 0 {
		port = 22
	}

	jumpArgs := args{
		host:            jh.Host,
		port:            port,
		user:            jumpUser,
		auth:            a.auth,
		alive:           a.alive,
		verbose:         a.verbose,
		noAgent:         a.noAgent,
		insecureHostKey: a.insecureHostKey,
		compress:        a.compress,
		agentForward:    a.agentForward,
	}

	// 应用 SSH config
	if cfg := loadSSHConfig(jh.Host); cfg != nil {
		if cfg.HostName != "" {
			jumpArgs.host = cfg.HostName
		}
		if cfg.Port != 0 && jumpArgs.port == 22 {
			jumpArgs.port = cfg.Port
		}
		if cfg.User != "" && jh.User == "" {
			jumpArgs.user = cfg.User
		}
		if cfg.IdentityFile != "" {
			jumpArgs.auth = cfg.IdentityFile
		}
	}

	return jumpArgs
}

// connectWithRetry 带重连的连接（带 session）
func connectWithRetry(a args) (*sshConn, error) {
	if !a.reconnect {
		return connect(a)
	}

	maxAttempts := a.reconnectMax
	if maxAttempts <= 0 {
		maxAttempts = 3
	}

	var lastErr error
	var attempts []error // 记录所有尝试的错误
	for attempt := 1; attempt <= maxAttempts; attempt++ {
		conn, err := connect(a)
		if err == nil {
			return conn, nil
		}
		lastErr = err
		attempts = append(attempts, err)
		if attempt < maxAttempts {
			if a.verbose {
				fmt.Fprintf(os.Stderr, "[sshell] Reconnect attempt %d/%d after error: %v\n", attempt, maxAttempts, err)
			}
			time.Sleep(exponentialBackoff(attempt)) // 指数退避: 1s, 2s, 4s, max 60s
		}
	}
	
	// 返回包含所有尝试错误的详细信息
	if len(attempts) > 1 {
		return nil, fmt.Errorf("reconnect failed after %d attempts, last error: %w", len(attempts), lastErr)
	}
	return nil, fmt.Errorf("reconnect failed: %w", lastErr)
}

// connectClientWithRetry 带重连的连接（不带 session，用于 SCP/转发模式）
func connectClientWithRetry(a args) (*sshConn, error) {
	if !a.reconnect {
		return connectClient(a)
	}

	maxAttempts := a.reconnectMax
	if maxAttempts <= 0 {
		maxAttempts = 3
	}

	var lastErr error
	for attempt := 1; attempt <= maxAttempts; attempt++ {
		conn, err := connectClient(a)
		if err == nil {
			return conn, nil
		}
		lastErr = err
		if attempt < maxAttempts {
			if a.verbose {
				fmt.Fprintf(os.Stderr, "[sshell] Reconnect attempt %d/%d after error: %v\n", attempt, maxAttempts, err)
			}
			time.Sleep(exponentialBackoff(attempt)) // 指数退避: 1s, 2s, 4s, max 60s
		}
	}
	return nil, fmt.Errorf("reconnect failed: %w", lastErr)
}

// exponentialBackoff 计算指数退避时间，上限 60 秒
func exponentialBackoff(attempt int) time.Duration {
	if attempt < 1 {
		attempt = 1
	}
	// 限制移位量避免整数溢出（2^6 = 64 > 60，所以 7 足够）
	shift := attempt - 1
	if shift > 6 {
		shift = 6
	}
	backoff := time.Duration(1<<uint(shift)) * time.Second
	if backoff > 60*time.Second {
		backoff = 60 * time.Second
	}
	return backoff
}

// buildSSHConfig 构建 SSH 客户端配置
func buildSSHConfig(a args, authMethods []ssh.AuthMethod, hostKeyCallback ssh.HostKeyCallback) *ssh.ClientConfig {
	config := &ssh.ClientConfig{
		User:            a.user,
		Auth:            authMethods,
		HostKeyCallback: hostKeyCallback,
		Timeout:         10 * time.Second,
	}

	// 注意：Go 的 x/crypto/ssh 库目前不支持压缩协商。
	// -C 标志已记录但需要底层库支持才能生效。
	// OpenSSH 服务端会根据客户端能力自动协商压缩。

	return config
}

// newSession 创建新的 SSH session
func newSession(client *ssh.Client, a args) (*sshConn, error) {
	session, err := client.NewSession()
	if err != nil {
		client.Close()
		return nil, fmt.Errorf("session: %w", err)
	}
	return &sshConn{Session: session, client: client}, nil
}

// loadKnownHosts 加载 known_hosts 文件进行主机密钥校验
func loadKnownHosts(verbose bool) (ssh.HostKeyCallback, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("cannot get home dir: %w", err)
	}

	knownHostsPath := filepath.Join(home, ".ssh", "known_hosts")
	callback, err := knownhosts.New(knownHostsPath)
	if err != nil {
		return nil, fmt.Errorf("cannot load known_hosts: %w", err)
	}

	if verbose {
		fmt.Fprintf(os.Stderr, "[sshell] Host key check: %s\n", knownHostsPath)
	}
	return callback, nil
}
