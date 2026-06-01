package main

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// sshHostConfig 表示从 ~/.ssh/config 中解析出的一个 Host 配置块
type sshHostConfig struct {
	Host               string
	HostName           string
	Port               uint16
	User               string
	IdentityFile       string
	ProxyJump          string
	Compression        bool
	ServerAliveInterval int
	LocalForward       []string
	RemoteForward      []string
	DynamicForward     []string
}

// loadSSHConfig 从 ~/.ssh/config 中加载与 hostAlias 匹配的配置
// 返回 nil 表示没有匹配的配置
func loadSSHConfig(hostAlias string) *sshHostConfig {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil
	}
	configPath := filepath.Join(home, ".ssh", "config")
	blocks := parseSSHConfig(configPath)

	// OpenSSH 语义：第一个匹配的 Host 块生效
	// Host 指令可以包含多个名称（如 "Host web1 web2 web3"）
	// Host * 作为全局默认，被后续匹配的 Host 块覆盖
	var wildcardCfg *sshHostConfig
	for _, block := range blocks {
		// 检查是否是 Host * 通配符块
		isWildcard := false
		for _, name := range strings.Fields(block.Host) {
			if name == "*" {
				isWildcard = true
				break
			}
		}

		if isWildcard {
			// 记录 Host * 块，作为兜底默认
			cfg := block
			wildcardCfg = &cfg
			continue
		}

		// 检查 Host 指令中的每个名称是否匹配
		for _, name := range strings.Fields(block.Host) {
			if matchHostPattern(name, hostAlias) {
				cfg := block // 复制
				// 如果有 Host * 块，合并未设置的字段（Host * 作为默认）
				if wildcardCfg != nil {
					cfg = mergeSSHConfig(*wildcardCfg, cfg)
				}
				// 替换 %h 和 %p token
				if cfg.IdentityFile != "" {
					cfg.IdentityFile = replaceTokens(cfg.IdentityFile, hostAlias, "")
				}
				return &cfg
			}
		}
	}
	// 没有匹配的 Host 块，但可能有 Host * 块
	if wildcardCfg != nil {
		cfg := *wildcardCfg
		if cfg.IdentityFile != "" {
			cfg.IdentityFile = replaceTokens(cfg.IdentityFile, hostAlias, "")
		}
		return &cfg
	}
	return nil
}

// mergeSSHConfig 将 defaults 中未在 target 中设置的字段合并过来
func mergeSSHConfig(defaults, target sshHostConfig) sshHostConfig {
	if target.HostName == "" {
		target.HostName = defaults.HostName
	}
	if target.Port == 0 {
		target.Port = defaults.Port
	}
	if target.User == "" {
		target.User = defaults.User
	}
	if target.IdentityFile == "" {
		target.IdentityFile = defaults.IdentityFile
	}
	if target.ProxyJump == "" {
		target.ProxyJump = defaults.ProxyJump
	}
	if !target.Compression && defaults.Compression {
		target.Compression = defaults.Compression
	}
	if target.ServerAliveInterval == 0 {
		target.ServerAliveInterval = defaults.ServerAliveInterval
	}
	if len(target.LocalForward) == 0 {
		target.LocalForward = defaults.LocalForward
	}
	if len(target.RemoteForward) == 0 {
		target.RemoteForward = defaults.RemoteForward
	}
	if len(target.DynamicForward) == 0 {
		target.DynamicForward = defaults.DynamicForward
	}
	return target
}

// parseSSHConfig 解析 SSH config 文件，返回所有 Host 配置块
func parseSSHConfig(path string) []sshHostConfig {
	return parseSSHConfigDepth(path, 0)
}

// maxIncludeDepth 防止 Include 指令无限递归
const maxIncludeDepth = 10

// maxIncludeFiles 防止 Include 通配符匹配过多文件
const maxIncludeFiles = 50

// parseSSHConfigDepth 带深度限制的 SSH config 解析
func parseSSHConfigDepth(path string, depth int) []sshHostConfig {
	total := 0
	return parseSSHConfigDepthCount(path, depth, &total)
}

