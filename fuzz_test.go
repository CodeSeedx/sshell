package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// splitArgs 将 null 分隔的字符串拆分为参数切片（fuzz 测试用）
func splitArgs(s string) []string {
	if s == "" {
		return nil
	}
	return strings.Split(s, "\x00")
}

// ==================== parseArgsFrom Fuzz 测试 ====================

func FuzzParseArgsFrom(f *testing.F) {
	f.Add("-u\x00root\x00192.168.1.1")
	f.Add("-u\x00admin\x00-p\x002222\x00host")
	f.Add("-h")
	f.Add("-V")
	f.Add("")
	f.Add("host")
	f.Add("-p\x00abc\x00host")
	f.Add("--help")
	f.Add("--version")
	f.Add("-u\x00\x00host")
	f.Add("---")
	f.Add("-u\x00root\x00-p\x0022\x00-k\x0060\x00-a\x00pw\x00-v\x00h")

	f.Fuzz(func(t *testing.T, raw string) {
		rawArgs := splitArgs(raw)
		if len(rawArgs) > 20 {
			t.Skip("args too long")
		}
		for i, arg := range rawArgs {
			if len(arg) > 1000 {
				rawArgs[i] = arg[:1000]
			}
		}

		a, err := parseArgsFrom(rawArgs)
		if err == nil {
			if a.port > 65535 {
				t.Errorf("port %d exceeds uint16 max", a.port)
			}
		}
	})
}

// FuzzParseArgsFromVariants 对 parseArgsFrom 的组合变体模糊测试
func FuzzParseArgsFromVariants(f *testing.F) {
	// 每次 f.Add 都要提供 3 个字符串参数
	f.Add("-u", "root", "192.168.1.1")
	f.Add("-p", "2222", "host")
	f.Add("-k", "60", "host")
	f.Add("-a", "password", "host")
	f.Add("-v", "host", "")
	f.Add("--help", "", "")
	f.Add("--version", "", "")
	f.Add("host", "", "")
	f.Add("", "", "")

	f.Fuzz(func(t *testing.T, arg1, arg2, arg3 string) {
		combinations := [][]string{
			{arg1},
			{arg1, arg2},
			{arg1, arg2, arg3},
			{"-u", arg1, arg2, arg3},
			{"-p", arg1, arg2, arg3},
			{"-k", arg1, arg2, arg3},
			{"-a", arg1, arg2, arg3},
			{"-u", arg1, "-p", arg2, "-k", arg3, "host"},
			{arg1, "-u", arg2, "-p", arg3},
		}

		for _, testArgs := range combinations {
			for i := range testArgs {
				if len(testArgs[i]) > 200 {
					testArgs[i] = testArgs[i][:200]
				}
			}
			parseArgsFrom(testArgs)
		}
	})
}

// ==================== applyConfig Fuzz 测试 ====================

// FuzzApplyConfig 使用 JSON 序列化来传递 args/config 结构体
func FuzzApplyConfig(f *testing.F) {
	f.Add(`{"user":"root"}`, `{"default_user":"admin","default_port":3322}`)
	f.Add(`{}`, `{}`)
	f.Add(`{"port":22}`, `{"default_port":3322}`)
	f.Add(`{"alive":30}`, `{"default_alive":120}`)
	f.Add(`{"verbose":true}`, `{"verbose":true}`)
	f.Add(`{}`, `{"default_user":"admin","default_port":3322,"default_auth":"/key","default_alive":120,"verbose":true}`)

	f.Fuzz(func(t *testing.T, argsJSON, configJSON string) {
		var a args
		var c config

		// 尝试解析 args JSON
		if err := json.Unmarshal([]byte(argsJSON), &a); err != nil {
			t.Skip("invalid args JSON")
		}
		// 尝试解析 config JSON（自定义解析）
		if err := json.Unmarshal([]byte(configJSON), &c); err != nil {
			t.Skip("invalid config JSON")
		}

		origUser := a.user
		origPort := a.port
		origAuth := a.auth
		origAlive := a.alive
		origVerbose := a.verbose

		applyConfig(&a, c)

		if origUser != "" && a.user != origUser {
			t.Errorf("user changed from %q to %q", origUser, a.user)
		}
		if origPort != 0 && a.port != origPort {
			t.Errorf("port changed from %d to %d", origPort, a.port)
		}
		if origAuth != "" && a.auth != origAuth {
			t.Errorf("auth changed from %q to %q", origAuth, a.auth)
		}
		if origAlive != 0 && a.alive != origAlive {
			t.Errorf("alive changed from %d to %d", origAlive, a.alive)
		}
		if origVerbose && !a.verbose {
			t.Error("verbose changed from true to false")
		}
		if a.port == 0 {
			t.Error("port should not be 0 after applyConfig")
		}
		if a.alive == 0 {
			t.Error("alive should not be 0 after applyConfig")
		}
	})
}

