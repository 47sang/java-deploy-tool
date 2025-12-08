package compiler

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"deploy-tool/internal/discovery"
	"deploy-tool/pkg/utils"
)

// BuildJavaProject 构建 Java 项目
func BuildJavaProject(projectDir string) error {
	fmt.Println("正在初始化构建流程 (v2025.12.08)...")

	// 检测并设置合适的 JDK 版本
	javaHome, err := detectJavaVersion(projectDir)
	if err != nil {
		return err
	}
	fmt.Printf("使用 JAVA_HOME: %s\n", javaHome)

	// 查找 Maven 可执行文件
	mavenInfo, err := discovery.FindMaven()
	if err != nil {
		return err
	}

	// 构建 Maven 命令参数
	args := []string{"clean", "package", "-DskipTests"}

	fmt.Printf("执行构建命令: %s %s\n", mavenInfo.Path, strings.Join(args, " "))

	// 创建命令
	cmd := exec.Command(mavenInfo.Path, args...)
	cmd.Dir = projectDir

	// 设置环境变量
	cmd.Env = os.Environ()
	cmd.Env = append(cmd.Env, fmt.Sprintf("JAVA_HOME=%s", javaHome))

	// 获取标准输出管道
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("无法获取标准输出管道: %v", err)
	}

	stderr, err := cmd.StderrPipe()
	if err != nil {
		return fmt.Errorf("无法获取标准错误管道: %v", err)
	}

	// 启动命令
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("无法启动构建命令: %v", err)
	}

	// 读取并显示标准输出
	go func() {
		scanner := bufio.NewScanner(stdout)
		for scanner.Scan() {
			fmt.Println(scanner.Text())
		}
	}()

	// 读取标准错误
	var stderrOutput strings.Builder
	go func() {
		scanner := bufio.NewScanner(stderr)
		for scanner.Scan() {
			line := scanner.Text()
			stderrOutput.WriteString(line + "\n")
			fmt.Println(line)
		}
	}()

	// 等待命令完成
	if err := cmd.Wait(); err != nil {
		errorMsg := stderrOutput.String()
		if strings.TrimSpace(errorMsg) == "" {
			errorMsg = "未获取到错误输出 (stderr 为空)。可能是命令未找到或环境变量配置错误。"
		}
		return fmt.Errorf("构建失败 (退出码: %v)\n错误详情:\n%s\n\n建议:\n1. 检查 JAVA_HOME 是否正确: %s\n2. 检查 Maven 路径: %s\n3. 尝试手动执行: %s %s",
			err, errorMsg, javaHome, mavenInfo.Path, mavenInfo.Path, strings.Join(args, " "))
	}

	utils.PrintSuccess("Java 项目构建成功!")
	return nil
}

// detectJavaVersion 检测项目需要的 Java 版本并返回对应的 JAVA_HOME 路径
func detectJavaVersion(projectDir string) (string, error) {
	pomPath := filepath.Join(projectDir, "pom.xml")

	content, err := os.ReadFile(pomPath)
	if err != nil {
		// 如果读取 pom.xml 失败，尝试查找任意可用的 JDK
		utils.PrintWarning("无法读取pom.xml文件，将使用默认JDK")
		jdkInfo, err := discovery.FindAnyJDK()
		if err != nil {
			return "", err
		}
		return jdkInfo.Path, nil
	}

	pomContent := string(content)
	var javaVersion string

	// 检查 Java 版本配置
	if containsJavaVersion(pomContent, "8") {
		javaVersion = "8"
	} else if containsJavaVersion(pomContent, "11") {
		javaVersion = "11"
	} else if containsJavaVersion(pomContent, "17") {
		javaVersion = "17"
	} else if containsJavaVersion(pomContent, "21") {
		javaVersion = "21"
	} else {
		// 默认使用 Java 8
		utils.PrintWarning("未检测到明确的Java版本配置，默认使用Java 8")
		javaVersion = "8"
	}

	// 使用智能检测函数查找对应版本的 JDK
	jdkInfo, err := discovery.FindJDK(javaVersion)
	if err != nil {
		return "", err
	}
	return jdkInfo.Path, nil
}

// containsJavaVersion 检查 pom.xml 是否包含指定的 Java 版本配置
func containsJavaVersion(content string, version string) bool {
	patterns := []string{
		fmt.Sprintf("<java.version>%s</java.version>", version),
		fmt.Sprintf("<maven.compiler.source>%s</maven.compiler.source>", version),
		fmt.Sprintf("<maven.compiler.target>%s</maven.compiler.target>", version),
	}

	// 对于 Java 8，还需要检查 1.8 格式
	if version == "8" {
		patterns = append(patterns,
			"<java.version>1.8</java.version>",
			"<maven.compiler.source>1.8</maven.compiler.source>",
			"<maven.compiler.target>1.8</maven.compiler.target>",
		)
	}

	for _, pattern := range patterns {
		if strings.Contains(content, pattern) {
			return true
		}
	}
	return false
}
