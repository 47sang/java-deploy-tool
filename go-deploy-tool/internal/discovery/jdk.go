package discovery

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"deploy-tool/pkg/utils"
)

// JDKInfo JDK 信息结构
type JDKInfo struct {
	Path    string // JDK 路径 (JAVA_HOME)
	Version string // 版本号 (8, 11, 17, 21)
	Source  string // 来源描述
}

// FindJDK 查找指定版本的 JDK
// 优先级：环境变量 > 自动发现路径
func FindJDK(requiredVersion string) (*JDKInfo, error) {
	utils.PrintInfo("正在搜索Java %s...", requiredVersion)

	// 1. 首先检查环境变量 JAVA_HOME
	if javaHome := os.Getenv("JAVA_HOME"); javaHome != "" {
		if info := checkJavaVersion(javaHome); info != nil {
			if info.Version == requiredVersion {
				utils.PrintSuccess("使用环境变量JAVA_HOME: %s", javaHome)
				info.Source = "环境变量 JAVA_HOME"
				return info, nil
			}
			utils.PrintWarning("JAVA_HOME版本不匹配(需要%s,当前%s),继续搜索...", requiredVersion, info.Version)
		}
	}

	// 2. 搜索所有可能的 JDK 路径
	searchPaths := getJDKSearchPaths()
	var foundJDKs []*JDKInfo

	for _, pattern := range searchPaths {
		matches, err := filepath.Glob(pattern)
		if err != nil {
			continue
		}
		for _, path := range matches {
			if info := checkJavaVersion(path); info != nil {
				fmt.Printf("   发现 Java %s at %s\n", info.Version, path)
				foundJDKs = append(foundJDKs, info)
			}
		}
	}

	// 3. 查找匹配版本的 JDK
	for _, info := range foundJDKs {
		if info.Version == requiredVersion {
			utils.PrintSuccess("找到匹配的Java %s: %s", requiredVersion, info.Path)
			return info, nil
		}
	}

	// 4. 如果没找到，返回详细错误信息
	if len(foundJDKs) == 0 {
		return nil, fmt.Errorf("未找到任何Java安装。请安装Java %s。\n建议:\n  - macOS: brew install openjdk@%s\n  - Windows: 下载并安装Zulu JDK %s\n  - Linux: sudo apt install openjdk-%s-jdk",
			requiredVersion, requiredVersion, requiredVersion, requiredVersion)
	}

	var availableVersions []string
	for _, info := range foundJDKs {
		availableVersions = append(availableVersions, fmt.Sprintf("  - Java %s at %s", info.Version, info.Path))
	}
	return nil, fmt.Errorf("未找到Java %s,但发现以下版本:\n%s\n\n请安装Java %s或修改项目配置使用已有版本。",
		requiredVersion, strings.Join(availableVersions, "\n"), requiredVersion)
}

// FindAnyJDK 查找任意可用的 JDK
func FindAnyJDK() (*JDKInfo, error) {
	utils.PrintInfo("正在搜索可用的Java...")

	// 1. 首先检查环境变量 JAVA_HOME
	if javaHome := os.Getenv("JAVA_HOME"); javaHome != "" {
		if info := checkJavaVersion(javaHome); info != nil {
			utils.PrintSuccess("使用环境变量JAVA_HOME: %s (Java %s)", javaHome, info.Version)
			info.Source = "环境变量 JAVA_HOME"
			return info, nil
		}
	}

	// 2. 检查 PATH 中的 java 命令
	if javaPath, err := exec.LookPath("java"); err == nil {
		// 获取 java 的真实路径
		realPath, err := filepath.EvalSymlinks(javaPath)
		if err == nil {
			// 推断 JAVA_HOME (通常是 bin 目录的父目录)
			javaHome := filepath.Dir(filepath.Dir(realPath))
			if info := checkJavaVersion(javaHome); info != nil {
				utils.PrintSuccess("使用PATH中的Java: %s (Java %s)", javaHome, info.Version)
				info.Source = "PATH 环境变量"
				return info, nil
			}
		}
	}

	// 3. 搜索所有可能的 JDK 路径
	searchPaths := getJDKSearchPaths()
	for _, pattern := range searchPaths {
		matches, err := filepath.Glob(pattern)
		if err != nil {
			continue
		}
		for _, path := range matches {
			if info := checkJavaVersion(path); info != nil {
				utils.PrintSuccess("自动发现Java %s: %s", info.Version, path)
				return info, nil
			}
		}
	}

	return nil, fmt.Errorf("未找到任何Java安装")
}

