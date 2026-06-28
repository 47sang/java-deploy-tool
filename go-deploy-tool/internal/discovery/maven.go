package discovery

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"

	"deploy-tool/internal/timeout"
	"deploy-tool/pkg/utils"
)

// MavenInfo Maven 信息结构
type MavenInfo struct {
	Path    string // Maven 可执行文件路径
	Version string // 版本号
	Source  string // 来源描述
}

// FindMaven 查找 Maven 可执行文件
// 优先级：环境变量/PATH > 自动发现路径
func FindMaven() (*MavenInfo, error) {
	utils.PrintInfo("正在搜索Maven...")

	// 1. 首先检查 MAVEN_HOME 环境变量
	if mavenHome := os.Getenv("MAVEN_HOME"); mavenHome != "" {
		mvnPath := getMvnExecutable(mavenHome)
		if info := checkMavenVersion(mvnPath); info != nil {
			utils.PrintSuccess("使用环境变量MAVEN_HOME: %s", mvnPath)
			info.Source = "环境变量 MAVEN_HOME"
			return info, nil
		}
	}

	// 也检查 M2_HOME (旧版本 Maven 使用)
	if m2Home := os.Getenv("M2_HOME"); m2Home != "" {
		mvnPath := getMvnExecutable(m2Home)
		if info := checkMavenVersion(mvnPath); info != nil {
			utils.PrintSuccess("使用环境变量M2_HOME: %s", mvnPath)
			info.Source = "环境变量 M2_HOME"
			return info, nil
		}
	}

	// 2. 检查 PATH 中是否有 mvn
	mvnCmd := "mvn"
	if runtime.GOOS == "windows" {
		mvnCmd = "mvn.cmd"
	}

	if mvnPath, err := exec.LookPath(mvnCmd); err == nil {
		if info := checkMavenVersion(mvnPath); info != nil {
			utils.PrintSuccess("使用PATH中的Maven: %s", mvnPath)
			info.Source = "PATH 环境变量"
			return info, nil
		}
	}

	// 3. 搜索所有可能的 Maven 路径
	searchPaths := getMavenSearchPaths()
	for _, pattern := range searchPaths {
		matches, err := filepath.Glob(pattern)
		if err != nil {
			continue
		}
		for _, path := range matches {
			if info := checkMavenVersion(path); info != nil {
				utils.PrintSuccess("找到Maven: %s", path)
				return info, nil
			}
		}
	}

	// 4. 检查 Maven Wrapper
	wrapperPath := getMavenWrapperPath()
	if wrapperPath != "" {
		if info := checkMavenVersion(wrapperPath); info != nil {
			utils.PrintSuccess("使用Maven Wrapper: %s", wrapperPath)
			info.Source = "Maven Wrapper"
			return info, nil
		}
	}

	return nil, fmt.Errorf("未找到Maven安装。请安装Maven。\n建议:\n  - macOS: brew install maven\n  - Windows: 下载并安装Apache Maven\n  - Linux: sudo apt install maven\n或者使用IntelliJ IDEA(自带Maven)")
}

// getMavenSearchPaths 获取 Maven 搜索路径列表
func getMavenSearchPaths() []string {
	homeDir, _ := os.UserHomeDir()

	switch runtime.GOOS {
	case "darwin":
		return []string{
			// IntelliJ IDEA 捆绑的 Maven
			"/Applications/IntelliJ IDEA.app/Contents/plugins/maven/lib/maven3/bin/mvn",
			"/Applications/IntelliJ IDEA CE.app/Contents/plugins/maven/lib/maven3/bin/mvn",
			"/Applications/IntelliJ IDEA*.app/Contents/plugins/maven/lib/maven3/bin/mvn",
			// JetBrains Toolbox 安装的 IDEA
			filepath.Join(homeDir, "Library/Application Support/JetBrains/Toolbox/apps/IDEA-*/*/IntelliJ IDEA*.app/Contents/plugins/maven/lib/maven3/bin/mvn"),
			// Homebrew 安装的 Maven
			"/opt/homebrew/bin/mvn",
			"/usr/local/bin/mvn",
			// 手动安装的 Maven
			"/usr/local/maven*/bin/mvn",
			"/usr/local/apache-maven*/bin/mvn",
			"/opt/maven*/bin/mvn",
			"/opt/apache-maven*/bin/mvn",
			// SDKMAN 安装的 Maven
			filepath.Join(homeDir, ".sdkman/candidates/maven/current/bin/mvn"),
			filepath.Join(homeDir, ".sdkman/candidates/maven/*/bin/mvn"),
		}
	case "windows":
		return []string{
			// IntelliJ IDEA 捆绑的 Maven
			"C:\\Program Files\\JetBrains\\IntelliJ IDEA*\\plugins\\maven\\lib\\maven3\\bin\\mvn.cmd",
			"C:\\Program Files\\JetBrains\\IntelliJ IDEA Community*\\plugins\\maven\\lib\\maven3\\bin\\mvn.cmd",
			// JetBrains Toolbox 安装的 IDEA
			filepath.Join(homeDir, "AppData\\Local\\JetBrains\\Toolbox\\apps\\IDEA-*\\*\\plugins\\maven\\lib\\maven3\\bin\\mvn.cmd"),
			// 手动安装的 Maven
			"C:\\Program Files\\Apache\\maven*\\bin\\mvn.cmd",
			"C:\\Program Files\\Maven\\apache-maven*\\bin\\mvn.cmd",
			"C:\\Program Files\\apache-maven*\\bin\\mvn.cmd",
			"C:\\maven*\\bin\\mvn.cmd",
			"C:\\apache-maven*\\bin\\mvn.cmd",
			// Scoop 安装的 Maven
			filepath.Join(homeDir, "scoop\\apps\\maven\\current\\bin\\mvn.cmd"),
			// Chocolatey 安装的 Maven
			"C:\\ProgramData\\chocolatey\\lib\\maven\\apache-maven*\\bin\\mvn.cmd",
		}
	default: // Linux
		return []string{
			// IntelliJ IDEA 捆绑的 Maven (Toolbox)
			filepath.Join(homeDir, ".local/share/JetBrains/Toolbox/apps/IDEA-*/*/plugins/maven/lib/maven3/bin/mvn"),
			// Snap 安装的 IDEA
			"/snap/intellij-idea-*/current/plugins/maven/lib/maven3/bin/mvn",
			// 系统安装的 Maven
			"/usr/bin/mvn",
			"/usr/local/bin/mvn",
			// 手动安装的 Maven
			"/usr/local/maven*/bin/mvn",
			"/usr/local/apache-maven*/bin/mvn",
			"/opt/maven*/bin/mvn",
			"/opt/apache-maven*/bin/mvn",
			// SDKMAN 安装的 Maven
			filepath.Join(homeDir, ".sdkman/candidates/maven/current/bin/mvn"),
			filepath.Join(homeDir, ".sdkman/candidates/maven/*/bin/mvn"),
		}
	}
}