// ==================== loadConfig JSON Fuzz 测试 ====================

func FuzzLoadConfigJSON(f *testing.F) {
	f.Add(`{}`)
	f.Add(`{"default_user": "admin"}`)
	f.Add(`{"default_port": 2222}`)
	f.Add(`{"default_alive": 120}`)
	f.Add(`{"verbose": true}`)
	f.Add(`{"default_user": "admin", "default_port": 2222, "default_auth": "/key", "default_alive": 120, "verbose": true}`)
	f.Add(`{"default_port": -1}`)
	f.Add(`{"default_port": 99999}`)
	f.Add(`{"verbose": null}`)
	f.Add(`{"default_user": 123}`)
	f.Add(`{"default_port": "abc"}`)
	f.Add(`{"extra_field": "value"}`)
	f.Add(`[1, 2, 3]`)
	f.Add(`null`)
	f.Add(`"just a string"`)
	f.Add(`12345`)
	f.Add(`true`)
	f.Add(`{"nested": {"deep": "value"}}`)

	f.Fuzz(func(t *testing.T, jsonData string) {
		if len(jsonData) > 10000 {
			jsonData = jsonData[:10000]
		}

		tmpDir := t.TempDir()
		sshDir := filepath.Join(tmpDir, ".sshell")
		os.MkdirAll(sshDir, 0700)

		configPath := filepath.Join(sshDir, "config")
		if err := os.WriteFile(configPath, []byte(jsonData), 0600); err != nil {
			t.Fatalf("write config file: %v", err)
		}

		origHome := os.Getenv("HOME")
		os.Setenv("HOME", tmpDir)
		defer os.Setenv("HOME", origHome)

		c := loadConfig()

		if c.DefaultPort > 65535 {
			t.Errorf("default_port %d exceeds uint16 max", c.DefaultPort)
		}

		var dummy config
		if err := json.Unmarshal([]byte(jsonData), &dummy); err != nil && !isEmptyConfig(c) {
			t.Error("JSON parse failed but config is not empty")
		}
	})
}

func isEmptyConfig(c config) bool {
	return c.DefaultUser == "" && c.DefaultPort == 0 && c.DefaultAuth == "" && c.DefaultAlive == 0 && !c.Verbose
}

// ==================== parseArgsFrom + applyConfig 组合 Fuzz 测试 ====================

func FuzzParseAndApply(f *testing.F) {
	f.Add("-u\x00root\x00192.168.1.1", `{"default_port": 2222}`)
	f.Add("-u\x00admin\x00-p\x0022\x00host", `{"default_alive": 120}`)
	f.Add("host", `{"default_user": "admin", "default_port": 3322}`)
	f.Add("-v\x00host", `{"verbose": true}`)
	f.Add("", `{}`)
	f.Add("-u\x00root\x00-p\x00abc\x00host", `{"default_port": 22}`)

	f.Fuzz(func(t *testing.T, rawArgs string, jsonData string) {
		rawArgsSlice := splitArgs(rawArgs)
		if len(rawArgsSlice) > 20 {
			t.Skip("args too long")
		}
		if len(jsonData) > 10000 {
			jsonData = jsonData[:10000]
		}
		for i := range rawArgsSlice {
			if len(rawArgsSlice[i]) > 1000 {
				rawArgsSlice[i] = rawArgsSlice[i][:1000]
			}
		}

		tmpDir := t.TempDir()
		sshDir := filepath.Join(tmpDir, ".sshell")
		os.MkdirAll(sshDir, 0700)

		configPath := filepath.Join(sshDir, "config")
		if err := os.WriteFile(configPath, []byte(jsonData), 0600); err != nil {
			t.Fatalf("write config file: %v", err)
		}

		origHome := os.Getenv("HOME")
		os.Setenv("HOME", tmpDir)
		defer os.Setenv("HOME", origHome)

		c := loadConfig()
		a, parseErr := parseArgsFrom(rawArgsSlice)

		if parseErr == nil {
			applyConfig(&a, c)

			if a.port == 0 {
				t.Error("port should not be 0 after applyConfig")
			}
			if a.alive == 0 {
				t.Error("alive should not be 0 after applyConfig")
			}
			if a.port > 65535 {
				t.Errorf("port %d exceeds uint16 max", a.port)
			}
		}
	})
}

