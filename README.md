# SSH SOCKS5 和 HTTP 代理

这是一个基于 SSH 隧道的代理服务器实现，支持 SOCKS5 和 HTTP 代理协议。它允许你通过 SSH 服务器建立本地代理服务，类似于 `ssh -D` 的功能，并额外提供 HTTP 代理支持。

## 功能特点

- 支持 SOCKS5 协议
- 支持 HTTP/HTTPS 代理
- 支持 SSH 密钥认证和密码认证
- 支持 IPv4、IPv6 和域名解析
- 支持自定义本地监听地址和端口
- 自动查找用户目录下的 SSH 密钥

## 快速开始

### 编译

1. 确保已安装 Go 1.18 或更高版本
2. 克隆并编译项目：

```bash
# 克隆仓库
git clone https://github.com/galaxy8691/sp
cd sp

# 编译
go build -o sp
```

跨平台编译：

```bash
# Linux 版本
GOOS=linux GOARCH=amd64 go build -o sp-linux-amd64

# Windows 版本
GOOS=windows GOARCH=amd64 go build -o sp-windows-amd64.exe

# macOS 版本
GOOS=darwin GOARCH=amd64 go build -o sp-darwin-amd64
```

### 基本用法

最简单的启动方式：

```bash
./sp -user your-ssh-username -host your-ssh-server.com
```

## 详细配置

### 命令行参数

- `-user`: SSH 用户名
- `-key`: SSH 私钥文件路径（可选，默认自动查找）
- `-p`: 使用密码认证
- `-host`: SSH 服务器地址
- `-port`: SSH 服务器端口（默认 22）
- `-bind`: 本地监听地址（默认 127.0.0.1）
- `-lport`: SOCKS5 代理端口（默认 1080）
- `-http-port`: HTTP 代理端口（默认 8080）

### SSH 认证方式

#### 密钥认证

程序会自动在用户的 `.ssh` 目录下查找以下密钥文件：

- `id_rsa`
- `id_ed25519`
- `id_ecdsa`
- `id_dsa`

使用指定密钥文件：

```bash
./sp -user your-username \
     -key /path/to/your/private/key \
     -host your-ssh-server.com
```

#### 密码认证

```bash
./sp -user your-username -p -host your-ssh-server.com
```

## 使用示例

1. 使用自动查找的密钥：

   ```bash
   ./sp -user john -host example.com
   ```

2. 自定义代理端口：
   ```bash
   ./sp -user john -host example.com -lport 1234 -http-port 8888
   ```

### 客户端配置

配置你的应用程序使用以下代理设置：

SOCKS5 代理：

- 地址：127.0.0.1
- 端口：1080（或自定义端口）

HTTP 代理：

- 地址：127.0.0.1
- 端口：8080（或自定义端口）

## 注意事项

- 程序会自动在 `~/.ssh` 目录下查找常用的密钥文件
- 如果找不到密钥文件，可以使用 `-key` 参数指定密钥路径
- 使用密码认证时，密码会以安全的方式读取（不会显示在屏幕上）
- 确保 SSH 服务器允许端口转发
- HTTP 代理会自动通过 SOCKS5 代理转发请求，形成链式代理

## 致谢

本项目 100% 使用 [Cursor](https://cursor.sh/) AI 辅助编程工具开发完成。感谢 Cursor AI 提供的智能编程支持。
