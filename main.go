package main

import (
	"flag"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"syscall"
	"time"

	"golang.org/x/net/proxy"

	"golang.org/x/crypto/ssh"
	"golang.org/x/term"
)

var (
	sshHost     string
	sshPort     int
	sshUser     string
	sshKeyFile  string
	usePassword bool
	localAddr   string
	localPort   int
	httpPort    int
	sshClient    *ssh.Client
	sshConnMutex sync.Mutex
)

func init() {
	// 获取用户主目录
	homeDir, err := os.UserHomeDir()
	if err != nil {
		log.Printf("警告: 无法获取用户主目录: %v", err)
		homeDir = "."
	}

	// 尝试查找可用的SSH密钥
	sshDir := filepath.Join(homeDir, ".ssh")
	defaultKey := ""
	keyFiles := []string{"id_rsa", "id_ed25519", "id_ecdsa", "id_dsa"}
	for _, keyFile := range keyFiles {
		keyPath := filepath.Join(sshDir, keyFile)
		if _, err := os.Stat(keyPath); err == nil {
			if _, err := os.Stat(keyPath + ".pub"); err == nil {
				defaultKey = keyPath
				break
			}
		}
	}

	flag.StringVar(&sshHost, "host", "remote.example.com", "SSH server hostname")
	flag.IntVar(&sshPort, "port", 22, "SSH server port")
	flag.StringVar(&sshUser, "user", "", "SSH username")
	flag.StringVar(&sshKeyFile, "key", defaultKey, "SSH private key file (optional if using password)")
	flag.BoolVar(&usePassword, "p", false, "Use password authentication")
	flag.StringVar(&localAddr, "bind", "127.0.0.1", "Local bind address")
	flag.IntVar(&localPort, "lport", 1080, "Local port for SOCKS5")
	flag.IntVar(&httpPort, "http-port", 8080, "Local port for HTTP proxy")
}

func findSSHKey() (string, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("无法获取用户主目录: %v", err)
	}

	// 常见的SSH私钥文件名（不包含.pub后缀的文件）
	keyFiles := []string{
		"id_rsa",
		"id_ed25519",
		"id_ecdsa",
		"id_dsa",
	}

	sshDir := filepath.Join(homeDir, ".ssh")
	for _, keyFile := range keyFiles {
		keyPath := filepath.Join(sshDir, keyFile)
		// 检查私钥文件
		if _, err := os.Stat(keyPath); err == nil {
			// 确保不是公钥文件
			if _, err := os.Stat(keyPath + ".pub"); err == nil {
				// 找到了对应的私钥和公钥
				return keyPath, nil
			}
		}
	}

	return "", fmt.Errorf("在 %s 目录下未找到有效的SSH私钥文件（注意：不能使用.pub结尾的公钥文件）", sshDir)
}

func getPassword() (string, error) {
	fmt.Printf("Enter SSH password for %s: ", sshUser)
	password, err := term.ReadPassword(int(syscall.Stdin))
	fmt.Println() // 打印换行
	if err != nil {
		return "", fmt.Errorf("读取密码失败: %v", err)
	}
	return string(password), nil
}

// 新增函数：检查SSH连接并在需要时重连
func maintainSSHConnection(config *ssh.ClientConfig) {
	for {
		time.Sleep(30 * time.Second)
		
		sshConnMutex.Lock()
		if sshClient == nil {
			log.Printf("SSH 连接不存在，尝试重新连接...")
		} else {
			// 发送一个 keep-alive 消息来检查连接
			_, _, err := sshClient.SendRequest("keepalive@golang.org", true, nil)
			if err != nil {
				log.Printf("SSH 连接已断开，正在重新连接: %v", err)
				sshClient.Close()
				sshClient = nil
			} else {
				sshConnMutex.Unlock()
				continue
			}
		}

		// 尝试重新连接
		newClient, err := ssh.Dial("tcp", fmt.Sprintf("%s:%d", sshHost, sshPort), config)
		if err != nil {
			log.Printf("重新连接失败: %v", err)
			sshConnMutex.Unlock()
			continue
		}
		
		sshClient = newClient
		log.Printf("SSH 连接已重新建立")
		sshConnMutex.Unlock()
	}
}

