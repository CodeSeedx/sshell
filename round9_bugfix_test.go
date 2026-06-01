package main

import (
	"os"
	"testing"
)

// ==================== 第 9 轮：SSH 配置解析边界条件 ====================

// Bug #42: SSH 配置中的 Include 指令
func TestSSHConfigIncludeDirectiveR9(t *testing.T) {
	dir := t.TempDir()
	configPath := dir + "/config"
	content := `Include config.d/*
Host myserver
    HostName example.com
`
	os.WriteFile(configPath, []byte(content), 0644)
	
	blocks := parseSSHConfig(configPath)
	if len(blocks) != 1 {
		t.Fatalf("expected 1 block, got %d", len(blocks))
	}
	if blocks[0].Host != "myserver" {
		t.Errorf("Host = %q, want 'myserver'", blocks[0].Host)
	}
}

// Bug #43: SSH 配置中的 Match 指令
func TestSSHConfigMatchDirectiveR9(t *testing.T) {
	dir := t.TempDir()
	configPath := dir + "/config"
	content := `Match host myserver
    User admin

Host myserver
    HostName example.com
`
	os.WriteFile(configPath, []byte(content), 0644)
	
	blocks := parseSSHConfig(configPath)
	if len(blocks) != 1 {
		t.Fatalf("expected 1 block, got %d", len(blocks))
	}
}

// Bug #44: SSH 配置中的多个 Host 名称
func TestSSHConfigMultipleHostNamesR9(t *testing.T) {
	dir := t.TempDir()
	configPath := dir + "/config"
	content := `Host web1 web2 web3
    HostName example.com
    Port 22
`
	os.WriteFile(configPath, []byte(content), 0644)
	
	blocks := parseSSHConfig(configPath)
	if len(blocks) != 1 {
		t.Fatalf("expected 1 block, got %d", len(blocks))
	}
	if blocks[0].Host != "web1 web2 web3" {
		t.Errorf("Host = %q, want 'web1 web2 web3'", blocks[0].Host)
	}
}

// Bug #45: SSH 配置中的全局选项（无 Host 块）
func TestSSHConfigGlobalOptionsR9(t *testing.T) {
	dir := t.TempDir()
	configPath := dir + "/config"
	content := `User globaluser
Port 2222

Host myserver
    HostName example.com
`
	os.WriteFile(configPath, []byte(content), 0644)
	
	blocks := parseSSHConfig(configPath)
	if len(blocks) != 1 {
		t.Fatalf("expected 1 block, got %d", len(blocks))
	}
}

// Bug #46: SSH 配置中的空行和注释
func TestSSHConfigEmptyLinesAndCommentsR9(t *testing.T) {
	dir := t.TempDir()
	configPath := dir + "/config"
	content := `# Comment 1

# Comment 2
Host myserver
    # Comment 3
    HostName example.com
    
# Comment 4
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

// Bug #47: SSH 配置中的大小写混合
func TestSSHConfigMixedCaseR9(t *testing.T) {
	dir := t.TempDir()
	configPath := dir + "/config"
	content := `HOST myserver
    hostname Example.COM
    PORT 2222
    USER admin
    identityfile ~/.ssh/mykey
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
	if blocks[0].IdentityFile != "~/.ssh/mykey" {
		t.Errorf("IdentityFile = %q, want '~/.ssh/mykey'", blocks[0].IdentityFile)
	}
}

// Bug #48: SSH 配置中的 Host *
func TestSSHConfigHostWildcardR9(t *testing.T) {
	dir := t.TempDir()
	configPath := dir + "/config"
	content := `Host *
    User defaultuser
    Port 22

Host myserver
    HostName example.com
`
	os.WriteFile(configPath, []byte(content), 0644)
	
	blocks := parseSSHConfig(configPath)
	if len(blocks) != 2 {
		t.Fatalf("expected 2 blocks, got %d", len(blocks))
	}
	if blocks[0].Host != "*" {
		t.Errorf("blocks[0].Host = %q, want '*'", blocks[0].Host)
	}
	if blocks[1].Host != "myserver" {
		t.Errorf("blocks[1].Host = %q, want 'myserver'", blocks[1].Host)
	}
}

// Bug #50: SSH 配置中的端口范围
func TestSSHConfigPortRangeR9(t *testing.T) {
	dir := t.TempDir()
	configPath := dir + "/config"
	tests := []struct {
		name    string
		content string
		want    uint16
	}{
		{"normal", "Host h\n    Port 22\n", 22},
		{"high", "Host h\n    Port 65535\n", 65535},
		{"low", "Host h\n    Port 1\n", 1},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			os.WriteFile(configPath, []byte(tt.content), 0644)
			blocks := parseSSHConfig(configPath)
			if len(blocks) != 1 {
				t.Fatalf("expected 1 block, got %d", len(blocks))
			}
			if blocks[0].Port != tt.want {
				t.Errorf("Port = %d, want %d", blocks[0].Port, tt.want)
			}
		})
	}
}
