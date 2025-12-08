package compiler

import (
	"bufio"
	"fmt"
	"os/exec"
	"runtime"
	"strings"

	"deploy-tool/pkg/utils"
)

// BuildVueProject 构建 Vue 项目
func BuildVueProject(projectDir string, scripts string) error {
	fmt.Printf("开始构建Vue项目，脚本: %s\n", scripts)

	var cmd *exec.Cmd

	// 根据操作系统类型选择适当的命令
	if runtime.GOOS == "windows" {
		cmd = exec.Command("cmd", "/c", "npm", "run", scripts)
	} else {
		cmd = exec.Command("npm", "run", scripts)
	}

	cmd.Dir = projectDir

	// 获取标准输出管道
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("无法获取标准输出管道: %v", err)
	}

	stderr, err := cmd.StderrPipe()
	if err != nil {
		return fmt.Errorf("无法获取标准错误管道: %v", err)
	}

	// 启动命令
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("执行npm命令失败: %v", err)
	}

	// 读取并显示标准输出
	go func() {
		scanner := bufio.NewScanner(stdout)
		for scanner.Scan() {
			fmt.Println(scanner.Text())
		}
	}()

	// 读取标准错误
	var stderrOutput strings.Builder
	go func() {
		scanner := bufio.NewScanner(stderr)
		for scanner.Scan() {
			line := scanner.Text()
			stderrOutput.WriteString(line + "\n")
			fmt.Println(line)
		}
	}()

	// 等待命令完成
	if err := cmd.Wait(); err != nil {
		errorMsg := stderrOutput.String()
		return fmt.Errorf("构建失败:请检查npm是否配置在环境变量中\n%s", errorMsg)
	}

	utils.PrintSuccess("%s环境下的Vue项目构建成功!", scripts)
	return nil
}

// CheckNodeJS 检查 Node.js 是否可用
func CheckNodeJS() error {
	// 检查 node 命令
	cmd := exec.Command("node", "--version")
	output, err := cmd.Output()
	if err != nil {
		return fmt.Errorf("Node.js 未安装或未在环境变量中配置")
	}
	fmt.Printf("Node.js 版本: %s", string(output))

	// 检查 npm 命令
	cmd = exec.Command("npm", "--version")
	output, err = cmd.Output()
	if err != nil {
		return fmt.Errorf("npm 未安装或未在环境变量中配置")
	}
	fmt.Printf("npm 版本: %s", string(output))

	return nil
}