func main() {
	flag.Parse()

	if sshUser == "" {
		log.Fatal("SSH username is required")
	}

	var config *ssh.ClientConfig
	if usePassword {
		password, err := getPassword()
		if err != nil {
			log.Fatal(err)
		}
		config = &ssh.ClientConfig{
			User: sshUser,
			Auth: []ssh.AuthMethod{
				ssh.Password(password),
			},
			HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		}
	} else {
		// 如果没有找到默认密钥或指定密钥，尝试自动查找
		if sshKeyFile == "" {
			var err error
			sshKeyFile, err = findSSHKey()
			if err != nil {
				log.Printf("警告: %v", err)
				log.Fatal("请使用 -key 指定SSH密钥文件或使用 -p 启用密码认证")
			}
		}
		
		log.Printf("使用SSH密钥: %s", sshKeyFile)

		// 使用密钥认证
		key, err := os.ReadFile(sshKeyFile)
		if err != nil {
			log.Printf("无法读取SSH私钥 %s: %v", sshKeyFile, err)
			log.Fatal("请确保密钥文件存在且有正确的访问权限")
		}

		signer, err := ssh.ParsePrivateKey(key)
		if err != nil {
			log.Printf("无法解析SSH私钥 %s: %v", sshKeyFile, err)
			log.Fatal("请确保使用了正确的私钥文件（不是.pub结尾的公钥文件）")
		}

		config = &ssh.ClientConfig{
			User: sshUser,
			Auth: []ssh.AuthMethod{
				ssh.PublicKeys(signer),
			},
			HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		}
	}

	// 初始连接
	var err error
	sshClient, err = ssh.Dial("tcp", fmt.Sprintf("%s:%d", sshHost, sshPort), config)
	if err != nil {
		log.Fatalf("无法连接到SSH服务器: %v", err)
	}

	// 启动连接维护协程
	go maintainSSHConnection(config)

	// 添加就绪标志
	socksReady := make(chan struct{})
	httpReady := make(chan struct{})
	
	// 启动SOCKS5服务器
	go func() {
		startSocks5Server(localAddr, localPort, sshClient)
		close(socksReady)
	}()
	
	// 启动HTTP代理服务器
	go func() {
		log.Printf("HTTP代理服务器正在监听 %s:%d", localAddr, httpPort)
		if err := startHttpProxy(localAddr, httpPort, fmt.Sprintf("%s:%d", localAddr, localPort)); err != nil {
			log.Printf("启动HTTP代理服务器失败: %v", err)
		}
		close(httpReady)
	}()
	
	// 等待两个服务都就绪
	<-socksReady
	<-httpReady
	
	log.Printf("所有服务已就绪")
	
	// 保持程序运行
	select {}
}

func startSocks5Server(addr string, port int, sshClient *ssh.Client) {
	listener, err := net.Listen("tcp", fmt.Sprintf("%s:%d", addr, port))
	if err != nil {
		log.Fatalf("无法启动SOCKS5服务器: %v", err)
	}

	log.Printf("SOCKS5代理服务器正在监听 %s:%d", addr, port)

	for {
		conn, err := listener.Accept()
		if err != nil {
			log.Printf("接受连接错误: %v", err)
			continue
		}
		go handleConnection(conn, sshClient)
	}
}

func startHttpProxy(addr string, port int, socks5Addr string) error {
	// 创建到SOCKS5代理的拨号器
	dialer, err := proxy.SOCKS5("tcp", socks5Addr, nil, proxy.Direct)
	if err != nil {
		return fmt.Errorf("创建SOCKS5拨号器失败: %v", err)
	}

	// 创建HTTP代理处理器
	handler := &httpProxyHandler{
		dialer: dialer,
	}

	server := &http.Server{
		Addr:    fmt.Sprintf("%s:%d", addr, port),
		Handler: handler,
	}

	return server.ListenAndServe()
}

type httpProxyHandler struct {
	dialer proxy.Dialer
}

func (h *httpProxyHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodConnect {
		h.handleHttps(w, r)
	} else {
		h.handleHttp(w, r)
	}
}

func (h *httpProxyHandler) handleHttp(w http.ResponseWriter, r *http.Request) {
	// 创建到目标服务器的连接
	conn, err := h.dialer.Dial("tcp", r.Host)
	if err != nil {
		http.Error(w, err.Error(), http.StatusServiceUnavailable)
		return
	}
	defer conn.Close()

	// 发送请求到目标服务器
	err = r.Write(conn)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// 将响应发送回客户端
	hj, ok := w.(http.Hijacker)
	if !ok {
		http.Error(w, "不支持hijacking", http.StatusInternalServerError)
		return
	}

	clientConn, _, err := hj.Hijack()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer clientConn.Close()

	// 双向转发数据
	var wg sync.WaitGroup
	wg.Add(2)

	go func() {
		defer wg.Done()
		io.Copy(clientConn, conn)
	}()

	go func() {
		defer wg.Done()
		io.Copy(conn, clientConn)
	}()

	wg.Wait()
}

