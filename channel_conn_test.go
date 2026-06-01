package main

import (
	"io"
	"net"
	"sync"
	"testing"
	"time"
)

// TestChannelConnWriteConcurrency 测试 channelConn 的并发写入安全性
func TestChannelConnWriteConcurrency(t *testing.T) {
	// 创建一个 mock channel
	mock := &mockChannel{}
	cc := &channelConn{
		Channel:    mock,
		remoteAddr: &net.TCPAddr{},
	}

	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			cc.Write([]byte("test"))
		}()
	}
	wg.Wait()
}

// TestChannelConnReadConcurrency 测试 channelConn 的并发读取安全性
func TestChannelConnReadConcurrency(t *testing.T) {
	mock := &mockChannel{}
	cc := &channelConn{
		Channel:    mock,
		remoteAddr: &net.TCPAddr{},
	}

	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			buf := make([]byte, 10)
			cc.Read(buf)
		}()
	}
	wg.Wait()
}

// TestChannelConnCloseConcurrency 测试 channelConn 的并发关闭安全性
func TestChannelConnCloseConcurrency(t *testing.T) {
	mock := &mockChannel{}
	cc := &channelConn{
		Channel:    mock,
		remoteAddr: &net.TCPAddr{},
	}

	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			cc.Close()
		}()
	}
	wg.Wait()
}

// TestChannelConnDeadlineAndClose 测试 deadline 和 close 的并发安全性
func TestChannelConnDeadlineAndClose(t *testing.T) {
	mock := &mockChannel{}
	cc := &channelConn{
		Channel:    mock,
		remoteAddr: &net.TCPAddr{},
	}

	var wg sync.WaitGroup
	wg.Add(2)

	// 一个 goroutine 设置 deadline
	go func() {
		defer wg.Done()
		for i := 0; i < 10; i++ {
			cc.SetDeadline(time.Now().Add(time.Second))
		}
	}()

	// 另一个 goroutine 关闭连接
	go func() {
		defer wg.Done()
		time.Sleep(10 * time.Millisecond)
		cc.Close()
	}()

	wg.Wait()
}

// mockChannel 实现 ssh.Channel 接口用于测试
type mockChannel struct {
	closed bool
}

func (m *mockChannel) Read(data []byte) (int, error) {
	return 0, nil
}

func (m *mockChannel) Write(data []byte) (int, error) {
	return len(data), nil
}

func (m *mockChannel) Close() error {
	m.closed = true
	return nil
}

func (m *mockChannel) CloseWrite() error {
	return nil
}

func (m *mockChannel) SendRequest(name string, wantReply bool, payload []byte) (bool, error) {
	return false, nil
}

func (m *mockChannel) Stderr() io.ReadWriter {
	return nil
}
