# sshell 新功能实现

## 已实现的新功能

### 1. 多跳板机链式跳转 (-J)

支持通过多个跳板机链式连接到目标主机。

**用法:**
```bash
# 单跳板机（原有功能）
sshell -u user -J bastion target

# 多跳板机链式跳转（新功能）
sshell -u user -J b1,b2,b3 target

# 支持 user@host:port 格式
sshell -u user -J admin@b1:2222,b2 root@target

# 结合 Agent 转发
sshell -u user -A -J b1,b2,b3 target
```

**实现细节:**
- 解析逗号分隔的跳板机列表
- 支持 `user@host:port` 格式
- 链式连接：local → jump1 → jump2 → ... → target
- 优雅关闭：从后往前关闭所有连接
- 详细模式显示连接进度

**相关文件:**
- `args.go`: `parseJumpHosts()` 解析跳板机列表
- `connect.go`: `connectViaMultiJump()`, `buildJumpChain()`, `closeJumpChain()`

### 2. 远程文件编辑 (--edit)

用本地编辑器编辑远程文件，自动下载→编辑→上传。

**用法:**
```bash
# 编辑远程文件
sshell -u user --edit /etc/nginx/nginx.conf host

# 指定编辑器
EDITOR=code sshell -u user --edit /path/to/file host

# 使用 VS Code
VISUAL="code --wait" sshell -u user --edit /path/to/file host
```

**功能特性:**
- 自动下载远程文件到临时目录
- 用本地编辑器打开（支持 EDITOR/VISUAL 环境变量）
- 编辑后自动上传（仅在文件有变化时）
- SHA256 哈希比较检测变化
- 保留原始文件权限
- 优先 SFTP，回退 SCP
- 临时文件自动清理

**编辑器优先级:**
1. `$EDITOR` 环境变量
2. `$VISUAL` 环境变量
3. vim
4. vi
5. nano
6. emacs

**相关文件:**
- `edit.go`: `remoteEdit()`, `getEditor()`, `fileSHA256()`
- `main.go`: `--edit` 选项处理

## 使用示例

### 多跳板机场景

```bash
# 企业网络：通过多个安全域跳转
sshell -u admin -J dmz-jump,internal-jump 10.0.0.100

# 带端口和用户
sshell -u root -J user1@jump1:2222,user2@jump2:2223 10.0.0.100

# 多主机批量执行
sshell -u root -J jump1,jump2 "host1,host2,host3" "df -h"

# 结合端口转发
sshell -u root -J jump1,jump2 -L 8080:localhost:80 10.0.0.100
```

### 远程编辑场景

```bash
# 编辑配置文件
sshell -u root --edit /etc/nginx/nginx.conf web-server

# 编辑脚本
sshell -u deploy --edit /opt/app/deploy.sh app-server

# 用 VS Code 编辑
VISUAL="code --wait" sshell -u dev --edit ~/project/config.yaml dev-server
```

## 测试建议

### 多跳板机测试
```bash
# 1. 设置测试环境（需要 3 台机器或虚拟机）
# 2. 测试基本连接
sshell -u user -J jump1,jump2 target "hostname"

# 3. 测试交互模式
sshell -u user -J jump1,jump2 target

# 4. 测试文件传输
sshell -u user -J jump1,jump2 --put ./local.txt:/tmp/ target

# 5. 测试端口转发
sshell -u user -J jump1,jump2 -L 8080:localhost:80 target
```

### 远程编辑测试
```bash
# 1. 测试基本编辑
sshell -u user --edit /tmp/test.txt host

# 2. 测试无变化退出
sshell -u user --edit /tmp/test.txt host  # 不修改保存

# 3. 测试权限保留
sshell -u root --edit /etc/shadow host  # 需要 root 权限

# 4. 测试编辑器切换
EDITOR=nano sshell -u user --edit /tmp/test.txt host
```

## 注意事项

1. **多跳板机**: 每个跳板机都需要有效的认证
2. **远程编辑**: 需要本地安装编辑器，且能处理终端输入
3. **文件权限**: SFTP 模式能更好地保留权限
4. **超时**: 多跳连接可能需要更长的超时时间
