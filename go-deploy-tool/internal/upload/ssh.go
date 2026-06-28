package upload

import (
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"strings"
	"time"

	"deploy-tool/internal/timeout"
	"deploy-tool/pkg/utils"

	"github.com/pkg/sftp"
	"github.com/schollz/progressbar/v3"
	"golang.org/x/crypto/ssh"
)

const (
	// MaxRetries 最大重试次数
	MaxRetries = 3
	// RetryDelay 重试间隔
	RetryDelay = 2 * time.Second
)

// SSHClient SSH 客户端
type SSHClient struct {
	client  *ssh.Client
	session *ssh.Session
}

// NewSSHClient 创建 SSH 客户端
func NewSSHClient(server, username, password string) (*SSHClient, error) {
	config := &ssh.ClientConfig{
		User: username,
		Auth: []ssh.AuthMethod{
			ssh.Password(password),
		},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		Timeout:         30 * time.Second,
	}

	// 确保服务器地址包含端口
	if !strings.Contains(server, ":") {
		server = server + ":22"
	}

	client, err := ssh.Dial("tcp", server, config)
	if err != nil {
		return nil, fmt.Errorf("SSH连接失败: %v", err)
	}

	return &SSHClient{client: client}, nil
}

// Close 关闭 SSH 连接
func (c *SSHClient) Close() {
	if c.session != nil {
		c.session.Close()
	}
	if c.client != nil {
		c.client.Close()
	}
}

// ExecuteCommand 执行远程命令，使用默认的 timeout.CommandTimeout 作为单条命令超时。
func (c *SSHClient) ExecuteCommand(command string) (string, error) {
	return c.executeCommandWithTimeout(command, timeout.CommandTimeout)
}

// executeCommandWithTimeout 执行远程命令，单条命令受 timeoutDur 约束。
// 超时则关闭 session 中断阻塞的 CombinedOutput，避免 kill/unzip/nohup 等远程命令
// 挂起导致整个部署永久阻塞。timeoutDur 参数便于单元测试注入短超时。
func (c *SSHClient) executeCommandWithTimeout(command string, timeoutDur time.Duration) (string, error) {
	session, err := c.client.NewSession()
	if err != nil {
		return "", fmt.Errorf("创建SSH会话失败: %v", err)
	}
	defer session.Close()

	fmt.Printf("执行远程命令: %s\n", command)

	var output []byte
	execErr := timeout.RunWithTimeout(timeoutDur, func() error {
		var e error
		output, e = session.CombinedOutput(command)
		return e
	}, func() {
		// 超时则关闭 session，使阻塞中的 CombinedOutput 立即返回
		session.Close()
	})
	if execErr != nil {
		// 优先识别超时
		if errors.Is(execErr, timeout.ErrTimeout) {
			return string(output), fmt.Errorf("远程命令执行超时 %s（上限 %v）: %w", command, timeoutDur, timeout.ErrTimeout)
		}
		// 检查是否是退出码错误
		if exitErr, ok := execErr.(*ssh.ExitError); ok {
			if exitErr.ExitStatus() != 0 {
				return string(output), fmt.Errorf("远程命令执行失败，退出状态: %d", exitErr.ExitStatus())
			}
		}
		return string(output), fmt.Errorf("执行远程命令失败: %v", execErr)
	}

	return string(output), nil
}

