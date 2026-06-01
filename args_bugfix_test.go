package main

import (
	"testing"
)

// ==================== Bug #5: args.go -J 参数解析 ====================

func TestParseArgsJumpWithUserHost(t *testing.T) {
	// -J user@host 格式
	a, err := parseArgsFrom([]string{"-u", "root", "-J", "admin@bastion", "target"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// -J user@host 的用户应存入 proxyJumpUser，不覆盖 -u 指定的目标用户
	if a.user != "root" {
		t.Errorf("user = %q, want 'root'", a.user)
	}
	if a.proxyJumpUser != "admin" {
		t.Errorf("proxyJumpUser = %q, want 'admin'", a.proxyJumpUser)
	}
	if a.proxyJump != "bastion" {
		t.Errorf("proxyJump = %q, want 'bastion'", a.proxyJump)
	}
}

func TestParseArgsJumpWithUserHostPort(t *testing.T) {
	// -J user@host:port 格式
	a, err := parseArgsFrom([]string{"-u", "root", "-J", "admin@bastion:2222", "target"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if a.user != "root" {
		t.Errorf("user = %q, want 'root'", a.user)
	}
	if a.proxyJumpUser != "admin" {
		t.Errorf("proxyJumpUser = %q, want 'admin'", a.proxyJumpUser)
	}
	if a.proxyJump != "bastion" {
		t.Errorf("proxyJump = %q, want 'bastion'", a.proxyJump)
	}
	if a.proxyJumpPort != 2222 {
		t.Errorf("proxyJumpPort = %d, want 2222", a.proxyJumpPort)
	}
}

func TestParseArgsJumpWithHostPort(t *testing.T) {
	// -J host:port 格式（没有 user@）
	a, err := parseArgsFrom([]string{"-u", "root", "-J", "bastion:2222", "target"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if a.user != "root" {
		t.Errorf("user = %q, want 'root'", a.user)
	}
	if a.proxyJump != "bastion" {
		t.Errorf("proxyJump = %q, want 'bastion'", a.proxyJump)
	}
	if a.proxyJumpPort != 2222 {
		t.Errorf("proxyJumpPort = %d, want 2222", a.proxyJumpPort)
	}
}

func TestParseArgsJumpHostOnly(t *testing.T) {
	// -J host 格式
	a, err := parseArgsFrom([]string{"-u", "root", "-J", "bastion", "target"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if a.proxyJump != "bastion" {
		t.Errorf("proxyJump = %q, want 'bastion'", a.proxyJump)
	}
	if a.proxyJumpPort != 0 {
		t.Errorf("proxyJumpPort = %d, want 0", a.proxyJumpPort)
	}
}

// ==================== Bug #6: args.go 长选项解析 ====================

func TestParseArgsLongOptPut(t *testing.T) {
	a, err := parseArgsFrom([]string{"-u", "root", "--put", "local.txt:/remote.txt", "host"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if a.scpPut != "local.txt:/remote.txt" {
		t.Errorf("scpPut = %q, want 'local.txt:/remote.txt'", a.scpPut)
	}
}

func TestParseArgsLongOptGet(t *testing.T) {
	a, err := parseArgsFrom([]string{"-u", "root", "--get", "/remote.txt:local.txt", "host"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if a.scpGet != "/remote.txt:local.txt" {
		t.Errorf("scpGet = %q, want '/remote.txt:local.txt'", a.scpGet)
	}
}

func TestParseArgsLongOptSftp(t *testing.T) {
	a, err := parseArgsFrom([]string{"-u", "root", "--sftp", "--put", "local.txt:/remote.txt", "host"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !a.sftp {
		t.Error("sftp should be true")
	}
}

func TestParseArgsLongOptLog(t *testing.T) {
	a, err := parseArgsFrom([]string{"-u", "root", "--log", "/tmp/session.log", "host"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if a.logFile != "/tmp/session.log" {
		t.Errorf("logFile = %q, want '/tmp/session.log'", a.logFile)
	}
}

func TestParseArgsLongOptSave(t *testing.T) {
	a, err := parseArgsFrom([]string{"-u", "root", "--save", "myserver", "host"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if a.save != "myserver" {
		t.Errorf("save = %q, want 'myserver'", a.save)
	}
}

func TestParseArgsLongOptDelete(t *testing.T) {
	a, err := parseArgsFrom([]string{"--delete", "myserver"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if a.delete != "myserver" {
		t.Errorf("delete = %q, want 'myserver'", a.delete)
	}
}

func TestParseArgsLongOptList(t *testing.T) {
	a, err := parseArgsFrom([]string{"--list"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !a.list {
		t.Error("list should be true")
	}
}

func TestParseArgsLongOptNoAgent(t *testing.T) {
	a, err := parseArgsFrom([]string{"-u", "root", "--no-agent", "host"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !a.noAgent {
		t.Error("noAgent should be true")
	}
}

func TestParseArgsLongOptReconnect(t *testing.T) {
	a, err := parseArgsFrom([]string{"-u", "root", "--reconnect", "host"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !a.reconnect {
		t.Error("reconnect should be true")
	}
}

func TestParseArgsLongOptReconnectMax(t *testing.T) {
	a, err := parseArgsFrom([]string{"-u", "root", "--reconnect", "--reconnect-max", "5", "host"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if a.reconnectMax != 5 {
		t.Errorf("reconnectMax = %d, want 5", a.reconnectMax)
	}
}

// ==================== Bug #7: args.go 多主机模式 ====================

func TestParseArgsMultiHost(t *testing.T) {
	a, err := parseArgsFrom([]string{"-u", "root", "host1,host2,host3", "uptime"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if a.host != "host1,host2,host3" {
		t.Errorf("host = %q, want 'host1,host2,host3'", a.host)
	}
	if a.cmd != "uptime" {
		t.Errorf("cmd = %q, want 'uptime'", a.cmd)
	}
}

func TestParseArgsMultiHostWithSpaces(t *testing.T) {
	a, err := parseArgsFrom([]string{"-u", "root", "host1, host2 , host3", "uptime"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// finalizeArgs 会 trim 空格
	if a.host != "host1, host2 , host3" {
		t.Errorf("host = %q", a.host)
	}
}

// ==================== Bug #8: args.go 端口转发组合 ====================

func TestParseArgsMultipleLocalForwards(t *testing.T) {
	a, err := parseArgsFrom([]string{"-u", "root", "-L", "8080:localhost:80", "-L", "3306:localhost:3306", "host"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(a.localFwd) != 2 {
		t.Errorf("localFwd count = %d, want 2", len(a.localFwd))
	}
	if a.localFwd[0] != "8080:localhost:80" {
		t.Errorf("localFwd[0] = %q", a.localFwd[0])
	}
	if a.localFwd[1] != "3306:localhost:3306" {
		t.Errorf("localFwd[1] = %q", a.localFwd[1])
	}
}

func TestParseArgsMultipleRemoteForwards(t *testing.T) {
	a, err := parseArgsFrom([]string{"-u", "root", "-R", "8080:localhost:80", "-R", "3306:localhost:3306", "host"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(a.remoteFwd) != 2 {
		t.Errorf("remoteFwd count = %d, want 2", len(a.remoteFwd))
	}
}

func TestParseArgsLocalAndRemoteForward(t *testing.T) {
	a, err := parseArgsFrom([]string{"-u", "root", "-L", "8080:localhost:80", "-R", "9090:localhost:90", "host"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(a.localFwd) != 1 {
		t.Errorf("localFwd count = %d, want 1", len(a.localFwd))
	}
	if len(a.remoteFwd) != 1 {
		t.Errorf("remoteFwd count = %d, want 1", len(a.remoteFwd))
	}
}

func TestParseArgsSocksProxy(t *testing.T) {
	a, err := parseArgsFrom([]string{"-u", "root", "-D", "1080", "host"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if a.socksPort != "1080" {
		t.Errorf("socksPort = %q, want '1080'", a.socksPort)
	}
}
