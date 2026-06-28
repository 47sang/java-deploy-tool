package upload

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/binary"
	"errors"
	"net"
	"strings"
	"testing"
	"time"

	"deploy-tool/internal/timeout"

	"golang.org/x/crypto/ssh"
)

// startTestSSHServer 启动一个本地 SSH 服务器（密码认证 test/test，客户端跳过 host key 校验）。
// 对 exec 请求：sleep 开头的命令会阻塞不返回（模拟卡死的远程命令，用于测超时）；
// 其他命令原样回写为 stdout 并以退出码 0 结束。返回监听地址。
func startTestSSHServer(t *testing.T) string {
	t.Helper()
	_, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("生成 host key 失败: %v", err)
	}
	signer, err := ssh.NewSignerFromSigner(priv)
	if err != nil {
		t.Fatalf("构造 signer 失败: %v", err)
	}

	cfg := &ssh.ServerConfig{
		PasswordCallback: func(c ssh.ConnMetadata, pass []byte) (*ssh.Permissions, error) {
			if c.User() == "test" && string(pass) == "test" {
				return nil, nil
			}
			return nil, errors.New("invalid credentials")
		},
	}
	cfg.AddHostKey(signer)

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("监听失败: %v", err)
	}
	t.Cleanup(func() { listener.Close() })

	go func() {
		for {
			nConn, err := listener.Accept()
			if err != nil {
				return // listener 已关闭
			}
			go serveSSHConn(nConn, cfg)
		}
	}()
	return listener.Addr().String()
}

// serveSSHConn 处理一条 SSH 连接上的所有 session 通道。
func serveSSHConn(nConn net.Conn, srvCfg *ssh.ServerConfig) {
	conn, chans, reqs, err := ssh.NewServerConn(nConn, srvCfg)
	if err != nil {
		return
	}
	defer conn.Close()
	go ssh.DiscardRequests(reqs)

	for newChan := range chans {
		if newChan.ChannelType() != "session" {
			_ = newChan.Reject(ssh.UnknownChannelType, "仅支持 session 通道")
			continue
		}
		go serveSession(newChan)
	}
}

// serveSession 处理一个 session 通道上的 exec 请求。
func serveSession(newChan ssh.NewChannel) {
	ch, reqs, err := newChan.Accept()
	if err != nil {
		return
	}
	defer ch.Close()

	for req := range reqs {
		if req.Type != "exec" {
			_ = req.Reply(false, nil)
			continue
		}
		// exec 请求 payload：uint32 长度 + 命令字符串
		if len(req.Payload) < 4 {
			_ = req.Reply(false, nil)
			continue
		}
		cmdLen := binary.BigEndian.Uint32(req.Payload[:4])
		if uint32(len(req.Payload)) < 4+cmdLen {
			_ = req.Reply(false, nil)
			continue
		}
		cmd := string(req.Payload[4 : 4+cmdLen])
		_ = req.Reply(true, nil)

		if strings.HasPrefix(strings.TrimSpace(cmd), "sleep") {
			// 模拟卡死的远程命令：阻塞足够长时间（超过客户端超时）且不发 exit-status，
			// 迫使客户端走超时路径关闭 session。不依赖读 channel（exec 后客户端会对
			// channel 发 EOF，读 channel 会立即返回而无法模拟阻塞）。
			time.Sleep(time.Second)
			return
		}
		// 普通命令：回写命令本身，退出码 0
		_, _ = ch.Write([]byte(cmd))
		exitBuf := make([]byte, 4)
		binary.BigEndian.PutUint32(exitBuf, 0)
		_, _ = ch.SendRequest("exit-status", false, exitBuf)
		return
	}
}

// TestExecuteCommand_Success 正常远程命令应返回其输出。
func TestExecuteCommand_Success(t *testing.T) {
	addr := startTestSSHServer(t)
	client, err := NewSSHClient(addr, "test", "test")
	if err != nil {
		t.Fatalf("连接测试服务器失败: %v", err)
	}
	defer client.Close()

	out, err := client.ExecuteCommand("hello-remote")
	if err != nil {
		t.Fatalf("ExecuteCommand 失败: %v", err)
	}
	if out != "hello-remote" {
		t.Errorf("输出 = %q, want %q", out, "hello-remote")
	}
}

// TestExecuteCommand_AuthFailure 错误密码应返回连接错误。
func TestExecuteCommand_AuthFailure(t *testing.T) {
	addr := startTestSSHServer(t)
	if _, err := NewSSHClient(addr, "test", "wrong-password"); err == nil {
		t.Fatal("错误密码应返回连接错误")
	}
}

// TestExecuteCommand_TimesOut 远程命令阻塞时应被超时机制中断，返回 timeout.ErrTimeout。
// 通过 executeCommandWithTimeout 注入短超时（默认 CommandTimeout 为 5min，不适合测试），
// 端到端验证"关闭 session 中断阻塞的 CombinedOutput"这一 SSH 命令超时机制真实生效。
func TestExecuteCommand_TimesOut(t *testing.T) {
	addr := startTestSSHServer(t)
	client, err := NewSSHClient(addr, "test", "test")
	if err != nil {
		t.Fatalf("连接测试服务器失败: %v", err)
	}
	defer client.Close()

	start := time.Now()
	_, err = client.executeCommandWithTimeout("sleep 60", 500*time.Millisecond)
	elapsed := time.Since(start)

	if err == nil {
		t.Fatal("阻塞命令应返回错误")
	}
	if !errors.Is(err, timeout.ErrTimeout) {
		t.Fatalf("期望 timeout.ErrTimeout，实际 %v", err)
	}
	// 500ms 超时触发后应及时返回，而非等到 sleep 自然结束或连接超时
	if elapsed > 5*time.Second {
		t.Fatalf("超时应及时返回，实际耗时 %v", elapsed)
	}
}