// UploadFile 上传文件到远程服务器
func (c *SSHClient) UploadFile(localPath, remotePath string, showProgress bool) error {
	// 读取本地文件
	localFile, err := os.Open(localPath)
	if err != nil {
		return fmt.Errorf("打开本地文件失败: %v", err)
	}
	defer localFile.Close()

	fileInfo, err := localFile.Stat()
	if err != nil {
		return fmt.Errorf("获取文件信息失败: %v", err)
	}

	fileSize := fileInfo.Size()
	fmt.Printf("读取本地文件: %s\n", localPath)
	fmt.Printf("已读取的文件大小: %.2f MB\n", utils.BytesToMB(uint64(fileSize)))

	// 创建 SFTP 客户端
	sftpClient, err := sftp.NewClient(c.client)
	if err != nil {
		return fmt.Errorf("创建SFTP客户端失败: %v", err)
	}
	defer sftpClient.Close()

	// 临时文件路径
	tempRemotePath := remotePath + ".tmp"

	// 确保远程目录存在
	remoteDir := filepath.Dir(remotePath)
	if err := sftpClient.MkdirAll(remoteDir); err != nil {
		// 忽略目录已存在的错误
	}

	// 创建远程临时文件
	remoteFile, err := sftpClient.Create(tempRemotePath)
	if err != nil {
		return fmt.Errorf("创建远程文件失败: %v", err)
	}
	defer remoteFile.Close()

	// 使用进度条
	var reader io.Reader = localFile
	if showProgress {
		bar := progressbar.NewOptions64(
			fileSize,
			progressbar.OptionEnableColorCodes(true),
			progressbar.OptionShowBytes(true),
			progressbar.OptionSetWidth(40),
			progressbar.OptionSetDescription("上传中"),
			progressbar.OptionSetTheme(progressbar.Theme{
				Saucer:        "[green]=[reset]",
				SaucerHead:    "[green]>[reset]",
				SaucerPadding: " ",
				BarStart:      "[",
				BarEnd:        "]",
			}),
		)
		reader = io.TeeReader(localFile, bar)
	}

	// 写入文件（受上传超时约束；超时则关闭远程文件句柄以中断阻塞的 io.Copy）
	copyErr := timeout.RunWithTimeout(timeout.UploadTimeout, func() error {
		var e error
		_, e = io.Copy(remoteFile, reader)
		return e
	}, func() { remoteFile.Close() })
	if copyErr != nil {
		if errors.Is(copyErr, timeout.ErrTimeout) {
			return fmt.Errorf("文件上传超时 %s（上限 %v）: %w", localPath, timeout.UploadTimeout, timeout.ErrTimeout)
		}
		return fmt.Errorf("写入远程文件失败: %v", copyErr)
	}

	fmt.Printf("\n文件上传完成: %s\n", tempRemotePath)

	// 检查远程文件是否存在
	_, err = sftpClient.Stat(remotePath)
	if err == nil {
		// 文件存在，创建备份
		fmt.Printf("远程文件已存在: %s\n", remotePath)
		backupPath := remotePath + ".bak"

		// 删除旧备份并创建新备份
		sftpClient.Remove(backupPath)
		if err := sftpClient.Rename(remotePath, backupPath); err != nil {
			return fmt.Errorf("备份远程文件失败: %v", err)
		}
		fmt.Printf("已创建备份: %s\n", backupPath)
	} else {
		fmt.Printf("远程文件不存在，将创建新文件: %s\n", remotePath)
	}

	// 将临时文件移动到最终位置
	if err := sftpClient.Rename(tempRemotePath, remotePath); err != nil {
		return fmt.Errorf("移动文件到最终位置失败: %v", err)
	}

	fmt.Printf("文件已移动到最终位置: %s\n", remotePath)

	return nil
}

// KillProcess 杀死远程服务器上的进程
func (c *SSHClient) KillProcess(jarPath, env string) error {
	// 查找进程 PID。"|| true" 兜底确保命令退出码恒为 0（grep 无匹配时退出码 1），
	// 使下方的 err 只代表真实执行异常（超时/SSH 会话失败等），可安全向上传播。
	// 原实现直接 return nil 会把这类异常误判为"无进程可杀"，跳过停服步骤，可能在
	// 启动新进程时造成双实例运行。
	findPidCmd := fmt.Sprintf("ps -ef | grep %s | grep -v grep | awk '{print $2}' || true", jarPath)
	pids, err := c.ExecuteCommand(findPidCmd)
	if err != nil {
		return fmt.Errorf("查找进程失败 %s: %w", jarPath, err)
	}

	pids = strings.TrimSpace(pids)
	if pids == "" {
		fmt.Printf("没有找到需要杀死的进程: %s\n", jarPath)
		return nil
	}

	// 根据部署环境，执行优雅关闭或者强制 kill 命令
	var killCmd string
	if env == "prod" {
		killCmd = fmt.Sprintf("kill %s", pids)
	} else {
		killCmd = fmt.Sprintf("kill -9 %s", pids)
	}

	output, _ := c.ExecuteCommand(killCmd)
	if output != "" {
		fmt.Printf("杀死进程命令输出: %s\n", output)
	}

	// 等待进程结束
	time.Sleep(1 * time.Second)

	// 检查进程是否还存在
	pidList := strings.ReplaceAll(pids, "\n", ",")
	checkCmd := fmt.Sprintf("ps -p %s > /dev/null 2>&1; echo $?", pidList)

	for attempt := 0; attempt < MaxRetries; attempt++ {
		if attempt > 0 {
			fmt.Printf("检查进程状态 (第%d次重试)...\n", attempt)
			time.Sleep(time.Duration(10*(attempt+1)) * time.Second)
		}

		exitCode, err := c.ExecuteCommand(checkCmd)
		if err == nil && strings.TrimSpace(exitCode) == "1" {
			fmt.Printf("进程已成功杀死: %s\n", pids)
			return nil
		}
	}

	// 尝试强制杀死
	fmt.Printf("进程杀死失败，执行强制杀死进程命令: %s\n", pids)
	forceKillCmd := fmt.Sprintf("kill -9 %s", pids)
	output, _ = c.ExecuteCommand(forceKillCmd)
	if output != "" {
		fmt.Printf("强制杀死命令输出: %s\n", output)
	}

	// 最后检查一次
	exitCode, _ := c.ExecuteCommand(checkCmd)
	if strings.TrimSpace(exitCode) == "1" {
		fmt.Println("强制杀死成功")
		return nil
	}

	return fmt.Errorf("最终进程检查失败，进程可能仍在运行: %s", pids)
}

