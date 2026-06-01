package main

import (
	"testing"
)

// ==================== Bug #1: splitForwardSpec 边界条件 ====================

func TestSplitForwardSpecEmptyString(t *testing.T) {
	// Bug: 空字符串应该返回空切片，而不是 [" ""]
	parts := splitForwardSpec("")
	if len(parts) != 0 {
		t.Errorf("splitForwardSpec('') returned %v (len=%d), want empty slice", parts, len(parts))
	}
}

func TestSplitForwardSpecSingleColon(t *testing.T) {
	// Bug: 单个冒号 ":" 应该返回两个空字符串
	parts := splitForwardSpec(":")
	if len(parts) != 2 {
		t.Errorf("splitForwardSpec(':') returned %v (len=%d), want ['', '']", parts, len(parts))
	}
}

func TestSplitForwardSpecTrailingColon(t *testing.T) {
	// 尾部冒号应该被正确处理
	parts := splitForwardSpec("8080:")
	if len(parts) != 2 {
		t.Errorf("splitForwardSpec('8080:') returned %v (len=%d), want 2 parts", parts, len(parts))
	}
	if parts[0] != "8080" {
		t.Errorf("parts[0] = %q, want '8080'", parts[0])
	}
	if parts[1] != "" {
		t.Errorf("parts[1] = %q, want ''", parts[1])
	}
}

func TestSplitForwardSpecIPv6(t *testing.T) {
	// IPv6 地址应该被正确处理
	parts := splitForwardSpec("[::1]:8080:localhost:80")
	if len(parts) != 4 {
		t.Errorf("splitForwardSpec IPv6 returned %v (len=%d), want 4 parts", parts, len(parts))
	}
	if parts[0] != "::1" {
		t.Errorf("parts[0] = %q, want '::1'", parts[0])
	}
	if parts[1] != "8080" {
		t.Errorf("parts[1] = %q, want '8080'", parts[1])
	}
}

func TestSplitForwardSpecIPv6BracketOnly(t *testing.T) {
	// 只有括号的情况
	parts := splitForwardSpec("[::1]")
	if len(parts) != 1 {
		t.Errorf("splitForwardSpec('[::1]') returned %v (len=%d), want 1 part", parts, len(parts))
	}
	if parts[0] != "::1" {
		t.Errorf("parts[0] = %q, want '::1'", parts[0])
	}
}

// ==================== Bug #2: parseForwardSpec 边界条件 ====================

func TestParseForwardSpecEmptyString(t *testing.T) {
	_, _, _, _, err := parseForwardSpec("")
	if err == nil {
		t.Error("parseForwardSpec('') should return error")
	}
}

func TestParseForwardSpecSingleField(t *testing.T) {
	_, _, _, _, err := parseForwardSpec("8080")
	if err == nil {
		t.Error("parseForwardSpec('8080') should return error")
	}
}

func TestParseForwardSpecPortZero(t *testing.T) {
	// 端口 0 应该被接受（表示随机端口）
	bindAddr, port, destHost, destPort, err := parseForwardSpec("0:localhost:80")
	if err != nil {
		t.Fatalf("parseForwardSpec('0:localhost:80') unexpected error: %v", err)
	}
	if bindAddr != "localhost" {
		t.Errorf("bindAddr = %q, want 'localhost'", bindAddr)
	}
	if port != 0 {
		t.Errorf("port = %d, want 0", port)
	}
	if destHost != "localhost" {
		t.Errorf("destHost = %q, want 'localhost'", destHost)
	}
	if destPort != 80 {
		t.Errorf("destPort = %d, want 80", destPort)
	}
}

