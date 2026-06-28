package utils

import (
	"fmt"
	"sync"
	"time"
)

// FormatBytes 格式化字节数为可读格式
func FormatBytes(bytes uint64) string {
	units := []string{"B", "KB", "MB", "GB", "TB"}
	size := float64(bytes)
	unitIndex := 0

	for size >= 1024 && unitIndex < len(units)-1 {
		size /= 1024
		unitIndex++
	}

	if unitIndex == 0 {
		return fmt.Sprintf("%d %s", bytes, units[unitIndex])
	}
	return fmt.Sprintf("%.1f %s", size, units[unitIndex])
}

// BytesToMB 将字节转换为 MB
func BytesToMB(bytes uint64) float64 {
	return float64(bytes) / (1024.0 * 1024.0)
}

// FormatDuration 格式化时间为可读格式
func FormatDuration(d time.Duration) string {
	totalSeconds := int64(d.Seconds())

	if totalSeconds < 60 {
		return fmt.Sprintf("%ds", totalSeconds)
	} else if totalSeconds < 3600 {
		minutes := totalSeconds / 60
		secs := totalSeconds % 60
		return fmt.Sprintf("%dm %ds", minutes, secs)
	} else if totalSeconds < 86400 {
		hours := totalSeconds / 3600
		minutes := (totalSeconds % 3600) / 60
		return fmt.Sprintf("%dh %dm", hours, minutes)
	}
	days := totalSeconds / 86400
	hours := (totalSeconds % 86400) / 3600
	return fmt.Sprintf("%dd %dh", days, hours)
}

// FormatExecutionTime 格式化执行时间为 HH:MM:SS 格式
func FormatExecutionTime(d time.Duration) string {
	totalSeconds := int64(d.Seconds())
	hours := totalSeconds / 3600
	minutes := (totalSeconds % 3600) / 60
	seconds := totalSeconds % 60
	return fmt.Sprintf("%02d:%02d:%02d", hours, minutes, seconds)
}

// outputMu 序列化并发 goroutine 的终端输出，避免多环境/多模块并发部署时日志交错。
// 并发场景下多个 goroutine 同时打印状态行/成功提示，会使输出难以辨认归属；该锁保证
// 每条输出原子完成。单线程场景下加锁无副作用。
var outputMu sync.Mutex

// SafePrintf 线程安全地打印格式化文本（不追加换行），供并发 goroutine 共享终端输出。
func SafePrintf(format string, args ...interface{}) {
	outputMu.Lock()
	defer outputMu.Unlock()
	fmt.Printf(format, args...)
}

// SafePrintln 线程安全地打印一行格式化文本（追加换行）。
func SafePrintln(format string, args ...interface{}) {
	SafePrintf(format+"\n", args...)
}

// PrintSuccess 打印成功消息
func PrintSuccess(format string, args ...interface{}) {
	SafePrintf("✅ "+format+"\n", args...)
}

// PrintWarning 打印警告消息
func PrintWarning(format string, args ...interface{}) {
	SafePrintf("⚠️  "+format+"\n", args...)
}

// PrintError 打印错误消息
func PrintError(format string, args ...interface{}) {
	SafePrintf("❌ "+format+"\n", args...)
}

// PrintInfo 打印信息消息
func PrintInfo(format string, args ...interface{}) {
	SafePrintf("🔍 "+format+"\n", args...)
}

// PrintStep 打印步骤消息
func PrintStep(step int, format string, args ...interface{}) {
	msg := fmt.Sprintf(format, args...)
	SafePrintf("%d.%s\n", step, msg)
}