// StartJar 启动 JAR 包
func (c *SSHClient) StartJar(jarPath, javaPath, env string) error {
	startCmd := fmt.Sprintf("nohup %s -jar %s --spring.profiles.active=%s > /dev/null 2>&1 &",
		javaPath, jarPath, env)

	_, err := c.ExecuteCommand(startCmd)
	if err != nil {
		return err
	}

	// 等待进程启动
	time.Sleep(2 * time.Second)

	// 检查进程是否成功启动
	checkCmd := fmt.Sprintf("ps -ef | grep %s | grep -v grep | awk '{print $2}'", jarPath)
	output, err := c.ExecuteCommand(checkCmd)
	if err != nil {
		return fmt.Errorf("检查进程状态失败: %v", err)
	}

	output = strings.TrimSpace(output)
	if output == "" {
		return fmt.Errorf("程序启动失败: %s", jarPath)
	}

	fmt.Printf("程序已在后台成功启动: %s,进程id %s\n", jarPath, output)
	return nil
}

// UnzipRemote 在远程服务器解压文件
func (c *SSHClient) UnzipRemote(zipPath, extractDir string) error {
	// 修复后的解压命令
	unzipCmd := fmt.Sprintf("rm -rf %s && mkdir -p %s && /usr/bin/unzip -o %s -d %s && echo '解压完成'",
		extractDir, extractDir, zipPath, extractDir)

	output, err := c.ExecuteCommand(unzipCmd)
	if err != nil {
		return fmt.Errorf("解压失败: %v, 输出: %s", err, output)
	}

	return nil
}

// UploadAndRunJar 上传并运行 JAR 包
func UploadAndRunJar(server, username, password, localPath, remotePath, javaPath, env string) error {
	// 读取本地文件
	fileInfo, err := os.Stat(localPath)
	if err != nil {
		return fmt.Errorf("无法获取本地文件信息: %v", err)
	}

	fmt.Printf("读取本地文件: %s\n", localPath)
	fmt.Printf("已读取的文件大小: %.2f MB\n", utils.BytesToMB(uint64(fileInfo.Size())))

	// 创建 SSH 客户端
	client, err := NewSSHClient(server, username, password)
	if err != nil {
		return err
	}
	defer client.Close()

	// 上传文件（带重试）
	var uploadErr error
	for attempt := 0; attempt < MaxRetries; attempt++ {
		if attempt > 0 {
			fmt.Printf("尝试重新上传文件 (第%d次重试)...\n", attempt)
			time.Sleep(RetryDelay)
		}

		uploadErr = client.UploadFile(localPath, remotePath, true)
		if uploadErr == nil {
			break
		}
		fmt.Printf("文件上传失败: %v，正在重试...\n", uploadErr)
	}

	if uploadErr != nil {
		return fmt.Errorf("文件上传失败，已达到最大重试次数(%d次): %v", MaxRetries, uploadErr)
	}

	utils.PrintSuccess("JAR 文件上传成功! %s -> %s (大小: %.2f MB)",
		localPath, remotePath, utils.BytesToMB(uint64(fileInfo.Size())))

	// 在文件上传成功后杀死旧进程
	if err := client.KillProcess(remotePath, env); err != nil {
		return fmt.Errorf("杀死旧进程失败: %v", err)
	}

	// 启动 JAR 包
	if err := client.StartJar(remotePath, javaPath, env); err != nil {
		return fmt.Errorf("启动JAR包失败: %v", err)
	}

	return nil
}