// ==================== parseArgsWithConfig Fuzz 测试 ====================

func FuzzParseArgsWithConfig(f *testing.F) {
	f.Add("-u\x00root\x00192.168.1.1", `{"default_port": 2222}`)
	f.Add("host", `{"default_user": "admin", "default_port": 3322}`)
	f.Add("", `{}`)
	f.Add("-h", `{}`)
	f.Add("-V", `{}`)

	f.Fuzz(func(t *testing.T, rawArgs string, jsonData string) {
		rawArgsSlice := splitArgs(rawArgs)
		if len(rawArgsSlice) > 20 {
			t.Skip("args too long")
		}
		if len(jsonData) > 10000 {
			jsonData = jsonData[:10000]
		}
		for i := range rawArgsSlice {
			if len(rawArgsSlice[i]) > 1000 {
				rawArgsSlice[i] = rawArgsSlice[i][:1000]
			}
		}

		tmpDir := t.TempDir()
		sshDir := filepath.Join(tmpDir, ".sshell")
		os.MkdirAll(sshDir, 0700)

		configPath := filepath.Join(sshDir, "config")
		if err := os.WriteFile(configPath, []byte(jsonData), 0600); err != nil {
			t.Fatalf("write config file: %v", err)
		}

		origHome := os.Getenv("HOME")
		os.Setenv("HOME", tmpDir)
		defer os.Setenv("HOME", origHome)

		a, err := parseArgsWithConfig(rawArgsSlice)

		if err == nil {
			if a.port > 65535 {
				t.Errorf("port %d exceeds uint16 max", a.port)
			}
			if a.host == "" || a.user == "" {
				t.Error("should return error when host or user is empty")
			}
			if a.port == 0 {
				t.Error("port should not be 0")
			}
			if a.alive == 0 {
				t.Error("alive should not be 0")
			}
		}
	})
}

// ==================== 边界条件 Fuzz 测试 ====================

func FuzzEdgeCases(f *testing.F) {
	f.Add(uint16(0), uint32(0))
	f.Add(uint16(1), uint32(1))
	f.Add(uint16(22), uint32(30))
	f.Add(uint16(2222), uint32(120))
	f.Add(uint16(65535), uint32(3600))
	f.Add(uint16(65535), uint32(4294967295))

	f.Fuzz(func(t *testing.T, port uint16, alive uint32) {
		cliArgs := []string{"-u", "root", "-p", fmt.Sprintf("%d", port), "-k", fmt.Sprintf("%d", alive), "host"}
		a, err := parseArgsFrom(cliArgs)

		if err == nil {
			if a.port != port {
				t.Errorf("port mismatch: got %d, want %d", a.port, port)
			}
			if a.alive != alive {
				t.Errorf("alive mismatch: got %d, want %d", a.alive, alive)
			}
		}

		testArgs := args{port: port, alive: alive}
		c := config{DefaultPort: 22, DefaultAlive: 30}
		applyConfig(&testArgs, c)

		if testArgs.port == 0 {
			t.Error("port should not be 0")
		}
		if testArgs.alive == 0 {
			t.Error("alive should not be 0")
		}
	})
}
