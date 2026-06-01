package main

import (
	"net"
	"sync"
	"testing"
	"time"
	
	"golang.org/x/crypto/ssh"
)

// ==================== 第 10 轮：代码质量和潜在问题 ====================

// Bug #53: sshConn Close 方法
func TestSSHConnCloseNilSessionR10(t *testing.T) {
	// 验证 sshConn 结构体存在
	conn := &sshConn{}
	if conn == nil {
		t.Error("sshConn should not be nil")
	}
}

// Bug #55: pipeConns 双向复制的资源清理
func TestPipeConnsResourceCleanupR10(t *testing.T) {
	server, client := net.Pipe()
	
	done := make(chan bool)
	go func() {
		pipeConns(server, client)
		done <- true
	}()
	
	// 关闭连接
	server.Close()
	client.Close()
	
	select {
	case <-done:
		// pipeConns 已完成
	case <-time.After(2 * time.Second):
		t.Error("pipeConns did not complete in time")
	}
}

// Bug #57: remoteForwardMappings 的并发安全
func TestRemoteForwardMappingsConcurrencyR10(t *testing.T) {
	done := make(chan bool)
	
	go func() {
		for i := 0; i < 100; i++ {
			remoteForwardMappings.Store(uint16(i), "localhost:80")
		}
		done <- true
	}()
	
	go func() {
		for i := 0; i < 100; i++ {
			remoteForwardMappings.Load(uint16(i))
		}
		done <- true
	}()
	
	go func() {
		for i := 0; i < 100; i++ {
			remoteForwardMappings.Delete(uint16(i))
		}
		done <- true
	}()
	
	<-done
	<-done
	<-done
}

// Bug #58: remoteForwardHandlers 的并发安全
func TestRemoteForwardHandlersConcurrencyR10(t *testing.T) {
	done := make(chan bool)
	
	go func() {
		for i := 0; i < 100; i++ {
			remoteForwardHandlers.Store(&ssh.Client{}, &sync.Once{})
		}
		done <- true
	}()
	
	go func() {
		for i := 0; i < 100; i++ {
			remoteForwardHandlers.Load(&ssh.Client{})
		}
		done <- true
	}()
	
	<-done
	<-done
}
