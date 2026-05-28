package main

import (
	"fmt"
	"io"
	"os"
	"os/signal"
	"syscall"

	"golang.org/x/crypto/ssh"
	"golang.org/x/term"
)

func interactiveShell(session *ssh.Session, a args) error {
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

	stdin, _ := session.StdinPipe()
	stdout, _ := session.StdoutPipe()
	stderr, _ := session.StderrPipe()

	if err := session.Shell(); err != nil {
		return fmt.Errorf("shell: %w", err)
	}
	if a.verbose {
		fmt.Fprintln(os.Stderr, "[sshell] Shell started.")
	}

	// 保存并设置原始模式
	oldState, err := term.MakeRaw(int(os.Stdin.Fd()))
	if err != nil {
		return fmt.Errorf("raw mode: %w", err)
	}
	defer term.Restore(int(os.Stdin.Fd()), oldState)

	// 信号处理：窗口 resize 和中断
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGWINCH, syscall.SIGINT, syscall.SIGTERM)

	// 转发 stdout
	outDone := make(chan struct{})
	go func() {
		io.Copy(os.Stdout, stdout)
		close(outDone)
	}()

	// 转发 stderr
	errDone := make(chan struct{})
	go func() {
		io.Copy(os.Stderr, stderr)
		close(errDone)
	}()

	// 转发 stdin
	go func() {
		buf := make([]byte, 4096)
		for {
			n, err := os.Stdin.Read(buf)
			if n > 0 {
				stdin.Write(buf[:n])
			}
			if err != nil {
				return
			}
		}
	}()

	// 处理信号
	go func() {
		for sig := range sigCh {
			switch sig {
			case syscall.SIGWINCH:
				w, h := getTerminalSize()
				session.WindowChange(h, w)
				if a.verbose {
					fmt.Fprintf(os.Stderr, "[sshell] Window resize: %dx%d\n", w, h)
				}
			case syscall.SIGINT, syscall.SIGTERM:
				session.Signal(ssh.SIGINT)
			}
		}
	}()

	// 等待输出结束
	<-outDone
	<-errDone
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
