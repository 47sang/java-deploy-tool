// Package timeout 提供部署工具各关键环节的超时时长与通用超时执行器。
//
// 设计动机：原先本地构建（Maven/npm）、远程命令执行（SSH）、SFTP 上传等关键环节
// 均无超时上限，一旦子进程或远程命令挂起（下载依赖卡死、网络抖动等），整个部署会
// 永久阻塞。本包集中给出合理默认值，并提供 RunWithTimeout 作为可中断的执行包装。
package timeout

import (
	"errors"
	"time"
)

// 默认超时时长。取值偏宽松，覆盖绝大多数部署场景，避免误杀正常的长任务。
const (
	// BuildTimeout 本地构建（Maven clean package / npm run）单次执行的超时上限。
	BuildTimeout = 30 * time.Minute
	// CommandTimeout 单条远程命令（kill、unzip、nohup 启动等）的超时上限。
	CommandTimeout = 5 * time.Minute
	// UploadTimeout 单个文件 SFTP 上传的超时上限。
	UploadTimeout = 30 * time.Minute
	// ConnectTimeout SSH 建连超时（与 NewSSHClient 现有行为一致）。
	ConnectTimeout = 30 * time.Second
	// ProbeTimeout 本地探测命令（java -version、mvn --version、node --version）超时上限。
	ProbeTimeout = 15 * time.Second
)

// ErrTimeout 表示操作因超过限定时长被中止。可用 errors.Is 判定。
var ErrTimeout = errors.New("操作超时")

// RunWithTimeout 在 timeout 时长内执行 fn。
//
// 行为：
//   - 若 fn 在超时前返回，返回其结果。
//   - 若超时，则调用 onTimeout（用于中断底层阻塞资源，例如关闭 SSH session 使
//     阻塞中的 CombinedOutput 立即返回），随后立即返回 ErrTimeout，不等待 fn。
//   - onTimeout 可为 nil。
//
// 实现要点：done 为缓冲通道（容量 1），即使超时分支无人接收，fn 的 goroutine
// 也能完成发送并退出，从而不会泄漏；同时超时分支无需阻塞等待 fn，保证及时返回。
//
// 契约：调用方应确保 onTimeout 能有效中断 fn 持有的资源（如关闭 session/连接）；
// 否则 fn 的 goroutine 将延续到其自然结束。该函数本身不依赖任何外部资源，便于单测。
func RunWithTimeout(timeout time.Duration, fn func() error, onTimeout func()) error {
	done := make(chan error, 1)
	go func() {
		done <- fn()
	}()

	select {
	case err := <-done:
		return err
	case <-time.After(timeout):
		if onTimeout != nil {
			onTimeout()
		}
		return ErrTimeout
	}
}
