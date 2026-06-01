package main

import (
	"fmt"
	"io"
	"os"
	"os/signal"
	"time"

	"golang.org/x/crypto/ssh"
	"golang.org/x/term"
)

func interactiveShell(session *ssh.Session, client *ssh.Client, a args) error {
	// 获取终端尺寸
	width, height := getTerminalSize()

	modes := ssh.TerminalModes{
		ssh.ECHO:          1,
		ssh.TTY_OP_ISPEED: 14400,
		ssh.TTY_OP_OSPEED: 14400,
	}
	if err := session.RequestPty("xterm-256color", height, width, modes); err != nil {
		return fmt.Errorf("pty: %w", err)
	}

	stdin, err := session.StdinPipe()
	if err != nil {
		return fmt.Errorf("stdin pipe: %w", err)
	}
	stdout, err := session.StdoutPipe()
	if err != nil {
		return fmt.Errorf("stdout pipe: %w", err)
	}
	stderr, err := session.StderrPipe()
	if err != nil {
		return fmt.Errorf("stderr pipe: %w", err)
	}

	if err := session.Shell(); err != nil {
		return fmt.Errorf("shell: %w", err)
	}
	if a.verbose {
		fmt.Fprintln(os.Stderr, "[sshell] Shell started.")
	}

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

	// 保存并设置原始模式
	oldState, err := term.MakeRaw(int(os.Stdin.Fd()))
	if err != nil {
		return fmt.Errorf("raw mode: %w", err)
	}
	defer term.Restore(int(os.Stdin.Fd()), oldState)

	// 信号处理：窗口 resize 和中断
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, sigResize, sigInterrupt, sigTerminate)
	sigDone := make(chan struct{})
	defer func() {
		signal.Stop(sigCh)
		close(sigDone)
	}()

	// 日志
	var logger *sessionLogger
	if a.logFile != "" {
		logger, err = newSessionLogger(a.logFile, a.verbose)
		if err != nil {
			return fmt.Errorf("log: %w", err)
		}
		defer logger.Close()
	}

	// 转发 stdout
	outTarget := io.Writer(os.Stdout)
	if logger != nil {
		outTarget = logger.WrapWriter(os.Stdout)
	}
	outDone := make(chan struct{})
	go func() {
		io.Copy(outTarget, stdout)
		close(outDone)
	}()

	// 转发 stderr
	errTarget := io.Writer(os.Stderr)
	if logger != nil {
		errTarget = logger.WrapWriter(os.Stderr)
	}
	errDone := make(chan struct{})
	go func() {
		io.Copy(errTarget, stderr)
		close(errDone)
	}()

	// 转发 stdin
	stdinDone := make(chan struct{})
	go func() {
		defer close(stdinDone)
		buf := make([]byte, 4096)
		for {
			n, err := os.Stdin.Read(buf)
			if n > 0 {
				if _, writeErr := stdin.Write(buf[:n]); writeErr != nil {
					return
				}
			}
			if err != nil {
				return
			}
		}
	}()

	// 处理信号
	go func() {
		for {
			select {
			case sig, ok := <-sigCh:
				if !ok {
					return
				}
				switch sig {
				case sigResize:
					w, h := getTerminalSize()
					session.WindowChange(h, w)
					if a.verbose {
						fmt.Fprintf(os.Stderr, "[sshell] Window resize: %dx%d\n", w, h)
					}
				case sigInterrupt, sigTerminate:
					session.Signal(ssh.SIGINT)
				}
			case <-sigDone:
				return
			}
		}
	}()

	// 等待输出结束
	<-outDone
	<-errDone

	// 关闭 stdin 管道，让 stdin goroutine 收到写入错误后退出
	stdin.Close()
	// stdin goroutine 可能阻塞在 os.Stdin.Read（无输入时），
	// 但 session 已结束，outDone/errDone 已关闭，所以可以安全忽略。
	// 恢复终端以解除可能的 Read 阻塞
	term.Restore(int(os.Stdin.Fd()), oldState)

	// 等待 stdin goroutine 退出或超时
	select {
	case <-stdinDone:
		// stdin goroutine 已退出
	case <-time.After(100 * time.Millisecond):
		// 超时，stdin goroutine 可能阻塞在 Read，忽略
		if a.verbose {
			fmt.Fprintln(os.Stderr, "[sshell] stdin goroutine timeout, ignoring")
		}
	}

	// 恢复终端后再等待 session，避免 Wait 输出乱码
	return session.Wait()
}

// getTerminalSize 获取当前终端尺寸
func getTerminalSize() (int, int) {
	w, h, err := term.GetSize(int(os.Stdout.Fd()))
	if err != nil {
		// 获取失败时使用默认值
		return 80, 24
	}
	return w, h
}
