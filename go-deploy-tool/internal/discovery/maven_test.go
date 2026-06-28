package discovery

import "testing"

// TestParseMavenVersion 覆盖标准 Maven --version 输出与无法识别的输出。
func TestParseMavenVersion(t *testing.T) {
	t.Parallel()
	tests := []struct {
		output string
		want   string
	}{
		{"Apache Maven 3.9.6 (ff97f057e...)\nMaven home: /opt/maven\n", "3.9.6"},
		{"Apache Maven 3.6.3\n", "3.6.3"},
		{"some unrelated output", "unknown"},
		{"", "unknown"},
	}
	for _, tt := range tests {
		if got := parseMavenVersion(tt.output); got != tt.want {
			t.Errorf("parseMavenVersion(%q) = %q, want %q", tt.output, got, tt.want)
		}
	}
}

func TestSplitLines(t *testing.T) {
	t.Parallel()
	got := splitLines("a\nb\r\nc")
	want := []string{"a", "b", "c"}
	if len(got) != len(want) {
		t.Fatalf("splitLines = %#v, want %#v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}

func TestSplitFields(t *testing.T) {
	t.Parallel()
	got := splitFields("a\tb  c")
	want := []string{"a", "b", "c"}
	if len(got) != len(want) {
		t.Fatalf("splitFields = %#v, want %#v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}

func TestContainsAndFindSubstring(t *testing.T) {
	t.Parallel()
	if !contains("hello world", "world") {
		t.Error(`contains("hello world","world") 应为 true`)
	}
	if contains("hello", "world") {
		t.Error(`contains("hello","world") 应为 false`)
	}
	if findSubstring("hello", "ll") != 2 {
		t.Error(`findSubstring("hello","ll") 应为 2`)
	}
	if findSubstring("hello", "x") != -1 {
		t.Error(`findSubstring("hello","x") 应为 -1`)
	}
}
