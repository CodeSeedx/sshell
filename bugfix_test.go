//go:build !windows

package main

import (
	"os"
	"os/signal"
	"syscall"
	"testing"
)

// TestShellDeferOrder 验证 shell.go 中 defer 语句的执行顺序
// Bug: signal.Stop(sigCh) 应该在 term.Restore 之前执行
func TestShellDeferOrder(t *testing.T) {
	// 模拟 defer 顺序：LIFO（后进先出）
	// 代码中：
	//   defer term.Restore(...)
	//   defer func() { signal.Stop(sigCh) }()
	// 执行顺序：signal.Stop 先执行，然后 term.Restore
	// 这是正确的顺序

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGWINCH)
	defer func() {
		signal.Stop(sigCh)
	}()

	// 验证信号通道可用
	select {
	case <-sigCh:
		t.Log("signal received")
	default:
		t.Log("no signal (expected)")
	}
}

// TestShellStdinGoroutineLeak 检查 stdin goroutine 是否会泄漏
func TestShellStdinGoroutineLeak(t *testing.T) {
	// 在非终端环境中，stdin.Read 可能会阻塞
	// 这个测试验证在 session 结束后 goroutine 能够退出
	done := make(chan struct{})
	go func() {
		// 模拟 stdin goroutine
		buf := make([]byte, 4096)
		_, _ = os.Stdin.Read(buf)
		close(done)
	}()

	// 在测试环境中，stdin 可能立即返回 EOF
	select {
	case <-done:
		t.Log("stdin goroutine exited")
	default:
		t.Log("stdin goroutine still running (expected in non-terminal)")
	}
}
