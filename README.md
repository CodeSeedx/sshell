sshell
=========

[![Go](https://img.shields.io/badge/Go-1.25+-00ADD8?logo=go&logoColor=white)](https://go.dev/)
[![License](https://img.shields.io/badge/License-MIT-green.svg)](https://opensource.org/licenses/MIT)
[![GitHub Release](https://img.shields.io/github/v/release/xiatianxuan/sshell)](https://github.com/xiatianxuan/sshell/releases)
[![Go Report Card](https://goreportcard.com/badge/github.com/xiatianxuan/sshell)](https://goreportcard.com/report/github.com/xiatianxuan/sshell)
[![Build](https://img.shields.io/github/actions/workflow/status/xiatianxuan/sshell/ci.yml?label=CI)](https://github.com/xiatianxuan/sshell/actions)

轻量级 SSH 客户端，Go 语言编写，单文件输出，无外部运行时依赖。

下载
----

从 [GitHub Releases](https://github.com/xiatianxuan/sshell/releases) 下载对应平台的二进制文件：

- sshell-linux-amd64   — Linux x86_64
- sshell-linux-arm64   — Linux ARM64
- sshell-darwin-amd64  — macOS Intel
- sshell-darwin-arm64  — macOS Apple Silicon
- sshell-windows-amd64.exe — Windows

编译
----

    make build          # 编译当前平台
    make build-all      # 编译全部 5 个平台

用法
----

    sshell -u <user> <host> [options]

    sshell -u root 192.168.1.100          # 用 root 登录，密钥自动探测
    sshell -u root -p 2222 192.168.1.100  # 指定端口
    sshell -u root -a ./id_rsa myhost     # 指定密钥文件
    sshell -u root -a mypassword myhost   # 直接传密码
    sshell -u root -v 192.168.1.100       # 详细输出
    sshell -V                             # 查看版本

选项
----

    -p <port>     SSH 端口，默认 22
    -u <user>     SSH 用户名，必填
    -a <auth>     认证方式：密码或私钥文件路径
    -k <seconds>  TCP Keep-Alive 间隔，默认 30 秒
    -v            详细输出
    -V            查看版本
    -h, --help    帮助

功能
----

- SSH 连接：TCP 连接 + SSH 握手 + Keep-Alive
- 交互终端：PTY 模式，支持窗口缩放
- 密钥认证：自动探测 ~/.ssh/ 下的密钥文件
- 密码认证：自动提示输入密码，支持带密码的密钥
- 主机校验：读取 ~/.ssh/known_hosts，首次连接自动接受并提示
- 信号处理：Ctrl+C 正确转发，SIGWINCH 动态调整终端大小
- 空闲检测：30 分钟无活动自动退出

架构
----

    main.go      入口
    args.go      参数解析
    auth.go      认证逻辑
    connect.go   SSH 连接
    shell.go     交互终端
    version.go   版本号（编译时注入）

发版流程
----

向 main 分支推送 tag 即可自动发布：

    git tag v1.0.0
    git push origin v1.0.0

GitHub Actions 会自动构建 5 个平台的二进制文件并发布到 Release。

依赖
----

    golang.org/x/crypto
    golang.org/x/term