// parseSSHConfigDepthCount 带深度和文件总数限制的 SSH config 解析
func parseSSHConfigDepthCount(path string, depth int, totalFiles *int) []sshHostConfig {
	if depth > maxIncludeDepth {
		return nil
	}
	if *totalFiles >= maxIncludeFiles {
		return nil
	}
	*totalFiles++

	f, err := os.Open(path)
	if err != nil {
		return nil
	}
	defer f.Close()

	var blocks []sshHostConfig
	var current *sshHostConfig

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		// 分割 key 和 value（支持空格和 tab 分隔，OpenSSH config 允许两者）
		idx := strings.IndexAny(line, " \t")
		if idx < 0 {
			continue
		}
		key := strings.ToLower(strings.TrimSpace(line[:idx]))
		value := strings.TrimSpace(line[idx:])

		switch key {
		case "host":
			// 保存上一个块
			if current != nil {
				blocks = append(blocks, *current)
			}
			current = &sshHostConfig{Host: value}

		case "hostname":
			if current != nil {
				current.HostName = value
			}
		case "port":
			if current != nil {
				var p uint16
				fmt.Sscanf(value, "%d", &p)
				current.Port = p
			}
		case "user":
			if current != nil {
				current.User = value
			}
		case "identityfile":
			if current != nil {
				current.IdentityFile = value
			}
		case "proxyjump":
			if current != nil {
				current.ProxyJump = value
			}
		case "compression":
			if current != nil {
				current.Compression = strings.ToLower(value) == "yes"
			}
		case "serveraliveinterval":
			if current != nil {
				fmt.Sscanf(value, "%d", &current.ServerAliveInterval)
			}
		case "localforward":
			if current != nil {
				current.LocalForward = append(current.LocalForward, value)
			}
		case "remoteforward":
			if current != nil {
				current.RemoteForward = append(current.RemoteForward, value)
			}
		case "dynamicforward":
			if current != nil {
				current.DynamicForward = append(current.DynamicForward, value)
			}
		case "include":
			// Include 指令：递归解析其他配置文件，支持通配符
			// 相对路径相对于当前配置文件所在目录
			expanded := expandPath(value)
			if !filepath.IsAbs(expanded) {
				expanded = filepath.Join(filepath.Dir(path), expanded)
			}
			matches, _ := filepath.Glob(expanded)
			if matches == nil {
				// 没有通配符或无匹配，尝试单文件
				matches = []string{expanded}
			}
			for _, match := range matches {
				if included := parseSSHConfigDepthCount(match, depth+1, totalFiles); included != nil {
					blocks = append(blocks, included...)
				}
			}
		}
	}

	// 保存最后一个块
	if current != nil {
		blocks = append(blocks, *current)
	}

	if err := scanner.Err(); err != nil {
		return nil
	}

	return blocks
}

// matchHostPattern 检查 hostAlias 是否匹配 Host 模式（支持 * 和 ? 通配符）
func matchHostPattern(pattern, host string) bool {
	// Host * 匹配所有
	if pattern == "*" {
		return true
	}
	// 简单通配符匹配
	return matchGlob(pattern, host)
}

// matchGlob 简单的通配符匹配（* 匹配任意字符序列，? 匹配单个字符）
func matchGlob(pattern, s string) bool {
	pi, si := 0, 0
	starPi, starSi := -1, -1

	for si < len(s) {
		if pi < len(pattern) && (pattern[pi] == '?' || pattern[pi] == s[si]) {
			pi++
			si++
		} else if pi < len(pattern) && pattern[pi] == '*' {
			starPi = pi
			starSi = si
			pi++
		} else if starPi >= 0 {
			pi = starPi + 1
			starSi++
			si = starSi
		} else {
			return false
		}
	}

	for pi < len(pattern) && pattern[pi] == '*' {
		pi++
	}

	return pi == len(pattern)
}

// replaceTokens 替换 IdentityFile 中的 OpenSSH token
// 支持: %h (hostname), %l (local hostname), %u (local user), %r (remote user), %p (port), %% (literal %)
func replaceTokens(path, hostname, localuser string) string {
	path = strings.ReplaceAll(path, "%h", hostname)
	path = strings.ReplaceAll(path, "%l", localuser)
	path = strings.ReplaceAll(path, "%%", "%")
	// 展开 ~ 前缀
	if strings.HasPrefix(path, "~/") {
		if home, err := os.UserHomeDir(); err == nil {
			path = filepath.Join(home, path[2:])
		}
	}
	return path
}

// expandPath 展开路径中的 ~ 和环境变量
func expandPath(path string) string {
	if strings.HasPrefix(path, "~/") {
		if home, err := os.UserHomeDir(); err == nil {
			return filepath.Join(home, path[2:])
		}
	}
	return os.ExpandEnv(path)
}

// parseSSHConfigHosts 从 SSH config 中提取所有 Host 别名（用于 Tab 补全）
func parseSSHConfigHosts() []string {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil
	}
	configPath := filepath.Join(home, ".ssh", "config")
	blocks := parseSSHConfig(configPath)

	var hosts []string
	for _, block := range blocks {
		if block.Host == "*" {
			continue
		}
		// Host 指令可以包含多个名称
		for _, name := range strings.Fields(block.Host) {
			if name != "*" {
				hosts = append(hosts, name)
			}
		}
	}
	return hosts
}
