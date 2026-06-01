package main

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"
)

// sessionLogger 将会话输出同时写入原始 writer 和日志文件
type sessionLogger struct {
	file    *os.File
	verbose bool
}

// newSessionLogger 创建新的会话日志器
func newSessionLogger(logPath string, verbose bool) (*sessionLogger, error) {
	// 确保目录存在
	dir := filepath.Dir(logPath)
	if dir != "" && dir != "." {
		if err := os.MkdirAll(dir, 0700); err != nil {
			return nil, fmt.Errorf("create log dir: %w", err)
		}
	}

	f, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0600)
	if err != nil {
		return nil, fmt.Errorf("open log file: %w", err)
	}

	// 写入日志头
	header := fmt.Sprintf("# sshell session log - started %s\n# ---\n", time.Now().Format(time.RFC3339))
	if _, err := f.WriteString(header); err != nil {
		f.Close()
		return nil, fmt.Errorf("write log header: %w", err)
	}

	if verbose {
		fmt.Fprintf(os.Stderr, "[sshell] Logging to %s\n", logPath)
	}

	return &sessionLogger{file: f, verbose: verbose}, nil
}

// WrapWriter 返回一个同时写入原始 writer 和日志文件的 writer
// 日志写入失败时不会影响原始 writer
func (l *sessionLogger) WrapWriter(w io.Writer) io.Writer {
	return &safeMultiWriter{writers: []io.Writer{w, l.file}}
}

// safeMultiWriter 类似 io.MultiWriter，但辅助 writer 写入失败时不影响主 writer
type safeMultiWriter struct {
	writers []io.Writer
}

func (m *safeMultiWriter) Write(p []byte) (n int, err error) {
	for i, w := range m.writers {
		n, err = w.Write(p)
		if err != nil {
			// 主 writer（第一个）失败则返回错误
			if i == 0 {
				return n, err
			}
			// 辅助 writer 失败，忽略错误，继续写入其他 writer
		}
	}
	return len(p), nil
}

// Close 关闭日志文件
func (l *sessionLogger) Close() error {
	_, err1 := l.file.WriteString(fmt.Sprintf("# ---\n# sshell session log - ended %s\n", time.Now().Format(time.RFC3339)))
	err2 := l.file.Close()
	if err1 != nil {
		return err1
	}
	return err2
}
