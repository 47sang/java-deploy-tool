package compiler

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"runtime"

	"deploy-tool/internal/timeout"
	"deploy-tool/pkg/utils"
)

// BuildVueProject 构建 Vue 项目。
// ctx 叠加 timeout.BuildTimeout 作为单次构建上限，超时后自动终止 npm 子进程，
// 避免 npm 构建卡死（依赖安装、网络问题）永久阻塞部署。
func BuildVueProject(ctx context.Context, projectDir string, scripts string) error {
	fmt.Printf("开始构建Vue项目，脚本: %s\n", scripts)

	// 根据操作系统类型选择适当的命令
	var name string
	var args []string
	if runtime.GOOS == "windows" {
		name = "cmd"
		args = []string{"/c", "npm", "run", scripts}
	} else {
		name = "npm"
		args = []string{"run", scripts}
	}

	stderrOutput, err := runBuildCommand(ctx, name, args, projectDir, nil)
	if err != nil {
		// 构建超时：错误已含超时提示，直接返回
		if errors.Is(err, context.DeadlineExceeded) {
			return err
		}
		return fmt.Errorf("构建失败:请检查npm是否配置在环境变量中\n%s", stderrOutput)
	}

	utils.PrintSuccess("%s环境下的Vue项目构建成功!", scripts)
	return nil
}

// CheckNodeJS 检查 Node.js 是否可用。
// 每条探测命令受 timeout.ProbeTimeout 约束，避免 node/npm 异常挂起阻塞流程。
func CheckNodeJS() error {
	// 检查 node 命令
	output, err := runProbeCommand("node", "--version")
	if err != nil {
		return fmt.Errorf("Node.js 未安装或未在环境变量中配置")
	}
	fmt.Printf("Node.js 版本: %s", output)

	// 检查 npm 命令
	output, err = runProbeCommand("npm", "--version")
	if err != nil {
		return fmt.Errorf("npm 未安装或未在环境变量中配置")
	}
	fmt.Printf("npm 版本: %s", output)

	return nil
}

// runProbeCommand 在 timeout.ProbeTimeout 内执行一条本地探测命令并返回其标准输出。
// 超时或执行失败均返回错误，由调用方决定如何处理。
func runProbeCommand(name string, args ...string) (string, error) {
	var output []byte
	err := timeout.RunWithTimeout(timeout.ProbeTimeout, func() error {
		var e error
		output, e = exec.Command(name, args...).Output()
		return e
	}, nil)
	if err != nil {
		return "", err
	}
	return string(output), nil
}
