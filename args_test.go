package main

import (
	"os"
	"strings"
	"testing"
)

func TestParseArgsBasic(t *testing.T) {
	a, err := parseArgsFrom([]string{"-u", "root", "192.168.1.1"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if a.host != "192.168.1.1" {
		t.Errorf("host = %q, want %q", a.host, "192.168.1.1")
	}
	if a.user != "root" {
		t.Errorf("user = %q, want %q", a.user, "root")
	}
}

func TestParseArgsDefaults(t *testing.T) {
	a, err := parseArgsFrom([]string{"-u", "admin", "myhost"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// 注意：默认值现在由 applyConfig 设置，parseArgsFrom 不再设置默认值
	if a.port != 0 {
		t.Errorf("port = %d, want 0 (defaults applied later)", a.port)
	}
	if a.alive != 0 {
		t.Errorf("alive = %d, want 0 (defaults applied later)", a.alive)
	}
	if a.verbose {
		t.Error("verbose should be false by default")
	}
}

func TestParseArgsPort(t *testing.T) {
	a, err := parseArgsFrom([]string{"-u", "root", "-p", "2222", "host"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if a.port != 2222 {
		t.Errorf("port = %d, want 2222", a.port)
	}
}

func TestParseArgsInvalidPort(t *testing.T) {
	_, err := parseArgsFrom([]string{"-u", "root", "-p", "abc", "host"})
	if err == nil {
		t.Error("expected error for invalid port")
	}
	if !strings.Contains(err.Error(), "invalid port") {
		t.Errorf("error = %q, should contain 'invalid port'", err.Error())
	}
}

func TestParseArgsKeepAlive(t *testing.T) {
	a, err := parseArgsFrom([]string{"-u", "root", "-k", "60", "host"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if a.alive != 60 {
		t.Errorf("alive = %d, want 60", a.alive)
	}
}

func TestParseArgsInvalidKeepAlive(t *testing.T) {
	_, err := parseArgsFrom([]string{"-u", "root", "-k", "xyz", "host"})
	if err == nil {
		t.Error("expected error for invalid keep-alive")
	}
}

func TestParseArgsVerbose(t *testing.T) {
	a, err := parseArgsFrom([]string{"-u", "root", "-v", "host"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !a.verbose {
		t.Error("verbose should be true")
	}
}

func TestParseArgsAuth(t *testing.T) {
	a, err := parseArgsFrom([]string{"-u", "root", "-a", "mypassword", "host"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if a.auth != "mypassword" {
		t.Errorf("auth = %q, want %q", a.auth, "mypassword")
	}
}

func TestParseArgsMissingUser(t *testing.T) {
	// 注意：parseArgsFrom 不再验证缺少 user，只解析参数
	// 验证在 parseArgsVerbose 和 parseArgsWithConfig 中进行
	a, err := parseArgsFrom([]string{"host"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if a.host != "host" {
		t.Errorf("host = %q, want %q", a.host, "host")
	}
	if a.user != "" {
		t.Errorf("user should be empty, got %q", a.user)
	}
}

func TestParseArgsMissingHost(t *testing.T) {
	// 注意：parseArgsFrom 不再验证缺少 host，只解析参数
	// 验证在 parseArgsVerbose 和 parseArgsWithConfig 中进行
	a, err := parseArgsFrom([]string{"-u", "root"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if a.user != "root" {
		t.Errorf("user = %q, want %q", a.user, "root")
	}
	if a.host != "" {
		t.Errorf("host should be empty, got %q", a.host)
	}
}

func TestParseArgsEmpty(t *testing.T) {
	// 注意：parseArgsFrom 不再验证空参数，只解析参数
	// 验证在 parseArgsVerbose 和 parseArgsWithConfig 中进行
	a, err := parseArgsFrom([]string{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if a.host != "" {
		t.Errorf("host should be empty, got %q", a.host)
	}
	if a.user != "" {
		t.Errorf("user should be empty, got %q", a.user)
	}
}

func TestParseArgsHostFirst(t *testing.T) {
	a, err := parseArgsFrom([]string{"-u", "root", "myhost"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if a.host != "myhost" {
		t.Errorf("host = %q, want %q", a.host, "myhost")
	}
}

func TestParseArgsFullCombination(t *testing.T) {
	a, err := parseArgsFrom([]string{"-u", "deploy", "-p", "2222", "-k", "60", "-a", "/path/to/key", "-v", "server.example.com"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if a.host != "server.example.com" {
		t.Errorf("host = %q", a.host)
	}
	if a.user != "deploy" {
		t.Errorf("user = %q", a.user)
	}
	if a.port != 2222 {
		t.Errorf("port = %d", a.port)
	}
	if a.alive != 60 {
		t.Errorf("alive = %d", a.alive)
	}
	if a.auth != "/path/to/key" {
		t.Errorf("auth = %q", a.auth)
	}
	if !a.verbose {
		t.Error("verbose should be true")
	}
}

// ==================== parseArgsVerbose 测试 ====================

func TestParseArgsVerboseFuncBasic(t *testing.T) {
	// 保存并替换 os.Args
	oldArgs := os.Args
	defer func() { os.Args = oldArgs }()

	os.Args = []string{"sshell-go", "-u", "root", "192.168.1.1"}
	a, err := parseArgsVerbose()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if a.host != "192.168.1.1" {
		t.Errorf("host = %q, want %q", a.host, "192.168.1.1")
	}
	if a.user != "root" {
		t.Errorf("user = %q, want %q", a.user, "root")
	}
}

func TestParseArgsVerboseFuncHelp(t *testing.T) {
	oldArgs := os.Args
	defer func() { os.Args = oldArgs }()

	os.Args = []string{"sshell-go", "-h"}
	_, err := parseArgsVerbose()
	if err == nil {
		t.Error("expected error for help flag")
	}
	if err.Error() != "help" {
		t.Errorf("error = %q, want %q", err.Error(), "help")
	}
}

func TestParseArgsVerboseFuncVersion(t *testing.T) {
	oldArgs := os.Args
	defer func() { os.Args = oldArgs }()

	os.Args = []string{"sshell-go", "-V"}
	_, err := parseArgsVerbose()
	if err == nil {
		t.Error("expected error for version flag")
	}
	if err.Error() != "version" {
		t.Errorf("error = %q, want %q", err.Error(), "version")
	}
}

func TestParseArgsVerboseFuncMissingArgs(t *testing.T) {
	oldArgs := os.Args
	defer func() { os.Args = oldArgs }()

	os.Args = []string{"sshell-go"}
	_, err := parseArgsVerbose()
	if err == nil {
		t.Error("expected error for missing args")
	}
}

func TestParseArgsVerboseFuncInvalidPort(t *testing.T) {
	oldArgs := os.Args
	defer func() { os.Args = oldArgs }()

	os.Args = []string{"sshell-go", "-u", "root", "-p", "abc", "host"}
	_, err := parseArgsVerbose()
	if err == nil {
		t.Error("expected error for invalid port")
	}
}

func TestParseArgsVerboseFuncInvalidKeepAlive(t *testing.T) {
	oldArgs := os.Args
	defer func() { os.Args = oldArgs }()

	os.Args = []string{"sshell-go", "-u", "root", "-k", "xyz", "host"}
	_, err := parseArgsVerbose()
	if err == nil {
		t.Error("expected error for invalid keep-alive")
	}
}

func TestParseArgsVerboseFuncFullOptions(t *testing.T) {
	oldArgs := os.Args
	defer func() { os.Args = oldArgs }()

	os.Args = []string{"sshell-go", "-u", "admin", "-p", "2222", "-k", "60", "-a", "password", "-v", "server.com"}
	a, err := parseArgsVerbose()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if a.host != "server.com" {
		t.Errorf("host = %q", a.host)
	}
	if a.user != "admin" {
		t.Errorf("user = %q", a.user)
	}
	if a.port != 2222 {
		t.Errorf("port = %d", a.port)
	}
	if a.alive != 60 {
		t.Errorf("alive = %d", a.alive)
	}
	if a.auth != "password" {
		t.Errorf("auth = %q", a.auth)
	}
	if !a.verbose {
		t.Error("verbose should be true")
	}
}

// ==================== printUsage 测试 ====================

func TestPrintUsage(t *testing.T) {
	// printUsage 输出到 stderr，这里只测不 panic
	printUsage()
}

// ==================== cmd 参数解析测试 ====================

func TestParseArgsCmdSingle(t *testing.T) {
	a, err := parseArgsFrom([]string{"-u", "root", "host", "ls"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if a.host != "host" {
		t.Errorf("host = %q, want %q", a.host, "host")
	}
	if a.cmd != "ls" {
		t.Errorf("cmd = %q, want %q", a.cmd, "ls")
	}
}

func TestParseArgsCmdQuoted(t *testing.T) {
	a, err := parseArgsFrom([]string{"-u", "root", "host", "df -h"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if a.cmd != "df -h" {
		t.Errorf("cmd = %q, want %q", a.cmd, "df -h")
	}
}

func TestParseArgsCmdMultipleWords(t *testing.T) {
	a, err := parseArgsFrom([]string{"-u", "root", "host", "ls", "-la", "/tmp"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if a.cmd != "ls -la /tmp" {
		t.Errorf("cmd = %q, want %q", a.cmd, "ls -la /tmp")
	}
}

func TestParseArgsNoCmd(t *testing.T) {
	a, err := parseArgsFrom([]string{"-u", "root", "host"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if a.cmd != "" {
		t.Errorf("cmd = %q, want empty", a.cmd)
	}
}

func TestParseArgsCmdWithFlags(t *testing.T) {
	a, err := parseArgsFrom([]string{"-u", "root", "-p", "2222", "-v", "host", "uptime"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if a.host != "host" {
		t.Errorf("host = %q, want %q", a.host, "host")
	}
	if a.cmd != "uptime" {
		t.Errorf("cmd = %q, want %q", a.cmd, "uptime")
	}
	if a.port != 2222 {
		t.Errorf("port = %d, want 2222", a.port)
	}
	if !a.verbose {
		t.Error("verbose should be true")
	}
}

func TestParseArgsCmdFlagBeforeHost(t *testing.T) {
	// -v 在 host 之前，cmd 在 host 之后
	a, err := parseArgsFrom([]string{"-u", "root", "-v", "host", "echo", "hello"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if a.host != "host" {
		t.Errorf("host = %q, want %q", a.host, "host")
	}
	if a.cmd != "echo hello" {
		t.Errorf("cmd = %q, want %q", a.cmd, "echo hello")
	}
	if !a.verbose {
		t.Error("verbose should be true")
	}
}

// ==================== 修复验证测试 ====================

func Test_finalizeArgs_MultipleHosts(t *testing.T) {
	// 测试多主机解析
	a := args{
		host: "host1,host2,host3",
	}
	result := finalizeArgs(a)
	
	if len(result.hosts) != 3 {
		t.Fatalf("Expected 3 hosts, got %d", len(result.hosts))
	}
	
	if result.hosts[0] != "host1" {
		t.Errorf("Expected 'host1', got '%s'", result.hosts[0])
	}
	if result.hosts[1] != "host2" {
		t.Errorf("Expected 'host2', got '%s'", result.hosts[1])
	}
	if result.hosts[2] != "host3" {
		t.Errorf("Expected 'host3', got '%s'", result.hosts[2])
	}
}

func Test_finalizeArgs_EmptyHosts(t *testing.T) {
	// 测试空主机字符串
	a := args{
		host: "",
	}
	result := finalizeArgs(a)
	
	if len(result.hosts) != 0 {
		t.Fatalf("Expected 0 hosts, got %d", len(result.hosts))
	}
}

func Test_finalizeArgs_ConsecutiveCommas(t *testing.T) {
	// 测试连续逗号（边界条件）
	a := args{
		host: "host1,,host2",
	}
	result := finalizeArgs(a)
	
	// 应该过滤掉空字符串
	if len(result.hosts) != 2 {
		t.Fatalf("Expected 2 hosts, got %d: %v", len(result.hosts), result.hosts)
	}
	
	if result.hosts[0] != "host1" {
		t.Errorf("Expected 'host1', got '%s'", result.hosts[0])
	}
	if result.hosts[1] != "host2" {
		t.Errorf("Expected 'host2', got '%s'", result.hosts[1])
	}
}

func Test_finalizeArgs_SpacesInHosts(t *testing.T) {
	// 测试主机名中包含空格
	a := args{
		host: " host1 , host2 ",
	}
	result := finalizeArgs(a)
	
	if len(result.hosts) != 2 {
		t.Fatalf("Expected 2 hosts, got %d: %v", len(result.hosts), result.hosts)
	}
	
	if result.hosts[0] != "host1" {
		t.Errorf("Expected 'host1', got '%s'", result.hosts[0])
	}
	if result.hosts[1] != "host2" {
		t.Errorf("Expected 'host2', got '%s'", result.hosts[1])
	}
}

func Test_finalizeArgs_SingleHost(t *testing.T) {
	// 测试单个主机
	a := args{
		host: "singlehost",
	}
	result := finalizeArgs(a)
	
	if len(result.hosts) != 1 {
		t.Fatalf("Expected 1 host, got %d", len(result.hosts))
	}
	
	if result.hosts[0] != "singlehost" {
		t.Errorf("Expected 'singlehost', got '%s'", result.hosts[0])
	}
}

func Test_finalizeArgs_TrailingComma(t *testing.T) {
	// 测试末尾逗号
	a := args{
		host: "host1,host2,",
	}
	result := finalizeArgs(a)
	
	// 应该过滤掉空字符串
	if len(result.hosts) != 2 {
		t.Fatalf("Expected 2 hosts, got %d: %v", len(result.hosts), result.hosts)
	}
}

func Test_finalizeArgs_LeadingComma(t *testing.T) {
	// 测试开头逗号
	a := args{
		host: ",host1,host2",
	}
	result := finalizeArgs(a)
	
	// 应该过滤掉空字符串
	if len(result.hosts) != 2 {
		t.Fatalf("Expected 2 hosts, got %d: %v", len(result.hosts), result.hosts)
	}
}