// getJDKSearchPaths 获取 JDK 搜索路径列表
func getJDKSearchPaths() []string {
	switch runtime.GOOS {
	case "darwin":
		homeDir, _ := os.UserHomeDir()
		return []string{
			// IntelliJ IDEA 捆绑的 JDK
			"/Applications/IntelliJ IDEA.app/Contents/jbr/Contents/Home",
			"/Applications/IntelliJ IDEA CE.app/Contents/jbr/Contents/Home",
			"/Applications/IntelliJ IDEA*.app/Contents/jbr/Contents/Home",
			// JetBrains Toolbox 安装的 IDEA
			filepath.Join(homeDir, "Library/Application Support/JetBrains/Toolbox/apps/IDEA-*/*/IntelliJ IDEA*.app/Contents/jbr/Contents/Home"),
			// 系统安装的 JDK
			"/Library/Java/JavaVirtualMachines/*/Contents/Home",
			"/Library/Java/JavaVirtualMachines/zulu-*/Contents/Home",
			"/Library/Java/JavaVirtualMachines/temurin-*/Contents/Home",
			"/Library/Java/JavaVirtualMachines/adoptopenjdk-*/Contents/Home",
			// Homebrew 安装的 JDK
			"/opt/homebrew/opt/openjdk@*/libexec/openjdk.jdk/Contents/Home",
			"/opt/homebrew/opt/openjdk/libexec/openjdk.jdk/Contents/Home",
			"/usr/local/opt/openjdk@*/libexec/openjdk.jdk/Contents/Home",
			"/usr/local/opt/openjdk/libexec/openjdk.jdk/Contents/Home",
			// SDKMAN 安装的 JDK
			filepath.Join(homeDir, ".sdkman/candidates/java/*/"),
			// Jabba 安装的 JDK
			filepath.Join(homeDir, ".jabba/jdk/*/Contents/Home"),
		}
	case "windows":
		homeDir, _ := os.UserHomeDir()
		return []string{
			// IntelliJ IDEA 捆绑的 JDK
			"C:\\Program Files\\JetBrains\\IntelliJ IDEA*\\jbr",
			"C:\\Program Files\\JetBrains\\IntelliJ IDEA Community*\\jbr",
			// JetBrains Toolbox 安装的 IDEA
			filepath.Join(homeDir, "AppData\\Local\\JetBrains\\Toolbox\\apps\\IDEA-*\\*\\jbr"),
			// Oracle JDK
			"C:\\Program Files\\Java\\jdk*",
			"C:\\Program Files\\Java\\jdk-*",
			// Zulu JDK
			"C:\\Program Files\\Zulu\\zulu-*",
			// Eclipse Temurin
			"C:\\Program Files\\Eclipse Adoptium\\jdk-*",
			"C:\\Program Files\\Eclipse Foundation\\jdk-*",
			// Amazon Corretto
			"C:\\Program Files\\Amazon Corretto\\*",
			// Microsoft Build of OpenJDK
			"C:\\Program Files\\Microsoft\\jdk-*",
			// Scoop 安装的 JDK
			filepath.Join(homeDir, "scoop\\apps\\openjdk*\\current"),
			filepath.Join(homeDir, "scoop\\apps\\temurin*\\current"),
			filepath.Join(homeDir, "scoop\\apps\\zulu*\\current"),
		}
	default: // Linux
		homeDir, _ := os.UserHomeDir()
		return []string{
			// IntelliJ IDEA 捆绑的 JDK (Toolbox)
			filepath.Join(homeDir, ".local/share/JetBrains/Toolbox/apps/IDEA-*/*/jbr"),
			// Snap 安装的 IDEA
			"/snap/intellij-idea-*/current/jbr",
			// 系统安装的 JDK
			"/usr/lib/jvm/*",
			"/usr/java/*",
			"/usr/local/java/*",
			// 手动安装的 JDK
			"/opt/java/*",
			"/opt/jdk*",
			"/opt/openjdk*",
			"/opt/zulu*",
			// SDKMAN 安装的 JDK
			filepath.Join(homeDir, ".sdkman/candidates/java/*/"),
			// Jabba 安装的 JDK
			filepath.Join(homeDir, ".jabba/jdk/*/"),
		}
	}
}

// checkJavaVersion 检查指定路径的 Java 版本
func checkJavaVersion(javaHome string) *JDKInfo {
	var javaBin string
	if runtime.GOOS == "windows" {
		javaBin = filepath.Join(javaHome, "bin", "java.exe")
	} else {
		javaBin = filepath.Join(javaHome, "bin", "java")
	}

	// 检查文件是否存在
	if _, err := os.Stat(javaBin); os.IsNotExist(err) {
		return nil
	}

	// 执行 java -version 获取版本信息
	cmd := exec.Command(javaBin, "-version")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return nil
	}

	versionOutput := string(output)
	version := parseJavaVersion(versionOutput)
	if version == "" {
		return nil
	}

	return &JDKInfo{
		Path:    javaHome,
		Version: version,
		Source:  "自动发现",
	}
}

// parseJavaVersion 解析 Java 版本输出
func parseJavaVersion(output string) string {
	output = strings.ToLower(output)

	// 检查各版本
	if strings.Contains(output, "\"1.8.") || strings.Contains(output, "\"8.") ||
		strings.Contains(output, "version \"8") || strings.Contains(output, "openjdk version \"1.8") {
		return "8"
	}
	if strings.Contains(output, "\"11.") || strings.Contains(output, "version \"11") {
		return "11"
	}
	if strings.Contains(output, "\"17.") || strings.Contains(output, "version \"17") {
		return "17"
	}
	if strings.Contains(output, "\"21.") || strings.Contains(output, "version \"21") {
		return "21"
	}

	return ""
}

// GetJavaBinPath 获取 java 可执行文件路径
func GetJavaBinPath(javaHome string) string {
	if runtime.GOOS == "windows" {
		return filepath.Join(javaHome, "bin", "java.exe")
	}
	return filepath.Join(javaHome, "bin", "java")
}

// GetJavacBinPath 获取 javac 可执行文件路径
func GetJavacBinPath(javaHome string) string {
	if runtime.GOOS == "windows" {
		return filepath.Join(javaHome, "bin", "javac.exe")
	}
	return filepath.Join(javaHome, "bin", "javac")
}
