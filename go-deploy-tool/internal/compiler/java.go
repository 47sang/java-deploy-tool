package compiler

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"deploy-tool/internal/discovery"
	"deploy-tool/pkg/utils"
)

// BuildJavaProject 构建 Java 项目。
// ctx 用于整体取消控制（如 Ctrl+C）；内部叠加 timeout.BuildTimeout 作为单次构建上限，
// 超时后自动终止 Maven 子进程，避免构建卡死（下载依赖、网络问题）永久阻塞部署。
func BuildJavaProject(ctx context.Context, projectDir string) error {
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

	// 注入 JAVA_HOME 环境变量：覆盖式设置，确保覆盖父进程可能已有的同名变量并生效
	// （直接 append 会在父进程已含 JAVA_HOME 时产生重复 key，Unix 取第一个导致旧值残留）
	env := overrideEnv(os.Environ(), "JAVA_HOME", javaHome)

	stderrOutput, err := runBuildCommand(ctx, mavenInfo.Path, args, projectDir, env)
	if err != nil {
		// 构建超时：错误已含超时提示，直接返回
		if errors.Is(err, context.DeadlineExceeded) {
			return err
		}
		msg := stderrOutput
		if strings.TrimSpace(msg) == "" {
			msg = "未获取到错误输出 (stderr 为空)。可能是命令未找到或环境变量配置错误。"
		}
		return fmt.Errorf("构建失败 (退出码: %w)\n错误详情:\n%s\n\n建议:\n1. 检查 JAVA_HOME 是否正确: %s\n2. 检查 Maven 路径: %s\n3. 尝试手动执行: %s %s",
			err, msg, javaHome, mavenInfo.Path, mavenInfo.Path, strings.Join(args, " "))
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
