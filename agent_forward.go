package main

import (
	"fmt"
	"net"
	"os"

	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/agent"
)

// agentForwarder 封装 SSH Agent 转发所需的资源
type agentForwarder struct {
	agentClient agent.ExtendedAgent
	agentConn   net.Conn
}

// setupAgentForwarding 连接本地 SSH Agent 并在 session 上启用 agent 转发。
// 返回 agentForwarder 和 cleanup 函数；caller 必须在连接关闭前调用 cleanup。
// 如果 verbose 为 true，会输出诊断信息。
func setupAgentForwarding(client *ssh.Client, session *ssh.Session, verbose bool) (*agentForwarder, func(), error) {
	// 连接本地 SSH Agent
	socket := os.Getenv("SSH_AUTH_SOCK")
	if socket == "" {
		return nil, nil, fmt.Errorf("SSH_AUTH_SOCK not set, cannot forward agent")
	}

	agentConn, err := net.Dial("unix", socket)
	if err != nil {
		return nil, nil, fmt.Errorf("connect SSH agent: %w", err)
	}

	agentClient := agent.NewClient(agentConn)

	// 请求服务端启用 agent forwarding
	if err := agent.RequestAgentForwarding(session); err != nil {
		agentConn.Close()
		return nil, nil, fmt.Errorf("request agent forwarding: %w", err)
	}

	// 启动 goroutine 处理服务端发来的 agent channel 请求
	go agent.ForwardToAgent(client, agentClient)

	if verbose {
		fmt.Fprintln(os.Stderr, "[sshell] Agent forwarding enabled.")
	}

	cleanup := func() {
		agentConn.Close()
	}

	return &agentForwarder{
		agentClient: agentClient,
		agentConn:   agentConn,
	}, cleanup, nil
}
