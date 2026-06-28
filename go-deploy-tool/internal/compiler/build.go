package compiler

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"os/exec"
	"strings"

	"deploy-tool/internal/timeout"
)

// runBuildCommand 在 timeout.BuildTimeout 内执行一条本地构建命令，实时打印 stdout 与 stderr。
//
// 入参：
//   - ctx：上层取消控制（如 Ctrl+C），其上叠加 BuildTimeout 作为单次构建上限。
//   - name、args：可执行文件名与参数。
//   - dir：命令工作目录。
//   - env：进程环境变量；为 nil 时继承父进程环境。
//
// 返回值：
//   - 累积的 stderr 内容（供失败时诊断）。
//   - 执行错误；超时时会 wrap context.DeadlineExceeded，可用 errors.Is 判定。
//
// 超时后 exec.CommandContext 会自动终止子进程，避免 Maven/npm 卡死永久阻塞部署。
// 本函数是 BuildJavaProject / BuildVueProject 的共用实现，消除两者原先重复的
// 管道读取与等待逻辑，并使构建超时行为可被单元测试覆盖。
func runBuildCommand(ctx context.Context, name string, args []string, dir string, env []string) (string, error) {
	buildCtx, cancel := context.WithTimeout(ctx, timeout.BuildTimeout)
	defer cancel()

	cmd := exec.CommandContext(buildCtx, name, args...)
	cmd.Dir = dir
	if env != nil {
		cmd.Env = env
	}

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return "", fmt.Errorf("无法获取标准输出管道: %w", err)
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return "", fmt.Errorf("无法获取标准错误管道: %w", err)
	}

	if err := cmd.Start(); err != nil {
		return "", fmt.Errorf("无法启动构建命令: %w", err)
	}

	// 实时打印 stdout（不累积，构建成功时无需保留）
	go func() {
		scanner := bufio.NewScanner(stdout)
		for scanner.Scan() {
			fmt.Println(scanner.Text())
		}
	}()

	// 累积并实时打印 stderr（失败时用于诊断）
	var stderrOutput strings.Builder
	go func() {
		scanner := bufio.NewScanner(stderr)
		for scanner.Scan() {
			line := scanner.Text()
			stderrOutput.WriteString(line + "\n")
			fmt.Println(line)
		}
	}()

	if err := cmd.Wait(); err != nil {
		// 超时后 ctx 触发子进程被 kill，wrap DeadlineExceeded 供调用方识别
		if errors.Is(buildCtx.Err(), context.DeadlineExceeded) {
			return stderrOutput.String(), fmt.Errorf("构建超时（超过 %v），已终止子进程: %w", timeout.BuildTimeout, context.DeadlineExceeded)
		}
		return stderrOutput.String(), err
	}
	return stderrOutput.String(), nil
}

// overrideEnv 在 base 环境变量列表上以 value 设置 key，覆盖任何已存在的同名变量。
//
// 直接 append(base, "KEY=...") 会在父进程已有该 key 时产生重复条目；Unix 进程加载
// 环境时通常取第一个匹配 key，导致后追加的值不生效（例如覆盖父进程的 JAVA_HOME 时，
// 子进程仍拿到旧 JDK）。因此这里先剔除同 key 条目再追加，确保注入值唯一且生效。
func overrideEnv(base []string, key, value string) []string {
	prefix := key + "="
	out := make([]string, 0, len(base)+1)
	for _, e := range base {
		if strings.HasPrefix(e, prefix) {
			continue
		}
		out = append(out, e)
	}
	return append(out, prefix+value)
}
