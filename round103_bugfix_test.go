package main

import (
	"testing"
)

// R103: 测试 --delete 书签删除不需要 host
func Test_ParseArgs_DeleteWithoutHost(t *testing.T) {
	// --delete 不需要 host 和 user
	a, err := parseArgsWithConfig([]string{"--delete", "mybook"})
	if err != nil {
		t.Fatalf("parseArgsWithConfig(--delete mybook) should not error, got: %v", err)
	}
	if a.delete != "mybook" {
		t.Errorf("expected delete=mybook, got %q", a.delete)
	}
	if a.host != "" {
		t.Errorf("expected empty host, got %q", a.host)
	}
}

// R103: 测试 --list 不需要 host（已有隐式覆盖，这里显式测试）
func Test_ParseArgs_ListWithoutHost(t *testing.T) {
	a, err := parseArgsWithConfig([]string{"--list"})
	if err != nil {
		t.Fatalf("parseArgsWithConfig(--list) should not error, got: %v", err)
	}
	if !a.list {
		t.Error("expected list=true")
	}
}

// R103: 测试没有 host 且没有 --list/--delete 时仍报错
func Test_ParseArgs_NoHostNoListNoDelete_Error(t *testing.T) {
	_, err := parseArgsWithConfig([]string{})
	if err == nil {
		t.Fatal("expected error when no host specified")
	}
}

// R103: 测试 --delete 与 --save 组合
func Test_ParseArgs_DeleteWithOtherFlags(t *testing.T) {
	// --delete 应该独立工作，不受其他标志影响
	a, err := parseArgsWithConfig([]string{"--delete", "mybook", "-v"})
	if err != nil {
		t.Fatalf("parseArgsWithConfig(--delete mybook -v) should not error, got: %v", err)
	}
	if a.delete != "mybook" {
		t.Errorf("expected delete=mybook, got %q", a.delete)
	}
	if !a.verbose {
		t.Error("expected verbose=true")
	}
}
