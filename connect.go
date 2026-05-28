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

func connect(a args) (*ssh.Session, error) {
	addr := net.JoinHostPort(a.host, fmt.Sprintf("%d", a.port))
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
	authMethods, err := findAuth(a)
	if err != nil {
		conn.Close()
		return nil, fmt.Errorf("auth: %w", err)
	}

	// 主机密钥校验
	hostKeyCallback, err := loadKnownHosts(a.verbose)
	if err != nil {
		fmt.Fprintf(os.Stderr, "[sshell] Warning: %v\n", err)
		fmt.Fprintln(os.Stderr, "[sshell] Falling back to insecure host key check")
		hostKeyCallback = ssh.InsecureIgnoreHostKey()
	}

	config := &ssh.ClientConfig{
		User:            a.user,
		Auth:            authMethods,
		HostKeyCallback: hostKeyCallback,
		Timeout:         10 * time.Second,
	}

	// SSH 握手
	sshConn, chans, reqs, err := ssh.NewClientConn(conn, addr, config)
	if err != nil {
		conn.Close()
		return nil, fmt.Errorf("ssh handshake: %w", err)
	}

	client := ssh.NewClient(sshConn, chans, reqs)
	if a.verbose {
		fmt.Fprintln(os.Stderr, "[sshell] Handshake OK.")
	}

	session, err := client.NewSession()
	if err != nil {
		client.Close()
		return nil, fmt.Errorf("session: %w", err)
	}

	return session, nil
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