func TestParseForwardSpecMaxPort(t *testing.T) {
	// 最大端口 65535
	bindAddr, port, destHost, destPort, err := parseForwardSpec("65535:localhost:65534")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if bindAddr != "localhost" {
		t.Errorf("bindAddr = %q, want 'localhost'", bindAddr)
	}
	if port != 65535 {
		t.Errorf("port = %d, want 65535", port)
	}
	if destHost != "localhost" {
		t.Errorf("destHost = %q, want 'localhost'", destHost)
	}
	if destPort != 65534 {
		t.Errorf("destPort = %d, want 65534", destPort)
	}
}

func TestParseForwardSpecOverflowPort(t *testing.T) {
	// 端口溢出应该返回错误
	_, _, _, _, err := parseForwardSpec("70000:localhost:80")
	if err == nil {
		t.Error("parseForwardSpec('70000:localhost:80') should return error")
	}
}

func TestParseForwardSpecNegativePort(t *testing.T) {
	// 负端口应该返回错误
	_, _, _, _, err := parseForwardSpec("-1:localhost:80")
	if err == nil {
		t.Error("parseForwardSpec('-1:localhost:80') should return error")
	}
}

// ==================== Bug #3: startRemoteForward 端口映射问题 ====================

func TestRemoteForwardMappingsStore(t *testing.T) {
	// 测试端口映射存储
	remoteForwardMappings.Store(uint16(8080), remoteForwardEntry{localAddr: "localhost:80", verbose: false})
	
	val, ok := remoteForwardMappings.Load(uint16(8080))
	if !ok {
		t.Error("expected port 8080 to be stored")
	}
	entry := val.(remoteForwardEntry)
	if entry.localAddr != "localhost:80" {
		t.Errorf("localAddr = %q, want 'localhost:80'", entry.localAddr)
	}
	if entry.verbose {
		t.Error("verbose should be false")
	}
	
	// 清理
	remoteForwardMappings.Delete(uint16(8080))
}

func TestRemoteForwardMappingsNonExistent(t *testing.T) {
	// 测试不存在的端口
	_, ok := remoteForwardMappings.Load(uint16(9999))
	if ok {
		t.Error("expected port 9999 to not be stored")
	}
}

// ==================== Bug #4: SOCKS5 地址解析 ====================

func TestReadSOCKS5AddrIPv4Loopback(t *testing.T) {
	// 127.0.0.1:8080
	data := []byte{127, 0, 0, 1, 0x1F, 0x90}
	r := &byteReader{data: data}
	addr, err := readSOCKS5Addr(r, socks5AddrTypeIPv4)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if addr != "127.0.0.1:8080" {
		t.Errorf("addr = %q, want '127.0.0.1:8080'", addr)
	}
}

func TestReadSOCKS5AddrIPv6Loopback(t *testing.T) {
	// ::1:443
	data := make([]byte, 18)
	data[15] = 1 // ::1
	data[16] = 0x01
	data[17] = 0xBB // 443
	r := &byteReader{data: data}
	addr, err := readSOCKS5Addr(r, socks5AddrTypeIPv6)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if addr != "[::1]:443" {
		t.Errorf("addr = %q, want '[::1]:443'", addr)
	}
}

func TestReadSOCKS5AddrDomainLong(t *testing.T) {
	// 长域名
	domain := "very-long-domain-name.example.com"
	data := make([]byte, 1+len(domain)+2)
	data[0] = byte(len(domain))
	copy(data[1:], domain)
	data[1+len(domain)] = 0x00
	data[2+len(domain)] = 0x50 // port 80
	r := &byteReader{data: data}
	addr, err := readSOCKS5Addr(r, socks5AddrTypeDomain)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	expected := domain + ":80"
	if addr != expected {
		t.Errorf("addr = %q, want %q", addr, expected)
	}
}

// byteReader 是一个简单的字节读取器，用于测试
type byteReader struct {
	data []byte
	pos  int
}

func (r *byteReader) Read(p []byte) (n int, err error) {
	if r.pos >= len(r.data) {
		return 0, nil
	}
	n = copy(p, r.data[r.pos:])
	r.pos += n
	return n, nil
}
