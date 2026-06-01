package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestBookmarkSaveLoad(t *testing.T) {
	dir := t.TempDir()
	origHome := os.Getenv("HOME")
	os.Setenv("HOME", dir)
	defer os.Setenv("HOME", origHome)

	b := bookmark{
		Host: "192.168.1.100",
		Port: 2222,
		User: "admin",
	}

	if err := saveBookmark("myserver", b); err != nil {
		t.Fatalf("saveBookmark failed: %v", err)
	}

	loaded, ok := lookupBookmark("myserver")
	if !ok {
		t.Fatal("lookupBookmark returned false")
	}
	if loaded.host != "192.168.1.100" {
		t.Errorf("host = %q, want %q", loaded.host, "192.168.1.100")
	}
	if loaded.port != 2222 {
		t.Errorf("port = %d, want 2222", loaded.port)
	}
	if loaded.user != "admin" {
		t.Errorf("user = %q, want %q", loaded.user, "admin")
	}
}

func TestBookmarkDelete(t *testing.T) {
	dir := t.TempDir()
	origHome := os.Getenv("HOME")
	os.Setenv("HOME", dir)
	defer os.Setenv("HOME", origHome)

	saveBookmark("test", bookmark{Host: "example.com"})

	if err := deleteBookmark("test"); err != nil {
		t.Fatalf("deleteBookmark failed: %v", err)
	}

	_, ok := lookupBookmark("test")
	if ok {
		t.Error("bookmark should have been deleted")
	}
}

func TestBookmarkDeleteNotFound(t *testing.T) {
	dir := t.TempDir()
	origHome := os.Getenv("HOME")
	os.Setenv("HOME", dir)
	defer os.Setenv("HOME", origHome)

	err := deleteBookmark("nonexistent")
	if err == nil {
		t.Error("expected error for deleting non-existent bookmark")
	}
}

func TestBookmarkOverwrite(t *testing.T) {
	dir := t.TempDir()
	origHome := os.Getenv("HOME")
	os.Setenv("HOME", dir)
	defer os.Setenv("HOME", origHome)

	saveBookmark("srv", bookmark{Host: "old.example.com"})
	saveBookmark("srv", bookmark{Host: "new.example.com"})

	a, ok := lookupBookmark("srv")
	if !ok {
		t.Fatal("lookupBookmark returned false")
	}
	if a.host != "new.example.com" {
		t.Errorf("host = %q, want %q", a.host, "new.example.com")
	}
}

func TestArgsToBookmarkAndBack(t *testing.T) {
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

func TestLookupBookmarkNotFound(t *testing.T) {
	dir := t.TempDir()
	origHome := os.Getenv("HOME")
	os.Setenv("HOME", dir)
	defer os.Setenv("HOME", origHome)

	_, ok := lookupBookmark("nonexistent")
	if ok {
		t.Error("expected false for non-existent bookmark")
	}
}

func TestBookmarkFilePermissions(t *testing.T) {
	dir := t.TempDir()
	origHome := os.Getenv("HOME")
	os.Setenv("HOME", dir)
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

func TestBookmarkDirCreated(t *testing.T) {
	dir := t.TempDir()
	origHome := os.Getenv("HOME")
	os.Setenv("HOME", dir)
	defer os.Setenv("HOME", origHome)

	saveBookmark("test", bookmark{Host: "example.com"})

	// 验证 ~/.sshell/ 目录被创建
	sshellDir := filepath.Join(dir, ".sshell")
	info, err := os.Stat(sshellDir)
	if err != nil {
		t.Fatalf("~/.sshell/ directory not created: %v", err)
	}
	if !info.IsDir() {
		t.Error("~/.sshell/ should be a directory")
	}
}
