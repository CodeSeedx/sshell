package main

import (
	"testing"
)

// ==================== 第 8 轮：配置和书签边界条件 ====================

// Bug #36: config.go 中 applyConfig 的端口默认值
func TestApplyConfigPortDefaultR8(t *testing.T) {
	c := config{}
	a := args{}
	applyConfig(&a, c)
	
	if a.port != 22 {
		t.Errorf("port = %d, want 22 (default)", a.port)
	}
}

func TestApplyConfigPortFromConfigR8(t *testing.T) {
	c := config{DefaultPort: 2222}
	a := args{}
	applyConfig(&a, c)
	
	if a.port != 2222 {
		t.Errorf("port = %d, want 2222 (from config)", a.port)
	}
}

func TestApplyConfigPortFromArgsR8(t *testing.T) {
	c := config{DefaultPort: 2222}
	a := args{port: 3333}
	applyConfig(&a, c)
	
	if a.port != 3333 {
		t.Errorf("port = %d, want 3333 (from args priority)", a.port)
	}
}

// Bug #37: config.go 中 applyConfig 的 alive 默认值
func TestApplyConfigAliveDefaultR8(t *testing.T) {
	c := config{}
	a := args{}
	applyConfig(&a, c)
	
	if a.alive != 30 {
		t.Errorf("alive = %d, want 30 (default)", a.alive)
	}
}

func TestApplyConfigAliveFromConfigR8(t *testing.T) {
	c := config{DefaultAlive: 60}
	a := args{}
	applyConfig(&a, c)
	
	if a.alive != 60 {
		t.Errorf("alive = %d, want 60 (from config)", a.alive)
	}
}

// Bug #38: bookmark.go 中 argsToBookmark 的字段映射
func TestArgsToBookmarkAllFieldsR8(t *testing.T) {
	a := args{
		host:      "example.com",
		port:      2222,
		user:      "admin",
		auth:      "/path/to/key",
		alive:     60,
		proxyJump: "bastion",
		compress:  true,
		cmd:       "uptime",
	}
	
	b := argsToBookmark(a)
	
	if b.Host != "example.com" {
		t.Errorf("Host = %q", b.Host)
	}
	if b.Port != 2222 {
		t.Errorf("Port = %d", b.Port)
	}
	if b.User != "admin" {
		t.Errorf("User = %q", b.User)
	}
	if b.Auth != "/path/to/key" {
		t.Errorf("Auth = %q", b.Auth)
	}
	if b.Alive != 60 {
		t.Errorf("Alive = %d", b.Alive)
	}
	if b.ProxyJump != "bastion" {
		t.Errorf("ProxyJump = %q", b.ProxyJump)
	}
	if b.Compress != true {
		t.Errorf("Compress = %v", b.Compress)
	}
	if b.Cmd != "uptime" {
		t.Errorf("Cmd = %q", b.Cmd)
	}
}

// Bug #39: bookmark.go 中 bookmarkToArgs 的字段映射
func TestBookmarkToArgsAllFieldsR8(t *testing.T) {
	b := bookmark{
		Host:      "example.com",
		Port:      2222,
		User:      "admin",
		Auth:      "/path/to/key",
		Alive:     60,
		ProxyJump: "bastion",
		Compress:  true,
		Cmd:       "uptime",
	}
	
	a := bookmarkToArgs(b)
	
	if a.host != "example.com" {
		t.Errorf("host = %q", a.host)
	}
	if a.port != 2222 {
		t.Errorf("port = %d", a.port)
	}
	if a.user != "admin" {
		t.Errorf("user = %q", a.user)
	}
	if a.auth != "/path/to/key" {
		t.Errorf("auth = %q", a.auth)
	}
	if a.alive != 60 {
		t.Errorf("alive = %d", a.alive)
	}
	if a.proxyJump != "bastion" {
		t.Errorf("proxyJump = %q", a.proxyJump)
	}
	if a.compress != true {
		t.Errorf("compress = %v", a.compress)
	}
	if a.cmd != "uptime" {
		t.Errorf("cmd = %q", a.cmd)
	}
}

// Bug #40: bookmark.go 中 argsToBookmark 和 bookmarkToArgs 的往返一致性
func TestBookmarkRoundTripConsistencyR8(t *testing.T) {
	original := args{
		host:      "example.com",
		port:      2222,
		user:      "admin",
		auth:      "/path/to/key",
		alive:     60,
		proxyJump: "bastion",
		compress:  true,
		cmd:       "uptime",
	}
	
	b := argsToBookmark(original)
	restored := bookmarkToArgs(b)
	
	if original.host != restored.host {
		t.Errorf("host mismatch: %q vs %q", original.host, restored.host)
	}
	if original.port != restored.port {
		t.Errorf("port mismatch: %d vs %d", original.port, restored.port)
	}
	if original.user != restored.user {
		t.Errorf("user mismatch: %q vs %q", original.user, restored.user)
	}
	if original.auth != restored.auth {
		t.Errorf("auth mismatch: %q vs %q", original.auth, restored.auth)
	}
	if original.alive != restored.alive {
		t.Errorf("alive mismatch: %d vs %d", original.alive, restored.alive)
	}
	if original.proxyJump != restored.proxyJump {
		t.Errorf("proxyJump mismatch: %q vs %q", original.proxyJump, restored.proxyJump)
	}
	if original.compress != restored.compress {
		t.Errorf("compress mismatch: %v vs %v", original.compress, restored.compress)
	}
	if original.cmd != restored.cmd {
		t.Errorf("cmd mismatch: %q vs %q", original.cmd, restored.cmd)
	}
}

// Bug #41: bookmark.go 中空端口处理
func TestArgsToBookmarkZeroPortR8(t *testing.T) {
	a := args{host: "example.com", port: 0}
	b := argsToBookmark(a)
	
	if b.Port != 0 {
		t.Errorf("Port = %d, want 0", b.Port)
	}
}

func TestBookmarkToArgsZeroPortR8(t *testing.T) {
	b := bookmark{Host: "example.com", Port: 0}
	a := bookmarkToArgs(b)
	
	// bookmarkToArgs 会将端口 0 转换为默认值 22
	if a.port != 22 {
		t.Errorf("port = %d, want 22 (default)", a.port)
	}
}
