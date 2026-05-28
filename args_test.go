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
	if a.port != 22 {
		t.Errorf("port = %d, want 22", a.port)
	}
	if a.alive != 30 {
		t.Errorf("alive = %d, want 30", a.alive)
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
	_, err := parseArgsFrom([]string{"host"})
	if err == nil {
		t.Error("expected error for missing user")
	}
}

func TestParseArgsMissingHost(t *testing.T) {
	_, err := parseArgsFrom([]string{"-u", "root"})
	if err == nil {
		t.Error("expected error for missing host")
	}
}

func TestParseArgsEmpty(t *testing.T) {
	_, err := parseArgsFrom([]string{})
	if err == nil {
		t.Error("expected error for empty args")
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
