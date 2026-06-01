package main

import (
	"os"
	"path/filepath"
	"testing"
)

// ==================== Bug #10: bookmark.go 书签管理 ====================

func TestBookmarkSaveAndLookupExtended(t *testing.T) {
	tmpDir := t.TempDir()
	origHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpDir)
	defer os.Setenv("HOME", origHome)
	
	b := bookmark{
		Host: "192.168.1.100",
		Port: 2222,
		User: "admin",
		Auth: "/path/to/key",
	}
	
	err := saveBookmark("myserver", b)
	if err != nil {
		t.Fatalf("saveBookmark failed: %v", err)
	}
	
	loaded, ok := lookupBookmark("myserver")
	if !ok {
		t.Fatal("lookupBookmark returned false")
	}
	if loaded.host != "192.168.1.100" {
		t.Errorf("host = %q, want '192.168.1.100'", loaded.host)
	}
	if loaded.port != 2222 {
		t.Errorf("port = %d, want 2222", loaded.port)
	}
	if loaded.user != "admin" {
		t.Errorf("user = %q, want 'admin'", loaded.user)
	}
	if loaded.auth != "/path/to/key" {
		t.Errorf("auth = %q, want '/path/to/key'", loaded.auth)
	}
}

func TestBookmarkOverwriteExtended(t *testing.T) {
	tmpDir := t.TempDir()
	origHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpDir)
	defer os.Setenv("HOME", origHome)
	
	// 第一次保存
	saveBookmark("srv", bookmark{Host: "old.example.com"})
	
	// 第二次保存（覆盖）
	saveBookmark("srv", bookmark{Host: "new.example.com"})
	
	loaded, ok := lookupBookmark("srv")
	if !ok {
		t.Fatal("lookupBookmark returned false")
	}
	if loaded.host != "new.example.com" {
		t.Errorf("host = %q, want 'new.example.com'", loaded.host)
	}
}

func TestBookmarkDeleteExtended(t *testing.T) {
	tmpDir := t.TempDir()
	origHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpDir)
	defer os.Setenv("HOME", origHome)
	
	saveBookmark("test", bookmark{Host: "example.com"})
	
	err := deleteBookmark("test")
	if err != nil {
		t.Fatalf("deleteBookmark failed: %v", err)
	}
	
	_, ok := lookupBookmark("test")
	if ok {
		t.Error("bookmark should have been deleted")
	}
}

func TestBookmarkDeleteNotFoundExtended(t *testing.T) {
	tmpDir := t.TempDir()
	origHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpDir)
	defer os.Setenv("HOME", origHome)
	
	err := deleteBookmark("nonexistent")
	if err == nil {
		t.Error("expected error for deleting non-existent bookmark")
	}
}

func TestBookmarkListEmpty(t *testing.T) {
	tmpDir := t.TempDir()
	origHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpDir)
	defer os.Setenv("HOME", origHome)
	
	// 列出空书签应该不报错
	err := listBookmarks()
	if err != nil {
		t.Fatalf("listBookmarks failed: %v", err)
	}
}

func TestBookmarkLookupNotFoundExtended(t *testing.T) {
	tmpDir := t.TempDir()
	origHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpDir)
	defer os.Setenv("HOME", origHome)
	
	_, ok := lookupBookmark("nonexistent")
	if ok {
		t.Error("expected false for non-existent bookmark")
	}
}

func TestBookmarkArgsToBookmarkAndBackExtended(t *testing.T) {
	a := args{
		host:      "example.com",
		port:      2222,
		user:      "admin",
		auth:      "/path/to/key",
		alive:     60,
		proxyJump: "bastion",
		compress:  true,
		cmd:       "uptime",
	}
	
	b := argsToBookmark(a)
	if b.Host != "example.com" {
		t.Errorf("Host = %q", b.Host)
	}
	if b.Port != 2222 {
		t.Errorf("Port = %d", b.Port)
	}
	if b.ProxyJump != "bastion" {
		t.Errorf("ProxyJump = %q", b.ProxyJump)
	}
	
	back := bookmarkToArgs(b)
	if back.host != a.host {
		t.Errorf("host mismatch: %q vs %q", back.host, a.host)
	}
	if back.port != a.port {
		t.Errorf("port mismatch: %d vs %d", back.port, a.port)
	}
	if back.proxyJump != a.proxyJump {
		t.Errorf("proxyJump mismatch: %q vs %q", back.proxyJump, a.proxyJump)
	}
}

func TestBookmarkFilePermissionsExtended(t *testing.T) {
	tmpDir := t.TempDir()
	origHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpDir)
	defer os.Setenv("HOME", origHome)
	
	saveBookmark("test", bookmark{Host: "example.com"})
	
	path, _ := bookmarkFile()
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("bookmark file not found: %v", err)
	}
	
	// 验证权限为 0600
	if info.Mode().Perm() != 0600 {
		t.Errorf("bookmark file permissions = %o, want 0600", info.Mode().Perm())
	}
}

func TestBookmarkDirectoryCreationExtended(t *testing.T) {
	tmpDir := t.TempDir()
	origHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpDir)
	defer os.Setenv("HOME", origHome)
	
	saveBookmark("test", bookmark{Host: "example.com"})
	
	// 验证 ~/.sshell/ 目录被创建
	sshellDir := filepath.Join(tmpDir, ".sshell")
	info, err := os.Stat(sshellDir)
	if err != nil {
		t.Fatalf("~/.sshell/ directory not created: %v", err)
	}
	if !info.IsDir() {
		t.Error("~/.sshell/ should be a directory")
	}
}

func TestBookmarkMultiple(t *testing.T) {
	tmpDir := t.TempDir()
	origHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpDir)
	defer os.Setenv("HOME", origHome)
	
	// 保存多个书签
	saveBookmark("srv1", bookmark{Host: "srv1.example.com"})
	saveBookmark("srv2", bookmark{Host: "srv2.example.com"})
	saveBookmark("srv3", bookmark{Host: "srv3.example.com"})
	
	// 验证所有书签都存在
	for _, name := range []string{"srv1", "srv2", "srv3"} {
		_, ok := lookupBookmark(name)
		if !ok {
			t.Errorf("bookmark %q not found", name)
		}
	}
	
	// 删除一个
	deleteBookmark("srv2")
	
	// 验证 srv2 被删除，其他存在
	_, ok := lookupBookmark("srv2")
	if ok {
		t.Error("srv2 should have been deleted")
	}
	
	_, ok = lookupBookmark("srv1")
	if !ok {
		t.Error("srv1 should still exist")
	}
	
	_, ok = lookupBookmark("srv3")
	if !ok {
		t.Error("srv3 should still exist")
	}
}
