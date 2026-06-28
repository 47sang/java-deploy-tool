package compiler

import "testing"

// TestContainsJavaVersion 覆盖各版本标签、Java 8 的 1.8 旧式写法及不匹配场景。
func TestContainsJavaVersion(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		content string
		version string
		want    bool
	}{
		{"java.version 标签", "<java.version>17</java.version>", "17", true},
		{"maven.compiler.source", "<maven.compiler.source>11</maven.compiler.source>", "11", true},
		{"maven.compiler.target", "<maven.compiler.target>21</maven.compiler.target>", "21", true},
		{"Java8 现代写法", "<java.version>8</java.version>", "8", true},
		{"Java8 旧式1.8写法", "<java.version>1.8</java.version>", "8", true},
		{"Java8 1.8 source", "<maven.compiler.source>1.8</maven.compiler.source>", "8", true},
		{"Java8 1.8 target", "<maven.compiler.target>1.8</maven.compiler.target>", "8", true},
		{"版本不匹配", "<java.version>17</java.version>", "11", false},
		{"无配置", "<project></project>", "17", false},
		{"空内容", "", "8", false},
		{"pom 完整片段", `<properties><java.version>17</java.version><maven.compiler.source>17</maven.compiler.source></properties>`, "17", true},
	}
	for _, tt := range tests {
		tt := tt // go 1.21 语义下需显式重新绑定循环变量，避免并行子测试共享 tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := containsJavaVersion(tt.content, tt.version); got != tt.want {
				t.Errorf("containsJavaVersion(...%q...) version=%q = %v, want %v",
					tt.content, tt.version, got, tt.want)
			}
		})
	}
}
