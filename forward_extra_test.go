package main

import (
	"bytes"
	"net"
	"testing"
	"time"
)

// ==================== Bug #19: channelConn 实现不完整 ====================

func TestChannelConnLocalAddr(t *testing.T) {
	cc := &channelConn{remoteAddr: &net.TCPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 8080}}
	
	localAddr := cc.LocalAddr()
	if localAddr == nil {
		t.Error("LocalAddr() should not return nil")
	}
}

func TestChannelConnRemoteAddr(t *testing.T) {
	remoteAddr := &net.TCPAddr{IP: net.IPv4(192, 168, 1, 1), Port: 22}
	cc := &channelConn{remoteAddr: remoteAddr}
	
	got := cc.RemoteAddr()
	if got != remoteAddr {
		t.Errorf("RemoteAddr() = %v, want %v", got, remoteAddr)
	}
}

func TestChannelConnSetDeadline(t *testing.T) {
	cc := &channelConn{}
	
	err := cc.SetDeadline(time.Now())
	if err != nil {
		t.Errorf("SetDeadline() returned error: %v", err)
	}
}

func TestChannelConnSetReadDeadline(t *testing.T) {
	cc := &channelConn{}
	
	err := cc.SetReadDeadline(time.Now())
	if err != nil {
		t.Errorf("SetReadDeadline() returned error: %v", err)
	}
}

func TestChannelConnSetWriteDeadline(t *testing.T) {
	cc := &channelConn{}
	
	err := cc.SetWriteDeadline(time.Now())
	if err != nil {
		t.Errorf("SetWriteDeadline() returned error: %v", err)
	}
}

// ==================== Bug #20: pipeConns 双向复制 ====================

func TestPipeConnsBasic(t *testing.T) {
	server, client := net.Pipe()
	defer server.Close()
	defer client.Close()
	
	go func() {
		server.Write([]byte("hello"))
		server.Close()
	}()
	
	buf := make([]byte, 10)
	n, err := client.Read(buf)
	if err != nil {
		t.Fatalf("Read failed: %v", err)
	}
	
	if string(buf[:n]) != "hello" {
		t.Errorf("Read data = %q, want 'hello'", string(buf[:n]))
	}
}

// ==================== Bug #21: SOCKS5 常量验证 ====================

func TestSOCKS5Constants(t *testing.T) {
	if socks5Version != 0x05 {
		t.Errorf("socks5Version = 0x%02x, want 0x05", socks5Version)
	}
	if socks5AuthNone != 0x00 {
		t.Errorf("socks5AuthNone = 0x%02x, want 0x00", socks5AuthNone)
	}
	if socks5CmdConnect != 0x01 {
		t.Errorf("socks5CmdConnect = 0x%02x, want 0x01", socks5CmdConnect)
	}
	if socks5AddrTypeIPv4 != 0x01 {
		t.Errorf("socks5AddrTypeIPv4 = 0x%02x, want 0x01", socks5AddrTypeIPv4)
	}
	if socks5AddrTypeDomain != 0x03 {
		t.Errorf("socks5AddrTypeDomain = 0x%02x, want 0x03", socks5AddrTypeDomain)
	}
	if socks5AddrTypeIPv6 != 0x04 {
		t.Errorf("socks5AddrTypeIPv6 = 0x%02x, want 0x04", socks5AddrTypeIPv6)
	}
	if socks5ReplySuccess != 0x00 {
		t.Errorf("socks5ReplySuccess = 0x%02x, want 0x00", socks5ReplySuccess)
	}
	if socks5ReplyGeneralFail != 0x01 {
		t.Errorf("socks5ReplyGeneralFail = 0x%02x, want 0x01", socks5ReplyGeneralFail)
	}
	if socks5ReplyHostUnreach != 0x04 {
		t.Errorf("socks5ReplyHostUnreach = 0x%02x, want 0x04", socks5ReplyHostUnreach)
	}
	if socks5ReplyCmdNotSupp != 0x07 {
		t.Errorf("socks5ReplyCmdNotSupp = 0x%02x, want 0x07", socks5ReplyCmdNotSupp)
	}
}

// ==================== Bug #22: sendSOCKS5Reply 格式验证 ====================

func TestSendSOCKS5ReplyFormatExtended(t *testing.T) {
	var buf bytes.Buffer
	conn := &bufConn{buf: &buf}
	
	err := sendSOCKS5Reply(conn, socks5ReplySuccess)
	if err != nil {
		t.Fatalf("sendSOCKS5Reply failed: %v", err)
	}
	
	reply := buf.Bytes()
	if len(reply) != 10 {
		t.Fatalf("reply length = %d, want 10", len(reply))
	}
	
	if reply[0] != socks5Version {
		t.Errorf("reply[0] = 0x%02x, want 0x%02x", reply[0], socks5Version)
	}
	if reply[1] != socks5ReplySuccess {
		t.Errorf("reply[1] = 0x%02x, want 0x%02x", reply[1], socks5ReplySuccess)
	}
	if reply[2] != 0x00 {
		t.Errorf("reply[2] = 0x%02x, want 0x00", reply[2])
	}
	if reply[3] != socks5AddrTypeIPv4 {
		t.Errorf("reply[3] = 0x%02x, want 0x%02x", reply[3], socks5AddrTypeIPv4)
	}
	for i := 4; i < 8; i++ {
		if reply[i] != 0x00 {
			t.Errorf("reply[%d] = 0x%02x, want 0x00", i, reply[i])
		}
	}
	if reply[8] != 0x00 || reply[9] != 0x00 {
		t.Errorf("BND.PORT = 0x%02x%02x, want 0x0000", reply[8], reply[9])
	}
}

func TestSendSOCKS5ReplyGeneralFail(t *testing.T) {
	var buf bytes.Buffer
	conn := &bufConn{buf: &buf}
	
	err := sendSOCKS5Reply(conn, socks5ReplyGeneralFail)
	if err != nil {
		t.Fatalf("sendSOCKS5Reply failed: %v", err)
	}
	
	reply := buf.Bytes()
	if reply[1] != socks5ReplyGeneralFail {
		t.Errorf("reply[1] = 0x%02x, want 0x%02x", reply[1], socks5ReplyGeneralFail)
	}
}

func TestSendSOCKS5ReplyHostUnreach(t *testing.T) {
	var buf bytes.Buffer
	conn := &bufConn{buf: &buf}
	
	err := sendSOCKS5Reply(conn, socks5ReplyHostUnreach)
	if err != nil {
		t.Fatalf("sendSOCKS5Reply failed: %v", err)
	}
	
	reply := buf.Bytes()
	if reply[1] != socks5ReplyHostUnreach {
		t.Errorf("reply[1] = 0x%02x, want 0x%02x", reply[1], socks5ReplyHostUnreach)
	}
}

func TestSendSOCKS5ReplyCmdNotSupp(t *testing.T) {
	var buf bytes.Buffer
	conn := &bufConn{buf: &buf}
	
	err := sendSOCKS5Reply(conn, socks5ReplyCmdNotSupp)
	if err != nil {
		t.Fatalf("sendSOCKS5Reply failed: %v", err)
	}
	
	reply := buf.Bytes()
	if reply[1] != socks5ReplyCmdNotSupp {
		t.Errorf("reply[1] = 0x%02x, want 0x%02x", reply[1], socks5ReplyCmdNotSupp)
	}
}
