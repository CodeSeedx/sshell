package main

import (
	"fmt"
	"io"
	"strings"
	"testing"
)

// Test_safeMultiWriter_PrimaryWriterByteCount 验证 safeMultiWriter 返回主 writer 的实际写入字节数
func Test_safeMultiWriter_PrimaryWriterByteCount(t *testing.T) {
	tests := []struct {
		name       string
		mainWriter io.Writer
		auxWriter  io.Writer
		data       string
		wantN      int
		wantErr    bool
	}{
		{
			name:       "both writers succeed",
			mainWriter: &strings.Builder{},
			auxWriter:  &strings.Builder{},
			data:       "hello",
			wantN:      5,
			wantErr:    false,
		},
		{
			name:       "primary writer returns partial write",
			mainWriter: &partialWriter{maxWrite: 3},
			auxWriter:  &strings.Builder{},
			data:       "hello",
			wantN:      3, // 主 writer 只写了 3 字节
			wantErr:    false,
		},
		{
			name:       "primary writer fails",
			mainWriter: &failWriter{},
			auxWriter:  &strings.Builder{},
			data:       "hello",
			wantN:      0,
			wantErr:    true,
		},
		{
			name:       "aux writer fails but primary succeeds",
			mainWriter: &strings.Builder{},
			auxWriter:  &failWriter{},
			data:       "hello",
			wantN:      5, // 主 writer 写了全部 5 字节
			wantErr:    false,
		},
		{
			name:       "aux writer partial write but primary succeeds",
			mainWriter: &strings.Builder{},
			auxWriter:  &partialWriter{maxWrite: 2},
			data:       "hello",
			wantN:      5, // 主 writer 写了全部 5 字节
			wantErr:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := &safeMultiWriter{writers: []io.Writer{tt.mainWriter, tt.auxWriter}}
			n, err := m.Write([]byte(tt.data))
			if (err != nil) != tt.wantErr {
				t.Errorf("Write() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if n != tt.wantN {
				t.Errorf("Write() n = %d, want %d", n, tt.wantN)
			}
		})
	}
}

// partialWriter 写入最多 maxWrite 字节后返回 io.ErrShortWrite
type partialWriter struct {
	maxWrite int
	total    int
}

func (w *partialWriter) Write(p []byte) (int, error) {
	remaining := w.maxWrite - w.total
	if remaining <= 0 {
		return 0, io.ErrShortWrite
	}
	n := len(p)
	if n > remaining {
		n = remaining
	}
	w.total += n
	return n, nil
}

// failWriter 总是返回错误
type failWriter struct{}

func (w *failWriter) Write(p []byte) (int, error) {
	return 0, fmt.Errorf("write failed")
}

// Test_safeMultiWriter_EmptyData 验证空数据写入
func Test_safeMultiWriter_EmptyData(t *testing.T) {
	m := &safeMultiWriter{writers: []io.Writer{&strings.Builder{}, &strings.Builder{}}}
	n, err := m.Write([]byte{})
	if err != nil {
		t.Errorf("Write() unexpected error: %v", err)
	}
	if n != 0 {
		t.Errorf("Write() n = %d, want 0", n)
	}
}

// Test_safeMultiWriter_SingleWriter 验证只有主 writer 时的行为
func Test_safeMultiWriter_SingleWriter(t *testing.T) {
	main := &strings.Builder{}
	m := &safeMultiWriter{writers: []io.Writer{main}}
	n, err := m.Write([]byte("test"))
	if err != nil {
		t.Errorf("Write() unexpected error: %v", err)
	}
	if n != 4 {
		t.Errorf("Write() n = %d, want 4", n)
	}
	if main.String() != "test" {
		t.Errorf("Write() data = %q, want %q", main.String(), "test")
	}
}
