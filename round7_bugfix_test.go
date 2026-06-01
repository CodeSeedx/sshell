package main

import (
	"testing"
)

// ==================== 第 7 轮：参数解析边界条件 ====================

// Bug #28: 长选项 = 格式解析
func TestParseArgsLongOptEquals(t *testing.T) {
	a, err := parseArgsFrom([]string{"-u", "root", "--put=local.txt:/remote.txt", "host"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if a.scpPut != "local.txt:/remote.txt" {
		t.Errorf("scpPut = %q, want 'local.txt:/remote.txt'", a.scpPut)
	}
}

func TestParseArgsLongOptEqualsLog(t *testing.T) {
	a, err := parseArgsFrom([]string{"-u", "root", "--log=/tmp/test.log", "host"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if a.logFile != "/tmp/test.log" {
		t.Errorf("logFile = %q, want '/tmp/test.log'", a.logFile)
	}
}

func TestParseArgsLongOptEqualsSave(t *testing.T) {
	a, err := parseArgsFrom([]string{"-u", "root", "--save=myserver", "host"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if a.save != "myserver" {
		t.Errorf("save = %q, want 'myserver'", a.save)
	}
}

// Bug #29: 短选项值以 - 开头的处理
func TestParseArgsValueStartsWithDash(t *testing.T) {
	// 这应该报错
	_, err := parseArgsFrom([]string{"-u", "-root", "host"})
	if err == nil {
		t.Error("expected error for value starting with dash")
	}
}

func TestParseArgsPortValueStartsWithDash(t *testing.T) {
	_, err := parseArgsFrom([]string{"-u", "root", "-p", "-22", "host"})
	if err == nil {
		t.Error("expected error for port value starting with dash")
	}
}

// Bug #30: 未知选项处理
func TestParseArgsUnknownShortOption(t *testing.T) {
	_, err := parseArgsFrom([]string{"-u", "root", "-x", "host"})
	if err == nil {
		t.Error("expected error for unknown option -x")
	}
}

func TestParseArgsUnknownLongOption(t *testing.T) {
	_, err := parseArgsFrom([]string{"-u", "root", "--unknown", "host"})
	if err == nil {
		t.Error("expected error for unknown option --unknown")
	}
}

// Bug #31: 帮助和版本选项
func TestParseArgsHelpShort(t *testing.T) {
	_, err := parseArgsFrom([]string{"-h"})
	if err == nil || err.Error() != "help" {
		t.Errorf("expected help error, got: %v", err)
	}
}

func TestParseArgsHelpLong(t *testing.T) {
	_, err := parseArgsFrom([]string{"--help"})
	if err == nil || err.Error() != "help" {
		t.Errorf("expected help error, got: %v", err)
	}
}

func TestParseArgsVersionShort(t *testing.T) {
	_, err := parseArgsFrom([]string{"-V"})
	if err == nil || err.Error() != "version" {
		t.Errorf("expected version error, got: %v", err)
	}
}

func TestParseArgsVersionLong(t *testing.T) {
	_, err := parseArgsFrom([]string{"--version"})
	if err == nil || err.Error() != "version" {
		t.Errorf("expected version error, got: %v", err)
	}
}

// Bug #32: 命令拼接
func TestParseArgsCommandConcatenation(t *testing.T) {
	a, err := parseArgsFrom([]string{"-u", "root", "host", "echo", "hello", "world"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if a.cmd != "echo hello world" {
		t.Errorf("cmd = %q, want 'echo hello world'", a.cmd)
	}
}

func TestParseArgsCommandWithFlags(t *testing.T) {
	a, err := parseArgsFrom([]string{"-u", "root", "host", "ls", "-la", "/tmp"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if a.cmd != "ls -la /tmp" {
		t.Errorf("cmd = %q, want 'ls -la /tmp'", a.cmd)
	}
}

// Bug #33: 空命令
func TestParseArgsNoCommand(t *testing.T) {
	a, err := parseArgsFrom([]string{"-u", "root", "host"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if a.cmd != "" {
		t.Errorf("cmd = %q, want ''", a.cmd)
	}
}

// Bug #34: finalizeArgs 多主机拆分
func TestFinalizeArgsMultiHost(t *testing.T) {
	a := args{host: "host1,host2,host3"}
	a = finalizeArgs(a)
	
	if len(a.hosts) != 3 {
		t.Fatalf("len(hosts) = %d, want 3", len(a.hosts))
	}
	if a.hosts[0] != "host1" {
		t.Errorf("hosts[0] = %q, want 'host1'", a.hosts[0])
	}
	if a.hosts[1] != "host2" {
		t.Errorf("hosts[1] = %q, want 'host2'", a.hosts[1])
	}
	if a.hosts[2] != "host3" {
		t.Errorf("hosts[2] = %q, want 'host3'", a.hosts[2])
	}
}

func TestFinalizeArgsMultiHostWithSpaces(t *testing.T) {
	a := args{host: "host1, host2 , host3"}
	a = finalizeArgs(a)
	
	if len(a.hosts) != 3 {
		t.Fatalf("len(hosts) = %d, want 3", len(a.hosts))
	}
	if a.hosts[0] != "host1" {
		t.Errorf("hosts[0] = %q, want 'host1'", a.hosts[0])
	}
	if a.hosts[1] != "host2" {
		t.Errorf("hosts[1] = %q, want 'host2'", a.hosts[1])
	}
	if a.hosts[2] != "host3" {
		t.Errorf("hosts[2] = %q, want 'host3'", a.hosts[2])
	}
}

func TestFinalizeArgsSingleHost(t *testing.T) {
	a := args{host: "myhost"}
	a = finalizeArgs(a)
	
	if len(a.hosts) != 1 {
		t.Fatalf("len(hosts) = %d, want 1", len(a.hosts))
	}
	if a.hosts[0] != "myhost" {
		t.Errorf("hosts[0] = %q, want 'myhost'", a.hosts[0])
	}
}

// Bug #35: reconnect 默认值
func TestFinalizeArgsReconnectDefault(t *testing.T) {
	a := args{reconnect: true, reconnectMax: 0}
	a = finalizeArgs(a)
	
	if a.reconnectMax != 3 {
		t.Errorf("reconnectMax = %d, want 3 (default)", a.reconnectMax)
	}
}

func TestFinalizeArgsReconnectCustom(t *testing.T) {
	a := args{reconnect: true, reconnectMax: 5}
	a = finalizeArgs(a)
	
	if a.reconnectMax != 5 {
		t.Errorf("reconnectMax = %d, want 5", a.reconnectMax)
	}
}

func TestFinalizeArgsReconnectDisabled(t *testing.T) {
	a := args{reconnect: false, reconnectMax: 0}
	a = finalizeArgs(a)
	
	if a.reconnectMax != 0 {
		t.Errorf("reconnectMax = %d, want 0", a.reconnectMax)
	}
}
