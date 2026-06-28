package utils

import (
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
