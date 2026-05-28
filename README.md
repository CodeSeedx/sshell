sshell
=========

轻量级 SSH 客户端，Go 语言编写，单文件输出，无外部运行时依赖。

编译
----

    go build -o sshell .

用法
----

    sshell -u <user> <host> [options]

    sshell -u root 192.168.1.100          # 用 root 登录，密钥自动探测
    sshell -u root -p 2222 192.168.1.100  # 指定端口
    sshell -u root -a ./id_rsa myhost     # 指定密钥文件
    sshell -u root -a mypassword myhost   # 直接传密码
    sshell -u root -v 192.168.1.100       # 详细输出

选项
----

    -p <port>     SSH 端口，默认 22
    -u <user>     SSH 用户名，必填
    -a <auth>     认证方式：密码或私钥文件路径
    -k <seconds>  TCP Keep-Alive 间隔，默认 30 秒
    -v            详细输出
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

依赖
----

    golang.org/x/crypto
    golang.org/x/term
