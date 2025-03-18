package upload

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"golang.org/x/crypto/ssh"
)

// bytesToMB 将字节转换为MB
func bytesToMB(bytes int64) float64 {
	return float64(bytes) / (1024.0 * 1024.0)
}

// createSSHClient 创建SSH客户端连接
func createSSHClient(server, username, password string) (*ssh.Client, error) {
	// 设置SSH客户端配置
	config := &ssh.ClientConfig{
		User: username,
		Auth: []ssh.AuthMethod{
			ssh.Password(password),
		},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		Timeout:         15 * time.Second,
	}

	// 建立SSH连接
	client, err := ssh.Dial("tcp", server, config)
	if err != nil {
		return nil, fmt.Errorf("ssh通信连接失败: %v", err)
	}

	return client, nil
}

// executeRemoteCommand 在远程服务器执行命令
func executeRemoteCommand(client *ssh.Client, command string) (string, error) {
	fmt.Printf("执行远程命令: %s\n", command)

	// 创建SSH会话
	session, err := client.NewSession()
	if err != nil {
		return "", fmt.Errorf("创建SSH会话失败: %v", err)
	}
	defer session.Close()

	// 获取命令输出
	output, err := session.CombinedOutput(command)
	if err != nil {
		return "", fmt.Errorf("执行远程命令失败: %v\n输出: %s", err, string(output))
	}

	return string(output), nil
}

// uploadFile 上传文件到远程服务器
func uploadFile(client *ssh.Client, localPath, remotePath string) error {
	// 获取文件信息
	fileInfo, err := os.Stat(localPath)
	if err != nil {
		return fmt.Errorf("获取文件信息失败: %v", err)
	}
	fileSize := fileInfo.Size()

	fmt.Printf("开始上传文件: %s (大小: %.2f MB)\n", localPath, bytesToMB(fileSize))

	// 读取本地文件
	localFile, err := os.Open(localPath)
	if err != nil {
		return fmt.Errorf("无法打开本地文件: %v", err)
	}
	defer localFile.Close()

	// 确保远程目录存在
	remoteDir := filepath.Dir(remotePath)
	if remoteDir != "." && remoteDir != "/" {
		mkdirCmd := fmt.Sprintf("mkdir -p %s", remoteDir)
		if _, err := executeRemoteCommand(client, mkdirCmd); err != nil {
			return fmt.Errorf("创建远程目录失败: %v", err)
		}
	}

	// 创建会话
	session, err := client.NewSession()
	if err != nil {
		return fmt.Errorf("创建SSH会话失败: %v", err)
	}
	defer session.Close()

	// 创建管道来写入文件内容
	w, err := session.StdinPipe()
	if err != nil {
		return fmt.Errorf("获取会话标准输入失败: %v", err)
	}
	defer w.Close()

	// 启动scp接收程序
	scpCmd := fmt.Sprintf("scp -t %s", remotePath)
	if err := session.Start(scpCmd); err != nil {
		return fmt.Errorf("启动SCP命令失败: %v", err)
	}

	// 发送文件头信息
	fmt.Fprintf(w, "C0644 %d %s\n", fileSize, filepath.Base(remotePath))

	// 拷贝文件内容
	_, err = io.Copy(w, localFile)
	if err != nil {
		return fmt.Errorf("传输文件内容失败: %v", err)
	}

	// 发送结束标志
	fmt.Fprint(w, "\x00")

	// 等待命令完成
	if err := session.Wait(); err != nil {
		return fmt.Errorf("SCP命令执行失败: %v", err)
	}

	fmt.Printf("文件上传成功: %s -> %s (大小: %.2f MB)\n", localPath, remotePath, bytesToMB(fileSize))
	return nil
}

// killProcess 杀死远程服务器上的进程
func killProcess(client *ssh.Client, jarPath string) error {
	killCmd := fmt.Sprintf("kill $(ps -ef | grep %s | grep -v grep | awk '{print $2}')", jarPath)
	output, err := executeRemoteCommand(client, killCmd)
	if err != nil {
		// 如果没有找到进程，不返回错误
		if strings.Contains(output, "No such process") {
			fmt.Println("没有找到要杀死的进程")
			return nil
		}
		return fmt.Errorf("杀死进程失败: %v", err)
	}

	if output != "" {
		fmt.Printf("杀死进程命令输出: %s\n", output)
	}
	return nil
}

// startJar 启动JAR包并检查进程状态
func startJar(client *ssh.Client, jarPath, javaPath, env string) error {
	// 启动JAR包
	startCmd := fmt.Sprintf("nohup %s -jar %s --spring.profiles.active=%s > /dev/null 2>&1 &",
		javaPath, jarPath, env)
	
	if _, err := executeRemoteCommand(client, startCmd); err != nil {
		return fmt.Errorf("启动JAR包失败: %v", err)
	}

	// 等待一小段时间确保进程已启动
	time.Sleep(2 * time.Second)

	// 检查进程是否成功启动
	checkCmd := fmt.Sprintf("ps -ef | grep %s | grep -v grep | awk '{print $2}'", jarPath)
	output, err := executeRemoteCommand(client, checkCmd)
	if err != nil {
		return fmt.Errorf("检查进程状态失败: %v", err)
	}

	if strings.TrimSpace(output) == "" {
		return fmt.Errorf("程序启动失败: %s", jarPath)
	}

	fmt.Printf("程序已在后台成功启动: %s, 进程ID %s\n", jarPath, strings.TrimSpace(output))
	return nil
}

// UploadAndRunJar 上传并运行JAR包
func UploadAndRunJar(server, username, password, localPath, remotePath, javaPath, env string) error {
	// 创建SSH客户端
	client, err := createSSHClient(server, username, password)
	if err != nil {
		return err
	}
	defer client.Close()

	// 上传文件
	if err := uploadFile(client, localPath, remotePath); err != nil {
		return err
	}

	// 杀死已存在的进程
	if err := killProcess(client, remotePath); err != nil {
		fmt.Printf("警告: 杀死旧进程失败: %v\n", err)
		// 继续执行，不返回错误
	}

	// 启动JAR包
	if err := startJar(client, remotePath, javaPath, env); err != nil {
		return err
	}

	fmt.Printf("%s环境JAR包部署和启动成功: %s\n", env, remotePath)
	return nil
}

// UploadFile 上传文件到远程服务器并解压
func UploadFile(server, username, password, localPath, remotePath string) error {
	// 创建SSH客户端
	client, err := createSSHClient(server, username, password)
	if err != nil {
		return err
	}
	defer client.Close()

	// 构建远程zip路径
	remoteZipPath := fmt.Sprintf("%s.zip", remotePath)

	// 上传文件
	if err := uploadFile(client, localPath, remoteZipPath); err != nil {
		return err
	}

	// 解压命令：先删除目标目录，然后解压zip文件
	unzipCmd := fmt.Sprintf("rm -rf %s && mkdir -p %s && cd %s && /usr/bin/unzip -o %s",
		remotePath, remotePath, remotePath, remoteZipPath)

	// 执行解压命令
	if _, err := executeRemoteCommand(client, unzipCmd); err != nil {
		return fmt.Errorf("解压文件失败: %v", err)
	}

	fmt.Printf("文件上传并解压成功: %s -> %s\n", localPath, remotePath)
	return nil
} 