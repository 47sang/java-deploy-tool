package compiler

import (
	"context"
	"errors"
	"runtime"
	"strings"
	"testing"
	"time"
)

// TestRunBuildCommand_Success 命令正常退出时返回 nil。
func TestRunBuildCommand_Success(t *testing.T) {
	t.Parallel()
	var name string
	var args []string
	if runtime.GOOS == "windows" {
		name, args = "cmd", []string{"/c", "echo", "hello"}
	} else {
		name, args = "echo", []string{"hello"}
	}
	if _, err := runBuildCommand(context.Background(), name, args, "", nil); err != nil {
		t.Fatalf("正常命令应返回 nil，实际 %v", err)
	}
}

// TestRunBuildCommand_Failure 命令非零退出时返回错误，且不被误判为超时。
func TestRunBuildCommand_Failure(t *testing.T) {
	t.Parallel()
	var name string
	var args []string
	if runtime.GOOS == "windows" {
		name, args = "cmd", []string{"/c", "exit", "1"}
	} else {
		name, args = "sh", []string{"-c", "exit 1"}
	}
	_, err := runBuildCommand(context.Background(), name, args, "", nil)
	if err == nil {
		t.Fatal("非零退出应返回错误")
	}
	if errors.Is(err, context.DeadlineExceeded) {
		t.Fatal("非零退出不应被识别为超时")
	}
}

// TestRunBuildCommand_TimesOut 子进程长时间阻塞时，上层 ctx 的短 deadline 会触发
// exec.CommandContext 终止子进程，runBuildCommand 返回 wrap 了 DeadlineExceeded 的错误。
// 这是对"本地构建超时自动杀进程"机制的真实端到端验证（用 sleep 模拟卡死的构建）。
func TestRunBuildCommand_TimesOut(t *testing.T) {
	t.Parallel()
	// 上层 ctx 设远短于 BuildTimeout 的 deadline；runBuildCommand 取 ctx 与 BuildTimeout 的更早者
	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()

	var name string
	var args []string
	if runtime.GOOS == "windows" {
		name, args = "cmd", []string{"/c", "ping", "-n", "30", "127.0.0.1"}
	} else {
		name, args = "sleep", []string{"30"}
	}

	start := time.Now()
	_, err := runBuildCommand(ctx, name, args, "", nil)
	elapsed := time.Since(start)

	if err == nil {
		t.Fatal("阻塞命令应返回错误")
	}
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("期望 wrap DeadlineExceeded 的超时错误，实际 %v", err)
	}
	if !strings.Contains(err.Error(), "构建超时") {
		t.Fatalf("超时错误应含中文提示，实际 %v", err)
	}
	// 500ms deadline 触发后应及时返回，而非等到 30s sleep 自然结束
	if elapsed > 5*time.Second {
		t.Fatalf("超时应及时终止子进程，实际耗时 %v", elapsed)
	}
}

// TestOverrideEnv 验证覆盖同名环境变量：新值生效、旧值被剔除、且不产生重复 key。
// 直接 append 会在父进程已有同名变量时残留旧值（Unix 取第一个 key），本测试锁定该修复。
func TestOverrideEnv(t *testing.T) {
	t.Parallel()
	base := []string{"PATH=/usr/bin", "JAVA_HOME=/old/jdk", "HOME=/root"}

	got := overrideEnv(base, "JAVA_HOME", "/new/jdk")
	joined := strings.Join(got, "\n")

	if !strings.Contains(joined, "JAVA_HOME=/new/jdk") {
		t.Errorf("应包含覆盖后的新值 JAVA_HOME=/new/jdk，实际 %v", got)
	}
	if strings.Contains(joined, "JAVA_HOME=/old/jdk") {
		t.Errorf("不应保留旧值 JAVA_HOME=/old/jdk，实际 %v", got)
	}
	if !strings.Contains(joined, "HOME=/root") || !strings.Contains(joined, "PATH=/usr/bin") {
		t.Errorf("不应丢失无关变量，实际 %v", got)
	}
	// JAVA_HOME 应唯一存在，不得出现重复 key
	count := 0
	for _, e := range got {
		if strings.HasPrefix(e, "JAVA_HOME=") {
			count++
		}
	}
	if count != 1 {
		t.Errorf("JAVA_HOME 应唯一，实际出现 %d 次", count)
	}

	// base 中不存在该 key 时，正常追加到末尾
	got = overrideEnv([]string{"A=1"}, "B", "2")
	if len(got) != 2 || got[1] != "B=2" {
		t.Errorf("新增 key 应追加到末尾，实际 %v", got)
	}
}
