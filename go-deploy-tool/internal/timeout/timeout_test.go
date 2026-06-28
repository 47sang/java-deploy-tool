package timeout

import (
	"errors"
	"testing"
	"time"
)

// TestRunWithTimeout_CompletesInTime fn 在超时前正常返回时，应原样透传其结果。
func TestRunWithTimeout_CompletesInTime(t *testing.T) {
	t.Parallel()
	if err := RunWithTimeout(1*time.Second, func() error { return nil }, nil); err != nil {
		t.Fatalf("期望返回 nil，实际 %v", err)
	}
}

// TestRunWithTimeout_PropagatesError fn 返回错误时应原样向上传播，而非当作超时。
func TestRunWithTimeout_PropagatesError(t *testing.T) {
	t.Parallel()
	sentinel := errors.New("boom")
	err := RunWithTimeout(1*time.Second, func() error { return sentinel }, nil)
	if !errors.Is(err, sentinel) {
		t.Fatalf("期望传播 sentinel 错误，实际 %v", err)
	}
}

// TestRunWithTimeout_TimesOut fn 阻塞超过超时时应返回 ErrTimeout、调用 onTimeout，并及时返回。
func TestRunWithTimeout_TimesOut(t *testing.T) {
	t.Parallel()
	onTimeoutCalled := false
	start := time.Now()
	err := RunWithTimeout(
		50*time.Millisecond,
		func() error { time.Sleep(2 * time.Second); return nil },
		func() { onTimeoutCalled = true },
	)
	elapsed := time.Since(start)

	if !errors.Is(err, ErrTimeout) {
		t.Fatalf("期望 ErrTimeout，实际 %v", err)
	}
	if !onTimeoutCalled {
		t.Fatal("超时时应调用 onTimeout 回调")
	}
	// 超时应及时返回（远小于 fn 内部的 2s 等待），留足 CI 抖动余量
	if elapsed > 500*time.Millisecond {
		t.Fatalf("超时应及时返回，实际耗时 %v", elapsed)
	}
}

// TestRunWithTimeout_NilOnTimeout onTimeout 为 nil 时超时不应 panic。
func TestRunWithTimeout_NilOnTimeout(t *testing.T) {
	t.Parallel()
	err := RunWithTimeout(
		50*time.Millisecond,
		func() error { time.Sleep(2 * time.Second); return nil },
		nil,
	)
	if !errors.Is(err, ErrTimeout) {
		t.Fatalf("期望 ErrTimeout，实际 %v", err)
	}
}

// TestRunWithTimeout_DoesNotBlock 超时分支不应阻塞等待 fn 自然结束：
// 连续多次（fn 内部 sleep 远大于超时）调用总耗时应近似"次数×超时"，而非"次数×sleep"。
func TestRunWithTimeout_DoesNotBlock(t *testing.T) {
	t.Parallel()
	const n = 50
	start := time.Now()
	for i := 0; i < n; i++ {
		err := RunWithTimeout(
			20*time.Millisecond,
			func() error { time.Sleep(300 * time.Millisecond); return nil },
			nil,
		)
		if !errors.Is(err, ErrTimeout) {
			t.Fatalf("第 %d 次期望 ErrTimeout，实际 %v", i, err)
		}
	}
	// 50×20ms=1s 上限即可证明未阻塞（若阻塞则为 50×300ms=15s）；留足抖动余量
	if elapsed := time.Since(start); elapsed > 3*time.Second {
		t.Fatalf("超时分支应不阻塞，50 次实际耗时 %v", elapsed)
	}
}
