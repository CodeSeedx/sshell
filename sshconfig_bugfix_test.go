package main

import (
	"os"
	"path/filepath"
	"testing"
)

// ==================== Bug #11: sshconfig.go SSH 配置解析 ====================

func TestSSHConfigParseBasicExtended(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "config")
	content := `Host myserver
    HostName 192.168.1.100
    Port 2222
    User admin
    IdentityFile ~/.ssh/mykey
`
	os.WriteFile(configPath, []byte(content), 0644)
	
	blocks := parseSSHConfig(configPath)
	if len(blocks) != 1 {
		t.Fatalf("expected 1 block, got %d", len(blocks))
	}
	
	b := blocks[0]
	if b.Host != "myserver" {
		t.Errorf("Host = %q, want 'myserver'", b.Host)
	}
	if b.HostName != "192.168.1.100" {
		t.Errorf("HostName = %q, want '192.168.1.100'", b.HostName)
	}
	if b.Port != 2222 {
		t.Errorf("Port = %d, want 2222", b.Port)
	}
	if b.User != "admin" {
		t.Errorf("User = %q, want 'admin'", b.User)
	}
	if b.IdentityFile != "~/.ssh/mykey" {
		t.Errorf("IdentityFile = %q, want '~/.ssh/mykey'", b.IdentityFile)
	}
}

func TestSSHConfigParseMultipleHostsExtended(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "config")
	content := `Host web
    HostName web.example.com
    Port 22

Host db
    HostName db.example.com
    Port 5432
    User postgres
`
	os.WriteFile(configPath, []byte(content), 0644)
	
	blocks := parseSSHConfig(configPath)
	if len(blocks) != 2 {
		t.Fatalf("expected 2 blocks, got %d", len(blocks))
	}
	
	if blocks[0].Host != "web" {
		t.Errorf("blocks[0].Host = %q, want 'web'", blocks[0].Host)
	}
	if blocks[1].Host != "db" {
		t.Errorf("blocks[1].Host = %q, want 'db'", blocks[1].Host)
	}
}

func TestSSHConfigParseWildcardExtended(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "config")
	content := `Host *
    User defaultuser
    Port 22

Host special
    User root
`
	os.WriteFile(configPath, []byte(content), 0644)
	
	blocks := parseSSHConfig(configPath)
	if len(blocks) != 2 {
		t.Fatalf("expected 2 blocks, got %d", len(blocks))
	}
}

func TestSSHConfigParseCommentsExtended(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "config")
	content := `# This is a comment
Host myserver
    # Another comment
    HostName example.com
    Port 22
`
	os.WriteFile(configPath, []byte(content), 0644)
	
	blocks := parseSSHConfig(configPath)
	if len(blocks) != 1 {
		t.Fatalf("expected 1 block, got %d", len(blocks))
	}
	if blocks[0].HostName != "example.com" {
		t.Errorf("HostName = %q, want 'example.com'", blocks[0].HostName)
	}
}

func TestSSHConfigParseEmptyExtended(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "config")
	os.WriteFile(configPath, []byte(""), 0644)
	
	blocks := parseSSHConfig(configPath)
	if len(blocks) != 0 {
		t.Fatalf("expected 0 blocks, got %d", len(blocks))
	}
}

func TestSSHConfigParseMissingExtended(t *testing.T) {
	blocks := parseSSHConfig("/nonexistent/path/config")
	if blocks != nil {
		t.Fatalf("expected nil for missing file, got %d blocks", len(blocks))
	}
}

func TestSSHConfigMatchHostPatternExtended(t *testing.T) {
	tests := []struct {
		pattern string
		host    string
		want    bool
	}{
		{"*", "anything", true},
		{"myserver", "myserver", true},
		{"myserver", "other", false},
		{"*.example.com", "web.example.com", true},
		{"*.example.com", "db.example.com", true},
		{"*.example.com", "example.com", false},
		{"web?.example.com", "web1.example.com", true},
		{"web?.example.com", "web12.example.com", false},
		{"192.168.1.*", "192.168.1.100", true},
		{"192.168.1.*", "192.168.2.100", false},
	}
	
	for _, tt := range tests {
		t.Run(tt.pattern+"_"+tt.host, func(t *testing.T) {
			got := matchHostPattern(tt.pattern, tt.host)
			if got != tt.want {
				t.Errorf("matchHostPattern(%q, %q) = %v, want %v", tt.pattern, tt.host, got, tt.want)
			}
		})
	}
}

