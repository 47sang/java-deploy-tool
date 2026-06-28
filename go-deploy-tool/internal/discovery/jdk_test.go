package discovery

import "testing"

// TestParseJavaVersion 覆盖各主版本、1.8 旧式输出及无法识别的输出。
func TestParseJavaVersion(t *testing.T) {
	t.Parallel()
	tests := []struct {
		output string
		want   string
	}{
		{`openjdk version "1.8.0_292"`, "8"},
		{`openjdk version "8"`, "8"},
		{`java version "1.8.0_301"`, "8"},
		{`openjdk version "11.0.12"`, "11"},
		{`openjdk version "11"`, "11"},
		{`openjdk version "17.0.1" 2021-10-19`, "17"},
		{`openjdk version "21" 2023-09-19`, "21"},
		{`java version "17.0.8"`, "17"},
		{`unexpected output`, ""},
		{``, ""},
	}
	for _, tt := range tests {
		if got := parseJavaVersion(tt.output); got != tt.want {
			t.Errorf("parseJavaVersion(%q) = %q, want %q", tt.output, got, tt.want)
		}
	}
}