// getMvnExecutable 获取 Maven 可执行文件路径
func getMvnExecutable(mavenHome string) string {
	if runtime.GOOS == "windows" {
		return filepath.Join(mavenHome, "bin", "mvn.cmd")
	}
	return filepath.Join(mavenHome, "bin", "mvn")
}

// getMavenWrapperPath 获取当前目录的 Maven Wrapper 路径
func getMavenWrapperPath() string {
	cwd, err := os.Getwd()
	if err != nil {
		return ""
	}

	var wrapperName string
	if runtime.GOOS == "windows" {
		wrapperName = "mvnw.cmd"
	} else {
		wrapperName = "mvnw"
	}

	wrapperPath := filepath.Join(cwd, wrapperName)
	if _, err := os.Stat(wrapperPath); err == nil {
		return wrapperPath
	}

	return ""
}

// checkMavenVersion 检查 Maven 版本
func checkMavenVersion(mvnPath string) *MavenInfo {
	// 检查文件是否存在
	if _, err := os.Stat(mvnPath); os.IsNotExist(err) {
		return nil
	}

	// 执行 mvn --version 获取版本信息（受探测超时约束，避免异常 Maven 卡住搜索）
	var output []byte
	if err := timeout.RunWithTimeout(timeout.ProbeTimeout, func() error {
		var e error
		output, e = exec.Command(mvnPath, "--version").CombinedOutput()
		return e
	}, nil); err != nil {
		return nil
	}

	version := parseMavenVersion(string(output))
	return &MavenInfo{
		Path:    mvnPath,
		Version: version,
		Source:  "自动发现",
	}
}

// parseMavenVersion 解析 Maven 版本输出
func parseMavenVersion(output string) string {
	// Maven 版本输出格式: "Apache Maven 3.9.6 (...)"
	// 简单提取版本号
	lines := splitLines(output)
	for _, line := range lines {
		if len(line) > 0 && (contains(line, "Apache Maven") || contains(line, "Maven")) {
			parts := splitFields(line)
			for i, part := range parts {
				if part == "Maven" && i+1 < len(parts) {
					return parts[i+1]
				}
			}
		}
	}
	return "unknown"
}

// 辅助函数
func splitLines(s string) []string {
	var lines []string
	start := 0
	for i := 0; i < len(s); i++ {
		if s[i] == '\n' {
			line := s[start:i]
			if len(line) > 0 && line[len(line)-1] == '\r' {
				line = line[:len(line)-1]
			}
			lines = append(lines, line)
			start = i + 1
		}
	}
	if start < len(s) {
		lines = append(lines, s[start:])
	}
	return lines
}

func splitFields(s string) []string {
	var fields []string
	start := -1
	for i := 0; i < len(s); i++ {
		if s[i] == ' ' || s[i] == '\t' {
			if start >= 0 {
				fields = append(fields, s[start:i])
				start = -1
			}
		} else {
			if start < 0 {
				start = i
			}
		}
	}
	if start >= 0 {
		fields = append(fields, s[start:])
	}
	return fields
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && findSubstring(s, substr) >= 0)
}

func findSubstring(s, substr string) int {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return i
		}
	}
	return -1
}
