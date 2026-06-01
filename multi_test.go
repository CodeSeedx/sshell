package main

import (
	"bytes"
	"strings"
	"testing"
)

func TestPrefixLines(t *testing.T) {
	var buf bytes.Buffer
	input := "line1\nline2\nline3"
	prefixLines("[host] ", strings.NewReader(input), &buf)

	expected := "[host] line1\n[host] line2\n[host] line3\n"
	if buf.String() != expected {
		t.Errorf("prefixLines output = %q, want %q", buf.String(), expected)
	}
}

func TestPrefixLinesEmpty(t *testing.T) {
	var buf bytes.Buffer
	prefixLines("[host] ", strings.NewReader(""), &buf)

	if buf.String() != "" {
		t.Errorf("prefixLines empty input should produce empty output, got %q", buf.String())
	}
}

func TestPrefixLinesSingleLine(t *testing.T) {
	var buf bytes.Buffer
	prefixLines("[srv] ", strings.NewReader("hello"), &buf)

	expected := "[srv] hello\n"
	if buf.String() != expected {
		t.Errorf("prefixLines output = %q, want %q", buf.String(), expected)
	}
}

func TestRunMultiHostNoHosts(t *testing.T) {
	a := args{hosts: []string{}}
	err := runMultiHost(a)
	if err == nil {
		t.Error("expected error for empty hosts")
	}
}