func (h *httpProxyHandler) handleHttps(w http.ResponseWriter, r *http.Request) {
	// 连接目标服务器
	conn, err := h.dialer.Dial("tcp", r.Host)
	if err != nil {
		http.Error(w, err.Error(), http.StatusServiceUnavailable)
		return
	}

	// 发送 200 Connection Established 到客户端
	w.WriteHeader(http.StatusOK)

	// 获取客户端连接
	hj, ok := w.(http.Hijacker)
	if !ok {
		http.Error(w, "不支持hijacking", http.StatusInternalServerError)
		return
	}

	clientConn, _, err := hj.Hijack()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// 双向转发数据
	var wg sync.WaitGroup
	wg.Add(2)

	go func() {
		defer wg.Done()
		defer conn.Close()
		io.Copy(conn, clientConn)
	}()

	go func() {
		defer wg.Done()
		defer clientConn.Close()
		io.Copy(clientConn, conn)
	}()

	wg.Wait()
}

func handleConnection(client net.Conn, sshConn *ssh.Client) {
	defer client.Close()

	// 添加重试次数
	maxRetries := 3
	for retry := 0; retry < maxRetries; retry++ {
		sshConnMutex.Lock()
		currentSSH := sshClient
		sshConnMutex.Unlock()
		
		if currentSSH == nil {
			log.Printf("等待 SSH 连接重新建立...")
			time.Sleep(time.Second * time.Duration(retry+1))
			continue
		}

		if err := handleSocks5Connection(client, currentSSH); err != nil {
			if retry < maxRetries-1 {
				log.Printf("连接处理失败，尝试重试 %d/%d: %v", retry+1, maxRetries, err)
				time.Sleep(time.Second * time.Duration(retry+1))
				continue
			}
			log.Printf("连接处理最终失败: %v", err)
		}
		break
	}
}

func handleSocks5Connection(client net.Conn, sshConn *ssh.Client) error {
	// 读取SOCKS5握手
	buf := make([]byte, 4)
	if _, err := io.ReadFull(client, buf[:2]); err != nil {
		log.Printf("读取SOCKS5握手错误: %v", err)
		return err
	}

	if buf[0] != 0x05 { // SOCKS5版本
		log.Printf("不支持的SOCKS版本: %d", buf[0])
		return fmt.Errorf("不支持的SOCKS版本: %d", buf[0])
	}

	// 读取认证方法数量
	methodCount := int(buf[1])
	methods := make([]byte, methodCount)
	if _, err := io.ReadFull(client, methods); err != nil {
		log.Printf("读取认证方法错误: %v", err)
		return err
	}

	// 回复不需要认证
	client.Write([]byte{0x05, 0x00})

	// 读取请求头
	if _, err := io.ReadFull(client, buf[:4]); err != nil {
		log.Printf("读取请求错误: %v", err)
		return err
	}

	var addr string
	switch buf[3] {
	case 0x01: // IPv4
		ipv4 := make([]byte, 4)
		if _, err := io.ReadFull(client, ipv4); err != nil {
			log.Printf("读取IPv4地址错误: %v", err)
			return err
		}
		addr = net.IP(ipv4).String()
	case 0x03: // 域名
		domainLen := make([]byte, 1)
		if _, err := io.ReadFull(client, domainLen); err != nil {
			log.Printf("读取域名长度错误: %v", err)
			return err
		}
		domain := make([]byte, domainLen[0])
		if _, err := io.ReadFull(client, domain); err != nil {
			log.Printf("读取域名错误: %v", err)
			return err
		}
		addr = string(domain)
	case 0x04: // IPv6
		ipv6 := make([]byte, 16)
		if _, err := io.ReadFull(client, ipv6); err != nil {
			log.Printf("读取IPv6地址错误: %v", err)
			return err
		}
		addr = net.IP(ipv6).String()
	default:
		log.Printf("不支持的地址类型: %d", buf[3])
		return fmt.Errorf("不支持的地址类型: %d", buf[3])
	}

	// 读取端口
	portBuf := make([]byte, 2)
	if _, err := io.ReadFull(client, portBuf); err != nil {
		log.Printf("读取端口错误: %v", err)
		return err
	}
	port := int(portBuf[0])<<8 | int(portBuf[1])

	// 通过SSH隧道建立到目标地址的连接
	target, err := sshConn.Dial("tcp", fmt.Sprintf("%s:%d", addr, port))
	if err != nil {
		log.Printf("连接目标地址错误: %v", err)
		client.Write([]byte{0x05, 0x01, 0x00, 0x01, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00})
		return err
	}
	defer target.Close()

	// 发送成功响应
	client.Write([]byte{0x05, 0x00, 0x00, 0x01, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00})

	// 开始转发数据
	var wg sync.WaitGroup
	wg.Add(2)

	go func() {
		defer wg.Done()
		io.Copy(target, client)
	}()

	go func() {
		defer wg.Done()
		io.Copy(client, target)
	}()

	wg.Wait()

	return nil
} 