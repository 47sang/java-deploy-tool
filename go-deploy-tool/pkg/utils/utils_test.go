package utils

import (
	"io"
	"os"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestFormatBytes(t *testing.T) {
	t.Parallel()
	tests := []struct {
		in   uint64
		want string
	}{
		{0, "0 B"},
		{512, "512 B"},
		{1024, "1.0 KB"},
		{1536, "1.5 KB"},
		{1048576, "1.0 MB"},
		{1073741824, "1.0 GB"},
	}
	for _, tt := range tests {
		if got := FormatBytes(tt.in); got != tt.want {
			t.Errorf("FormatBytes(%d) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

func TestBytesToMB(t *testing.T) {
	t.Parallel()
	if got := BytesToMB(1048576); got != 1.0 {
		t.Errorf("BytesToMB(1MiB) = %v, want 1.0", got)
	}
	if got := BytesToMB(2 * 1048576); got != 2.0 {
		t.Errorf("BytesToMB(2MiB) = %v, want 2.0", got)
	}
}

func TestFormatDuration(t *testing.T) {
	t.Parallel()
	tests := []struct {
		d    time.Duration
		want string
	}{
		{0, "0s"},
		{45 * time.Second, "45s"},
		{90 * time.Second, "1m 30s"},
		{3700 * time.Second, "1h 1m"},
		{90000 * time.Second, "1d 1h"},
	}
	for _, tt := range tests {
		if got := FormatDuration(tt.d); got != tt.want {
			t.Errorf("FormatDuration(%v) = %q, want %q", tt.d, got, tt.want)
		}
	}
}

func TestFormatExecutionTime(t *testing.T) {
	t.Parallel()
	tests := []struct {
		d    time.Duration
		want string
	}{
		{0, "00:00:00"},
		{90 * time.Second, "00:01:30"},
		{3700 * time.Second, "01:01:40"},
	}
	for _, tt := range tests {
		if got := FormatExecutionTime(tt.d); got != tt.want {
			t.Errorf("FormatExecutionTime(%v) = %q, want %q", tt.d, got, tt.want)
		}
	}
}

// TestSafePrintf_Concurrent 并发 goroutine 各打印一行时，输出应恰好为 N 条完整行，
// 不出现交错（行数 == goroutine 数即说明每条 SafePrintf 原子完成）。
func TestSafePrintf_Concurrent(t *testing.T) {
	// 该测试需串行捕获 os.Stdout，不能 t.Parallel
	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	const n = 200
	var wg sync.WaitGroup
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			SafePrintf("line-%d\n", i)
		}(i)
	}
	wg.Wait()
	w.Close()
	os.Stdout = old

	out, _ := io.ReadAll(r)
	if got := strings.Count(string(out), "\n"); got != n {
		t.Errorf("并发输出应得到 %d 条完整行，实际 %d 行（存在交错）", n, got)
	}
	for i := 0; i < n; i++ {
		if !strings.Contains(string(out), "line-"+strconv.Itoa(i)+"\n") {
			t.Errorf("输出缺失完整行 line-%d", i)
			break
		}
	}
}
