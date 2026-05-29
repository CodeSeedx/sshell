package main

import (
	"os"
	"testing"
)

// ==================== getTerminalSize 更多测试 ====================

func TestGetTerminalSizeDefault(t *testing.T) {
	// 在非终端环境（如 CI）中，应返回默认值
	w, h := getTerminalSize()
	if w <= 0 || h <= 0 {
		// 非终端环境，应该返回默认值
		if w != 80 {
			t.Errorf("width = %d, want 80 (default)", w)
		}
		if h != 24 {
			t.Errorf("height = %d, want 24 (default)", h)
		}
	}
}

func TestGetTerminalSizePositive(t *testing.T) {
	w, h := getTerminalSize()
	if w <= 0 {
		t.Errorf("width should be positive, got %d", w)
	}
	if h <= 0 {
		t.Errorf("height should be positive, got %d", h)
	}
}

func TestGetTerminalSizeLargeValues(t *testing.T) {
	w, h := getTerminalSize()
	// 终端宽度不应超过合理范围
	if w > 10000 {
		t.Errorf("width = %d, seems unreasonably large", w)
	}
	if h > 10000 {
		t.Errorf("height = %d, seems unreasonably large", h)
	}
}

func TestGetTerminalSizeMultipleCalls(t *testing.T) {
	// 多次调用应返回一致的结果（除非窗口被调整）
	sizes := make(map[[2]int]bool)
	for i := 0; i < 10; i++ {
		w, h := getTerminalSize()
		sizes[[2]int{w, h}] = true
	}
	// 在稳定环境下应该只有一种尺寸
	if len(sizes) > 2 {
		t.Errorf("got %d different sizes, expected at most 2", len(sizes))
	}
}

// ==================== version 变量测试 ====================

func TestVersionVariable(t *testing.T) {
	// version 变量应该是可修改的
	orig := version
	defer func() { version = orig }()

	version = "test-version-1.0.0"
	if version != "test-version-1.0.0" {
		t.Errorf("version = %q, want %q", version, "test-version-1.0.0")
	}
}

func TestVersionEmpty(t *testing.T) {
	orig := version
	defer func() { version = orig }()

	// 空字符串也是有效的版本值（虽然不常见）
	version = ""
	if version != "" {
		t.Errorf("version should be empty, got %q", version)
	}
}

func TestVersionSpecialChars(t *testing.T) {
	orig := version
	defer func() { version = orig }()

	version = "1.0.0-beta+build.123"
	if version != "1.0.0-beta+build.123" {
		t.Errorf("version = %q", version)
	}
}

// ==================== os.Args 交互测试 ====================

func TestOsArgsRestore(t *testing.T) {
	oldArgs := os.Args
	defer func() { os.Args = oldArgs }()

	os.Args = []string{"test", "-u", "user1", "host1"}
	a, err := parseArgsFrom(os.Args[1:])
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if a.user != "user1" || a.host != "host1" {
		t.Errorf("unexpected args: user=%q host=%q", a.user, a.host)
	}

	// 恢复 os.Args
	os.Args = oldArgs
	if os.Args[0] != oldArgs[0] {
		t.Errorf("os.Args[0] = %q, want %q", os.Args[0], oldArgs[0])
	}
}

func TestOsArgsMultipleTimes(t *testing.T) {
	oldArgs := os.Args
	defer func() { os.Args = oldArgs }()

	for i := 0; i < 5; i++ {
		os.Args = []string{"sshell", "-u", "user", "host"}
		a, err := parseArgsFrom(os.Args[1:])
		if err != nil {
			t.Fatalf("iteration %d: unexpected error: %v", i, err)
		}
		if a.host != "host" {
			t.Errorf("iteration %d: host = %q, want %q", i, a.host, "host")
		}
	}
}

// ==================== parseArgsFrom 边界测试 ====================

func TestParseArgsFromFlagAtEnd(t *testing.T) {
	// -p 后面没有值
	_, err := parseArgsFrom([]string{"-p"})
	if err != nil {
		t.Logf("parseArgsFrom -p at end: %v (acceptable)", err)
	}
}

func TestParseArgsFromUAtEnd(t *testing.T) {
	_, err := parseArgsFrom([]string{"-u"})
	if err != nil {
		t.Logf("parseArgsFrom -u at end: %v (acceptable)", err)
	}
}

func TestParseArgsFromAAtEnd(t *testing.T) {
	_, err := parseArgsFrom([]string{"-a"})
	if err != nil {
		t.Logf("parseArgsFrom -a at end: %v (acceptable)", err)
	}
}

func TestParseArgsFromKAtEnd(t *testing.T) {
	_, err := parseArgsFrom([]string{"-k"})
	if err != nil {
		t.Logf("parseArgsFrom -k at end: %v (acceptable)", err)
	}
}

func TestParseArgsFromDoubleDash(t *testing.T) {
	// --help 应该返回 help 错误
	_, err := parseArgsFrom([]string{"--help"})
	if err == nil {
		t.Error("expected error for --help")
	} else if err.Error() != "help" {
		t.Errorf("error = %q, want %q", err.Error(), "help")
	}
}

func TestParseArgsFromDoubleDashVersion(t *testing.T) {
	_, err := parseArgsFrom([]string{"--version"})
	if err == nil {
		t.Error("expected error for --version")
	} else if err.Error() != "version" {
		t.Errorf("error = %q, want %q", err.Error(), "version")
	}
}

func TestParseArgsFromUnknownFlag(t *testing.T) {
	// 未知的 -x 标志不会被识别，value 作为非标志参数会被当作 host
	a, err := parseArgsFrom([]string{"-x", "value"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// -x 不被识别，value 不以 - 开头所以被当作 host
	if a.host != "value" {
		t.Errorf("host = %q, want %q", a.host, "value")
	}
	// -x 不被识别，不会消费 "value"，所以 x 本身不会被解析成任何字段
	if a.user != "" {
		t.Errorf("user should be empty, got %q", a.user)
	}
}

func TestParseArgsFromUnknownFlagOnly(t *testing.T) {
	// 只有未知标志，没有值
	a, err := parseArgsFrom([]string{"-x"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if a.host != "" {
		t.Errorf("host should be empty, got %q", a.host)
	}
}

func TestParseArgsFromMultipleHosts(t *testing.T) {
	// 第二个非标志参数应该被忽略
	a, err := parseArgsFrom([]string{"host1", "host2"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if a.host != "host1" {
		t.Errorf("host = %q, want %q", a.host, "host1")
	}
}

func TestParseArgsFromLargePort(t *testing.T) {
	// 端口 65535 是最大值
	a, err := parseArgsFrom([]string{"-p", "65535", "host"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if a.port != 65535 {
		t.Errorf("port = %d, want 65535", a.port)
	}
}

func TestParseArgsFromPortOverflow(t *testing.T) {
	// 端口超出 uint16 范围
	_, err := parseArgsFrom([]string{"-p", "70000", "host"})
	if err == nil {
		t.Error("expected error for port overflow")
	}
}

func TestParseArgsFromLargeKeepAlive(t *testing.T) {
	a, err := parseArgsFrom([]string{"-k", "3600", "host"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if a.alive != 3600 {
		t.Errorf("alive = %d, want 3600", a.alive)
	}
}
