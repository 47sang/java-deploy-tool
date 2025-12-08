package main

import (
	"fmt"
	"os"
	"strings"
	"time"

	"deploy-tool/internal/config"
	"deploy-tool/internal/deploy"
	"deploy-tool/pkg/utils"

	"github.com/spf13/cobra"
)

const (
	version = "2.0.0"
	author  = "士钰 <zhoushiyu92@gmail.com>"
)

var (
	// 命令行参数
	environments   []string
	vueEnvironments []string
	models         []string
	projectDir     string
	uploadOnly     bool
	initConfig     bool
)

func main() {
	rootCmd := &cobra.Command{
		Use:     "deploy-tool",
		Short:   "一键部署Java和Vue项目",
		Long:    "一键部署Java和Vue项目,支持多环境部署,支持多模块部署\n支持JDK/Maven自动发现",
		Version: version,
		Run:     runDeploy,
	}

	// 设置版本模板
	rootCmd.SetVersionTemplate(fmt.Sprintf("deploy-tool version %s\nAuthor: %s\n", version, author))

	// 添加命令行参数
	rootCmd.Flags().StringSliceVarP(&environments, "env", "e", nil,
		"部署后端服务环境，多个环境用逗号分隔 (例如: dev,prod)")
	rootCmd.Flags().StringSliceVarP(&vueEnvironments, "vue", "v", nil,
		"部署web端环境，多个环境用逗号分隔 (例如: dev,prod)")
	rootCmd.Flags().StringSliceVarP(&models, "model", "m", nil,
		"部署jar模块，多个模块用逗号分隔 (例如: admin,client,websocket)")
	rootCmd.Flags().StringVarP(&projectDir, "project-dir", "p", ".",
		"指定项目根目录路径")
	rootCmd.Flags().BoolVarP(&uploadOnly, "upload-only", "u", false,
		"仅上传文件到服务器，不执行命令")
	rootCmd.Flags().BoolVar(&initConfig, "init-config", false,
		"创建示例配置文件")

	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

func runDeploy(cmd *cobra.Command, args []string) {
	startTime := time.Now()
	fmt.Println("开始执行脚本程序")

	configPath := "./deploy.toml"

	// 如果指定了 init-config 参数，创建示例配置文件并退出
	if initConfig {
		if err := config.CreateSpringBootConfig(configPath); err != nil {
			fmt.Printf("创建配置文件失败: %v\n", err)
			return
		}
		fmt.Printf("示例配置文件已创建: %s\n", configPath)
		fmt.Println("请修改配置文件中的参数后再运行部署。")
		return
	}

	// 打印配置信息
	utils.PrintStep(1, "项目根目录: %s", projectDir)
	utils.PrintStep(2, "后端环境: %v", environments)
	utils.PrintStep(3, "web端环境: %v", vueEnvironments)
	utils.PrintStep(4, "部署模块: %v", models)

	// 显示 upload_only 的实际配置情况
	if uploadOnly {
		utils.PrintStep(5, "仅上传模式: %v (来自命令行参数)", uploadOnly)
	} else {
		utils.PrintStep(5, "仅上传模式: (将根据各环境配置文件决定)")
		// 如果有环境参数，显示每个环境的 upload_only 配置
		if len(environments) > 0 || len(vueEnvironments) > 0 {
			allEnvs := make(map[string]bool)
			for _, env := range environments {
				allEnvs[env] = true
			}
			for _, env := range vueEnvironments {
				allEnvs[env] = true
			}

			for env := range allEnvs {
				cfg, err := config.FromFile(configPath, env)
				if err != nil {
					fmt.Printf("   - %s 环境配置: 读取失败\n", env)
				} else {
					fmt.Printf("   - %s 环境配置: upload_only = %v\n", env, cfg.UploadOnly)
				}
			}
		}
	}

	// 部署 Java 项目
	if len(environments) > 0 {
		utils.PrintStep(6, "开始编译Java项目,请稍等...")
		if err := deploy.DeployJavaProject(projectDir, configPath, environments, models, uploadOnly); err != nil {
			fmt.Printf("部署失败: %v\n", err)
		}
	}

	// 部署 Vue 项目
	if len(vueEnvironments) > 0 {
		utils.PrintStep(6, "开始编译Vue项目,比较慢,请稍等...")
		if err := deploy.DeployVueProject(projectDir, configPath, vueEnvironments, uploadOnly); err != nil {
			fmt.Printf("部署失败: %v\n", err)
		}
	}

	// 打印执行时间
	elapsed := time.Since(startTime)
	fmt.Printf("本次部署执行时间: %s\n", utils.FormatExecutionTime(elapsed))
	fmt.Printf("当前系统时间: %s\n", time.Now().Format("2006-01-02 15:04:05"))
}

// parseCommaSeparated 解析逗号分隔的字符串
func parseCommaSeparated(input string) []string {
	if input == "" {
		return nil
	}
	parts := strings.Split(input, ",")
	result := make([]string, 0, len(parts))
	for _, part := range parts {
		trimmed := strings.TrimSpace(part)
		if trimmed != "" {
			result = append(result, trimmed)
		}
	}
	return result
}