// UploadJarOnly 仅上传 JAR 文件
func UploadJarOnly(server, username, password, localPath, remotePath string) error {
	// 读取本地文件
	fileInfo, err := os.Stat(localPath)
	if err != nil {
		return fmt.Errorf("无法获取本地文件信息: %v", err)
	}

	fmt.Printf("读取本地文件: %s\n", localPath)
	fmt.Printf("已读取的文件大小: %.2f MB\n", utils.BytesToMB(uint64(fileInfo.Size())))

	// 创建 SSH 客户端
	client, err := NewSSHClient(server, username, password)
	if err != nil {
		return err
	}
	defer client.Close()

	// 上传文件（带重试）
	var uploadErr error
	for attempt := 0; attempt < MaxRetries; attempt++ {
		if attempt > 0 {
			fmt.Printf("尝试重新上传文件 (第%d次重试)...\n", attempt)
			time.Sleep(RetryDelay)
		}

		uploadErr = client.UploadFile(localPath, remotePath, true)
		if uploadErr == nil {
			break
		}
		fmt.Printf("文件上传失败: %v，正在重试...\n", uploadErr)
	}

	if uploadErr != nil {
		return fmt.Errorf("文件上传失败，已达到最大重试次数(%d次): %v", MaxRetries, uploadErr)
	}

	utils.PrintSuccess("JAR 文件上传成功! %s -> %s (大小: %.2f MB)",
		localPath, remotePath, utils.BytesToMB(uint64(fileInfo.Size())))

	return nil
}

// UploadZipOnly 仅上传 ZIP 文件
func UploadZipOnly(server, username, password, localPath, remotePath string) error {
	// 读取本地文件
	fileInfo, err := os.Stat(localPath)
	if err != nil {
		return fmt.Errorf("无法获取本地文件信息: %v", err)
	}

	// 创建 SSH 客户端
	client, err := NewSSHClient(server, username, password)
	if err != nil {
		return err
	}
	defer client.Close()

	// 调整远程路径：去除末尾的 .zip 后缀（仅上传场景，远程文件名不应携带 .zip）。
	// 注意：原实现紧跟一个恒为 true 的 if 回退判断——TrimSuffix 后必然不含 .zip，
	// 于是 remoteZipPath 被还原为原 remotePath，后缀实际从未被去除，与 Rust 版
	// 的 .replace(".zip", "") 行为不一致。
	remoteZipPath := strings.TrimSuffix(remotePath, ".zip")

	// 上传文件
	if err := client.UploadFile(localPath, remoteZipPath, true); err != nil {
		return err
	}

	utils.PrintSuccess("ZIP 文件上传成功! %s -> %s (大小: %.2f MB)",
		localPath, remoteZipPath, utils.BytesToMB(uint64(fileInfo.Size())))

	return nil
}

// UploadAndExtractZip 上传并解压 ZIP 文件
func UploadAndExtractZip(server, username, password, localPath, remotePath string) error {
	// 读取本地文件
	fileInfo, err := os.Stat(localPath)
	if err != nil {
		return fmt.Errorf("无法获取本地文件信息: %v", err)
	}

	// 创建 SSH 客户端
	client, err := NewSSHClient(server, username, password)
	if err != nil {
		return err
	}
	defer client.Close()

	// 确保远程 ZIP 路径包含 .zip 后缀
	remoteZipPath := remotePath
	if !strings.HasSuffix(remoteZipPath, ".zip") {
		remoteZipPath = remotePath + ".zip"
	}

	// 获取解压目标目录
	extractDir := strings.TrimSuffix(remotePath, ".zip")
	if extractDir == remotePath {
		extractDir = remotePath
	}

	// 上传 ZIP 文件
	if err := client.UploadFile(localPath, remoteZipPath, true); err != nil {
		return err
	}

	fmt.Println("正在解压文件...")

	// 解压文件
	if err := client.UnzipRemote(remoteZipPath, extractDir); err != nil {
		return err
	}

	utils.PrintSuccess("文件上传并解压成功! %s -> %s (大小: %.2f MB)",
		localPath, extractDir, utils.BytesToMB(uint64(fileInfo.Size())))

	return nil
}

// TestConnection 测试 SSH 连接
func TestConnection(server, username, password string) error {
	// 确保服务器地址包含端口
	if !strings.Contains(server, ":") {
		server = server + ":22"
	}

	config := &ssh.ClientConfig{
		User: username,
		Auth: []ssh.AuthMethod{
			ssh.Password(password),
		},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		Timeout:         10 * time.Second,
	}

	conn, err := net.DialTimeout("tcp", server, 10*time.Second)
	if err != nil {
		return fmt.Errorf("无法连接到服务器 %s: %v", server, err)
	}
	conn.Close()

	client, err := ssh.Dial("tcp", server, config)
	if err != nil {
		return fmt.Errorf("SSH认证失败: %v", err)
	}
	client.Close()

	return nil
}
