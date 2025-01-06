# SSH SOCKS5 和 HTTP 代理

这是一个基于 SSH 隧道的代理服务器实现，支持 SOCKS5 和 HTTP 代理协议。它允许你通过 SSH 服务器建立本地代理服务，类似于 `ssh -D` 的功能，并额外提供 HTTP 代理支持。

## 功能特点

- 支持 SOCKS5 协议
- 支持 HTTP/HTTPS 代理
- 支持 SSH 密钥认证和密码认证
- 支持 IPv4、IPv6 和域名解析
- 支持自定义本地监听地址和端口
- 自动查找用户目录下的 SSH 密钥

## 使用方法

### 使用密钥认证

程序会自动在用户的 `.ssh` 目录下查找以下密钥文件：
- `id_rsa`
- `id_ed25519`
- `id_ecdsa`
- `id_dsa`

1. 最简单的使用方式（自动使用 `~/.ssh` 目录下的密钥）：
   ```bash
   ./myapp -user your-ssh-username -host your-ssh-server.com
   ```

2. 指定自定义密钥文件：
   ```bash
   ./myapp -user your-ssh-username \
          -key /path/to/your/private/key \
          -host your-ssh-server.com \
          -port 22 \
          -bind 127.0.0.1 \
          -lport 1080 \
          -http-port 8080
   ```

### 使用密码认证

运行程序时添加 `-p` 参数：
```bash
./myapp -user your-ssh-username \
       -p \
       -host your-ssh-server.com \
       -port 22 \
       -bind 127.0.0.1 \
       -lport 1080 \
       -http-port 8080
```
程序会提示你输入密码。

参数说明：
- `-user`: SSH 用户名
- `-key`: SSH 私钥文件路径（可选，默认自动查找）
- `-p`: 使用密码认证
- `-host`: SSH 服务器地址
- `-port`: SSH 服务器端口（默认 22）
- `-bind`: 本地监听地址（默认 127.0.0.1）
- `-lport`: SOCKS5 代理端口（默认 1080）
- `-http-port`: HTTP 代理端口（默认 8080）

## 使用示例

1. 使用自动查找的密钥启动代理服务器：
   ```bash
   ./myapp -user john -host example.com
   ```

2. 使用指定的密钥文件：
   ```bash
   ./myapp -user john -key ~/.ssh/custom_key -host example.com
   ```

3. 使用密码认证：
   ```bash
   ./myapp -user john -p -host example.com
   ```

4. 自定义代理端口：
   ```bash
   ./myapp -user john -host example.com -lport 1234 -http-port 8888
   ```

5. 配置你的应用程序使用代理：

   SOCKS5 代理：
   - 代理地址：127.0.0.1
   - 代理端口：1080（或自定义的端口）

   HTTP 代理：
   - 代理地址：127.0.0.1
   - 代理端口：8080（或自定义的端口）

## 注意事项

- 程序会自动在 `~/.ssh` 目录下查找常用的密钥文件
- 如果找不到密钥文件，可以使用 `-key` 参数指定密钥路径
- 使用密码认证时，密码会以安全的方式读取（不会显示在屏幕上）
- 确保 SSH 服务器允许端口转发
- HTTP 代理会自动通过 SOCKS5 代理转发请求，形成链式代理 