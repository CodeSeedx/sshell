package main

import (
	"fmt"
	"io"
	"os"

	"golang.org/x/crypto/ssh"
)

// runRemoteCommand 非交互式执行远程命令，返回远程命令的退出码
func runRemoteCommand(session *ssh.Session, client *ssh.Client, a args) (int, error) {
	// Agent Forwarding
	if a.agentForward {
		af, afCleanup, err := setupAgentForwarding(client, session, a.verbose)
		if err != nil {
			fmt.Fprintf(os.Stderr, "[sshell] Agent forwarding failed: %v\n", err)
		} else {
			defer afCleanup()
			_ = af
		}
	}

	// 管道：直接绑定到本地 stdio，不走 PTY
	session.Stdin = os.Stdin
	session.Stdout = os.Stdout
	session.Stderr = os.Stderr

	if a.verbose {
		fmt.Fprintf(os.Stderr, "[sshell] Exec: %s\n", a.cmd)
	}

	err := session.Run(a.cmd)
	if err == nil {
		return 0, nil
	}

	// 提取远程退出码
	if exitErr, ok := err.(*ssh.ExitError); ok {
		return exitErr.ExitStatus(), nil
	}
	
	// 其他错误（网络错误等）返回-1表示未知
	return -1, fmt.Errorf("run: %w", err)
}

// runRemoteCommandIO 带自定义 IO 的远程命令执行（便于测试）
func runRemoteCommandIO(session *ssh.Session, a args, stdin io.Reader, stdout, stderr io.Writer) (int, error) {
	session.Stdin = stdin
	session.Stdout = stdout
	session.Stderr = stderr

	if a.verbose {
		fmt.Fprintf(os.Stderr, "[sshell] Exec: %s\n", a.cmd)
	}

	err := session.Run(a.cmd)
	if err == nil {
		return 0, nil
	}

	if exitErr, ok := err.(*ssh.ExitError); ok {
		return exitErr.ExitStatus(), nil
	}

	// 与 runRemoteCommand 保持一致：未知错误返回 -1
	return -1, fmt.Errorf("run: %w", err)
}
