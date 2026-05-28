package main

import (
	"os"
	"testing"
)

func TestGetTerminalSize(t *testing.T) {
	w, h := getTerminalSize()
	if w <= 0 || h <= 0 {
		if w != 80 {
			t.Errorf("width = %d, want 80 (default)", w)
		}
		if h != 24 {
			t.Errorf("height = %d, want 24 (default)", h)
		}
	}
	t.Logf("terminal size: %dx%d", w, h)
}

func TestGetTerminalSizeNotPanic(t *testing.T) {
	for i := 0; i < 100; i++ {
		getTerminalSize()
	}
}

func TestVersionDefault(t *testing.T) {
	if version != "dev" {
		t.Errorf("version = %q, want %q", version, "dev")
	}
}

func TestVersionAfterSet(t *testing.T) {
	old := version
	version = "1.2.3"
	defer func() { version = old }()

	if version != "1.2.3" {
		t.Errorf("version = %q, want %q", version, "1.2.3")
	}
}

func TestVersionIsString(t *testing.T) {
	if len(version) == 0 {
		t.Error("version should not be empty")
	}
}

func TestOsArgsBackup(t *testing.T) {
	oldArgs := os.Args
	defer func() { os.Args = oldArgs }()

	os.Args = []string{"sshell-go", "-u", "test", "host"}
	a, err := parseArgsFrom(os.Args[1:])
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if a.host != "host" {
		t.Errorf("host = %q, want %q", a.host, "host")
	}
}

func TestGetTerminalSizeConsistency(t *testing.T) {
	// 多次调用应该返回一致的结果
	w1, h1 := getTerminalSize()
	w2, h2 := getTerminalSize()
	if w1 != w2 || h1 != h2 {
		t.Errorf("inconsistent results: (%d,%d) vs (%d,%d)", w1, h1, w2, h2)
	}
}

func BenchmarkGetTerminalSize(b *testing.B) {
	for i := 0; i < b.N; i++ {
		getTerminalSize()
	}
}