func TestSSHConfigMatchGlobExtended(t *testing.T) {
	tests := []struct {
		pattern string
		s       string
		want    bool
	}{
		{"abc", "abc", true},
		{"abc", "def", false},
		{"a*c", "abc", true},
		{"a*c", "a123c", true},
		{"a*c", "ac", true},
		{"a*c", "ab", false},
		{"a?c", "abc", true},
		{"a?c", "ac", false},
		{"a?c", "a12c", false},
		{"", "", true},
		{"*", "anything", true},
		{"*test*", "mytestfile", true},
	}
	
	for _, tt := range tests {
		t.Run(tt.pattern+"_"+tt.s, func(t *testing.T) {
			got := matchGlob(tt.pattern, tt.s)
			if got != tt.want {
				t.Errorf("matchGlob(%q, %q) = %v, want %v", tt.pattern, tt.s, got, tt.want)
			}
		})
	}
}

func TestSSHConfigCaseInsensitiveExtended(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "config")
	content := `Host myserver
    hostname Example.COM
    PORT 2222
    USER admin
`
	os.WriteFile(configPath, []byte(content), 0644)
	
	blocks := parseSSHConfig(configPath)
	if len(blocks) != 1 {
		t.Fatalf("expected 1 block, got %d", len(blocks))
	}
	if blocks[0].HostName != "Example.COM" {
		t.Errorf("HostName = %q, want 'Example.COM'", blocks[0].HostName)
	}
	if blocks[0].Port != 2222 {
		t.Errorf("Port = %d, want 2222", blocks[0].Port)
	}
	if blocks[0].User != "admin" {
		t.Errorf("User = %q, want 'admin'", blocks[0].User)
	}
}

func TestSSHConfigLoadSSHConfigExtended(t *testing.T) {
	dir := t.TempDir()
	sshDir := filepath.Join(dir, ".ssh")
	os.MkdirAll(sshDir, 0700)
	configPath := filepath.Join(sshDir, "config")
	content := `Host myserver
    HostName 10.0.0.1
    Port 2222
    User admin
`
	os.WriteFile(configPath, []byte(content), 0644)
	
	origHome := os.Getenv("HOME")
	os.Setenv("HOME", dir)
	defer os.Setenv("HOME", origHome)
	
	cfg := loadSSHConfig("myserver")
	if cfg == nil {
		t.Fatal("expected config for myserver, got nil")
	}
	if cfg.HostName != "10.0.0.1" {
		t.Errorf("HostName = %q, want '10.0.0.1'", cfg.HostName)
	}
	if cfg.Port != 2222 {
		t.Errorf("Port = %d, want 2222", cfg.Port)
	}
	if cfg.User != "admin" {
		t.Errorf("User = %q, want 'admin'", cfg.User)
	}
}

func TestSSHConfigLoadNoMatchExtended(t *testing.T) {
	dir := t.TempDir()
	sshDir := filepath.Join(dir, ".ssh")
	os.MkdirAll(sshDir, 0700)
	configPath := filepath.Join(sshDir, "config")
	content := `Host myserver
    HostName 10.0.0.1
`
	os.WriteFile(configPath, []byte(content), 0644)
	
	origHome := os.Getenv("HOME")
	os.Setenv("HOME", dir)
	defer os.Setenv("HOME", origHome)
	
	cfg := loadSSHConfig("otherhost")
	if cfg != nil {
		t.Errorf("expected nil for non-matching host, got %+v", cfg)
	}
}

func TestSSHConfigParseHostsExtended(t *testing.T) {
	dir := t.TempDir()
	sshDir := filepath.Join(dir, ".ssh")
	os.MkdirAll(sshDir, 0700)
	configPath := filepath.Join(sshDir, "config")
	content := `Host *
    User default

Host web1 web2
    HostName example.com

Host db
    HostName db.example.com
`
	os.WriteFile(configPath, []byte(content), 0644)
	
	origHome := os.Getenv("HOME")
	os.Setenv("HOME", dir)
	defer os.Setenv("HOME", origHome)
	
	hosts := parseSSHConfigHosts()
	// 应该有 web1, web2, db（不含 *）
	if len(hosts) < 3 {
		t.Errorf("expected at least 3 hosts, got %d: %v", len(hosts), hosts)
	}
}
