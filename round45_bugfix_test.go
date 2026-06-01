package main

import (
	"net"
	"sync"
	"testing"
	"time"
)

// TestChannelConnResetTimerExpiredDeadlineRacedWithNewDeadline
// 验证过期 deadline 的 goroutine 不会错误关闭被新 SetDeadline 更新后的 channel
func TestChannelConnResetTimerExpiredDeadlineRacedWithNewDeadline(t *testing.T) {
	mock := &mockChannel{}
	cc := &channelConn{
		Channel:    mock,
		remoteAddr: &net.TCPAddr{},
	}

	// 设置一个已过期的 deadline（触发 d <= 0 分支）
	cc.SetDeadline(time.Now().Add(-1 * time.Second))

	// 立即设置一个新的未来 deadline（过期 goroutine 还未获得锁）
	cc.SetDeadline(time.Now().Add(10 * time.Second))

	// 等待足够时间，让过期 goroutine 有机会执行
	time.Sleep(100 * time.Millisecond)

	// channel 应该仍然开放（未被过期 goroutine 关闭）
	cc.mu.Lock()
	closed := cc.closed
	cc.mu.Unlock()

	if closed {
		t.Error("channel was incorrectly closed by expired deadline goroutine after SetDeadline updated deadline")
	}

	// 清理
	cc.Close()
}

// TestChannelConnResetTimerTimerCallbackRacedWithNewDeadline
// 验证 timer 回调在 deadline 被新 SetDeadline 修改后不会错误关闭 channel
func TestChannelConnResetTimerTimerCallbackRacedWithNewDeadline(t *testing.T) {
	mock := &mockChannel{}
	cc := &channelConn{
		Channel:    mock,
		remoteAddr: &net.TCPAddr{},
	}

	// 设置一个即将过期的 deadline（50ms 后）
	cc.SetDeadline(time.Now().Add(50 * time.Millisecond))

	// 在 timer 触发前更新 deadline（到很远的未来）
	time.Sleep(10 * time.Millisecond)
	cc.SetDeadline(time.Now().Add(10 * time.Second))

	// 等待旧 timer 原本应该触发的时间
	time.Sleep(100 * time.Millisecond)

	// channel 应该仍然开放
	cc.mu.Lock()
	closed := cc.closed
	cc.mu.Unlock()

	if closed {
		t.Error("channel was incorrectly closed by timer callback after SetDeadline updated deadline")
	}

	// 清理
	cc.Close()
}

// TestChannelConnDeadlineExpiredThenClose 验证过期 deadline 后正常 close 仍然有效
func TestChannelConnDeadlineExpiredThenClose(t *testing.T) {
	mock := &mockChannel{}
	cc := &channelConn{
		Channel:    mock,
		remoteAddr: &net.TCPAddr{},
	}

	// 设置已过期 deadline
	cc.SetDeadline(time.Now().Add(-1 * time.Second))

	// 等待过期 goroutine 关闭
	time.Sleep(200 * time.Millisecond)

	cc.mu.Lock()
	closed := cc.closed
	cc.mu.Unlock()

	if !closed {
		t.Error("channel should be closed after expired deadline with no subsequent SetDeadline")
	}
}

// TestChannelConnDeadlineTimerExpires 验证 timer 正常过期关闭
func TestChannelConnDeadlineTimerExpires(t *testing.T) {
	mock := &mockChannel{}
	cc := &channelConn{
		Channel:    mock,
		remoteAddr: &net.TCPAddr{},
	}

	// 设置很短的 deadline
	cc.SetDeadline(time.Now().Add(50 * time.Millisecond))

	// 等待 timer 触发
	time.Sleep(200 * time.Millisecond)

	cc.mu.Lock()
	closed := cc.closed
	cc.mu.Unlock()

	if !closed {
		t.Error("channel should be closed after deadline timer expired")
	}
}

// TestChannelConnSetDeadlineClearDeadline 验证 SetDeadline(zero) 清除 deadline
func TestChannelConnSetDeadlineClearDeadline(t *testing.T) {
	mock := &mockChannel{}
	cc := &channelConn{
		Channel:    mock,
		remoteAddr: &net.TCPAddr{},
	}

	// 设置未来 deadline
	cc.SetDeadline(time.Now().Add(1 * time.Second))

	// 立即清除
	cc.SetDeadline(time.Time{})

	// 等待超过原 deadline 时间
	time.Sleep(200 * time.Millisecond)

	cc.mu.Lock()
	closed := cc.closed
	deadlineIsZero := cc.deadline.IsZero()
	cc.mu.Unlock()

	if closed {
		t.Error("channel should not be closed after deadline was cleared")
	}
	if !deadlineIsZero {
		t.Error("deadline should be zero after SetDeadline(zero)")
	}
}

// TestChannelConnResetTimerConcurrentStress 压力测试并发 SetDeadline 和 Close
func TestChannelConnResetTimerConcurrentStress(t *testing.T) {
	mock := &mockChannel{}
	cc := &channelConn{
		Channel:    mock,
		remoteAddr: &net.TCPAddr{},
	}

	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(2)
		go func() {
			defer wg.Done()
			for j := 0; j < 100; j++ {
				cc.SetDeadline(time.Now().Add(time.Duration(j%20-5) * 10 * time.Millisecond))
			}
		}()
		go func() {
			defer wg.Done()
			for j := 0; j < 100; j++ {
				cc.Write([]byte("test"))
				cc.Read(make([]byte, 10))
			}
		}()
	}
	wg.Wait()
	cc.Close()
}
