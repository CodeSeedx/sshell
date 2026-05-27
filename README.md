# sshell-go

一个轻量级的 SSH 客户端工具，使用 Go 语言编写。

## 功能特性

- 支持密码认证和密钥认证
- 交互式 PTY 终端
- TCP Keep-Alive 连接保持
- 详细日志模式
- 自动探测 SSH 密钥文件

## 安装

```bash
go build -o sshell-go
```

## 使用方法

```bash
# 使用密码连接
./sshell-go -u root -a yourpassword 192.168.1.100

# 使用密钥文件连接
./sshell-go -u root -a ~/.ssh/id_rsa 192.168.1.100

# 指定端口和详细模式
./sshell-go -u root -p 2222 -v 192.168.1.100
```

## 参数说明

| 参数 | 说明 | 默认值 |
|------|------|--------|
| `-u` | SSH 用户名 (必填) | - |
| `-p` | SSH 端口 | 22 |
| `-a` | 密码或私钥文件路径 | 自动探测 |
| `-k` | Keep-Alive 间隔 (秒) | 30 |
| `-v` | 详细输出模式 | 关闭 |

## 认证方式

1. **密钥文件**: 通过 `-a` 参数指定私钥文件路径
2. **密码认证**: 通过 `-a` 参数直接传入密码
3. **自动探测**: 未指定 `-a` 时，自动查找 `~/.ssh/` 下的密钥文件 (id_ed25519, id_rsa, id_ecdsa)
4. **交互式输入**: 以上方式均未找到时，提示输入密码

## 示例

```bash
# 查看帮助
./sshell-go -h

# 使用自动探测密钥
./sshell-go -u admin 10.0.0.1

# 使用密码并开启详细模式
./sshell-go -u root -a mypassword -v myserver.com
```

## 许可证

MIT License - 详见 [LICENSE](LICENSE) 文件
