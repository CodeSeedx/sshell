package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// config 表示配置文件的结构
type config struct {
	DefaultUser  string `json:"default_user"`
	DefaultPort  uint16 `json:"default_port"`
	DefaultAuth  string `json:"default_auth"`
	DefaultAlive uint32 `json:"default_alive"`
	Verbose      bool   `json:"verbose"`
}

// configDir 返回配置文件所在目录（多平台兼容）
func configDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".sshell"), nil
}

// configFile 返回配置文件路径
func configFile() (string, error) {
	dir, err := configDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "config"), nil
}

// loadConfig 从配置文件加载配置，如果文件不存在则返回空配置
func loadConfig() config {
	var c config
	path, err := configFile()
	if err != nil {
		return c
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return c
	}
	if err := json.Unmarshal(data, &c); err != nil {
		fmt.Fprintf(os.Stderr, "[sshell] Warning: config file parse error: %v\n", err)
	}
	return c
}

// applyConfig 将配置文件的值应用到 args，仅当命令行参数未设置时
// 同时设置默认值（如果配置文件和命令行都没有设置）
func applyConfig(a *args, c config) {
	if !a.cliUser && a.user == "" && c.DefaultUser != "" {
		a.user = c.DefaultUser
	}
	if !a.cliPort && a.port == 0 && c.DefaultPort != 0 {
		a.port = c.DefaultPort
	}
	if !a.cliAuth && a.auth == "" && c.DefaultAuth != "" {
		// 展开路径中的 ~ 前缀
		auth := c.DefaultAuth
		if len(auth) >= 2 && auth[0] == '~' && auth[1] == '/' {
			if home, err := os.UserHomeDir(); err == nil {
				auth = filepath.Join(home, auth[2:])
			}
		}
		a.auth = auth
	}
	if !a.cliAlive && a.alive == 0 && c.DefaultAlive != 0 {
		a.alive = c.DefaultAlive
	}
	if !a.verbose && c.Verbose {
		a.verbose = c.Verbose
	}

	// 设置默认值（如果配置文件和命令行都没有设置）
	if a.port == 0 {
		a.port = 22
	}
	if a.alive == 0 {
		a.alive = 30
	}
}

// saveConfig 保存配置到文件（原子写入）
func saveConfig(c config) error {
	dir, err := configDir()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(dir, 0700); err != nil {
		return err
	}
	path, err := configFile()
	if err != nil {
		return err
	}
	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return err
	}
	tmpPath := path + ".tmp"
	if err := os.WriteFile(tmpPath, data, 0600); err != nil {
		return err
	}
	return os.Rename(tmpPath, path)
}