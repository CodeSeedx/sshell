package main

import (
	"testing"
)

// ==================== Bug #13: logging.go 日志记录 ====================

func TestSessionLoggerNew(t *testing.T) {
	dir := t.TempDir()
	logPath := dir + "/test.log"
	
	logger, err := newSessionLogger(logPath, false)
	if err != nil {
		t.Fatalf("newSessionLogger failed: %v", err)
	}
	defer logger.Close()
	
	if logger.file == nil {
		t.Error("logger.file should not be nil")
	}
}

func TestSessionLoggerVerbose(t *testing.T) {
	dir := t.TempDir()
	logPath := dir + "/test.log"
	
	logger, err := newSessionLogger(logPath, true)
	if err != nil {
		t.Fatalf("newSessionLogger failed: %v", err)
	}
	defer logger.Close()
	
	if !logger.verbose {
		t.Error("logger.verbose should be true")
	}
}

func TestSessionLoggerCloseTwice(t *testing.T) {
	dir := t.TempDir()
	logPath := dir + "/test.log"
	
	logger, err := newSessionLogger(logPath, false)
	if err != nil {
		t.Fatalf("newSessionLogger failed: %v", err)
	}
	
	// 第一次关闭
	err1 := logger.Close()
	if err1 != nil {
		t.Errorf("first Close failed: %v", err1)
	}
	
	// 第二次关闭可能会失败（文件已关闭），但不应该 panic
	logger.Close()
}

// ==================== Bug #14: multi.go 多主机执行 ====================

func TestPrefixLinesExtended(t *testing.T) {
	// 测试多行输入
	input := "line1\nline2\nline3\n"
	reader := &stringReader{s: input}
	var buf []byte
	writer := &byteWriter{buf: &buf}
	
	prefixLines("[host] ", reader, writer)
	
	expected := "[host] line1\n[host] line2\n[host] line3\n"
	if string(buf) != expected {
		t.Errorf("prefixLines output = %q, want %q", string(buf), expected)
	}
}

func TestPrefixLinesEmptyExtended(t *testing.T) {
	reader := &stringReader{s: ""}
	var buf []byte
	writer := &byteWriter{buf: &buf}
	
	prefixLines("[host] ", reader, writer)
	
	if len(buf) != 0 {
		t.Errorf("prefixLines empty input should produce empty output, got %q", string(buf))
	}
}

func TestPrefixLinesSingleLineExtended(t *testing.T) {
	reader := &stringReader{s: "hello"}
	var buf []byte
	writer := &byteWriter{buf: &buf}
	
	prefixLines("[srv] ", reader, writer)
	
	expected := "[srv] hello\n"
	if string(buf) != expected {
		t.Errorf("prefixLines output = %q, want %q", string(buf), expected)
	}
}

// stringReader 是一个简单的字符串读取器
type stringReader struct {
	s   string
	pos int
}

func (r *stringReader) Read(p []byte) (n int, err error) {
	if r.pos >= len(r.s) {
		return 0, nil
	}
	n = copy(p, r.s[r.pos:])
	r.pos += n
	return n, nil
}

// byteWriter 是一个简单的字节写入器
type byteWriter struct {
	buf *[]byte
}

func (w *byteWriter) Write(p []byte) (n int, err error) {
	*w.buf = append(*w.buf, p...)
	return len(p), nil
}
