package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/user/deploy-tool/build"
	"github.com/user/deploy-tool/config"
	"github.com/user/deploy-tool/upload"
)

// 测量执行时间的函数
func measureExecutionTime(f func()) time.Duration {
	start := time.Now()
	f()
	elapsed := time.Since(start)
	
	hours := int(elapsed.Hours())
	minutes := int(elapsed.Minutes()) % 60
	seconds := int(elapsed.Seconds()) % 60
	
	formattedTime := fmt.Sprintf("%02d:%02d:%02d", hours, minutes, seconds)
	fmt.Printf("本次部署执行时间: %s\n", formattedTime)
	
	now := time.Now()
	fmt.Printf("当前系统时间: %s\n", now.Format("2006-01-02 15:04:05"))
	
	return elapsed
}

// 部署Java项目
func deployJavaProject(projectDir, configPath string, environments, models []string) error {
	// 构建Java项目
	if err := build.BuildJavaProject(projectDir); err != nil {
		return err
	}

	// 为每个环境创建部署任务
	var wg sync.WaitGroup
	errorChan := make(chan error, len(environments)*3) // 假设每个环境最多3个jar包

	for _, env := range environments {
		// 加载环境配置
		cfg, err := config.FromFile(configPath, env)
		if err != nil {
			fmt.Fprintf(os.Stderr, "加载%s环境配置失败: %v\n", env, err)
			continue
		}

		for _, jarName := range cfg.JarFiles {
			// 检查是否需要部署此模块
			if len(models) > 0 {
				// 从jar名称中提取模块名
				moduleName := strings.Split(jarName, ".")[0]
				shouldDeploy := false
				for _, model := range models {
					if model == moduleName {
						shouldDeploy = true
						break
					}
				}
				if !shouldDeploy {
					fmt.Printf("%s模块不参与部署\n", jarName)
					continue
				}
			}

			// 复制变量到闭包中使用
			env := env
			jarName := jarName
			cfg := cfg

			wg.Add(1)
			go func() {
				defer wg.Done()

				// 获取编译产物文件名称，组装上传路径
				var jarPath string
				if len(cfg.JarFiles) == 1 {
					jarPath = filepath.Join(projectDir, "target", jarName)
				} else {
					jarPath = filepath.Join(projectDir, strings.Split(jarName, ".")[0], "target", jarName)
				}

				remotePath := filepath.Join(cfg.RemoteBasePath, jarName)
				remotePath = strings.ReplaceAll(remotePath, "\\", "/") // 确保远程路径使用斜杠

				fmt.Printf("开始部署 %s 到 %s 环境\n", jarName, env)

				// 上传并运行JAR包
				err := upload.UploadAndRunJar(
					cfg.Server,
					cfg.Username,
					cfg.Password,
					jarPath,
					remotePath,
					cfg.JavaPath,
					env,
				)
				if err != nil {
					errorChan <- fmt.Errorf("部署失败 %s (%s环境): %v", jarName, env, err)
					return
				}
				fmt.Printf("部署成功: %s (%s环境)\n", jarName, env)
			}()
		}
	}

	// 等待所有任务完成
	wg.Wait()
	close(errorChan)

	// 检查是否有错误
	var errors []string
	for err := range errorChan {
		errors = append(errors, err.Error())
	}

	if len(errors) > 0 {
		return fmt.Errorf("部署过程中出现错误:\n%s", strings.Join(errors, "\n"))
	}

	return nil
}

// 部署Vue项目
func deployVueProject(projectDir, configPath string, environments []string) error {
	// 为每个环境创建部署任务
	var wg sync.WaitGroup
	errorChan := make(chan error, len(environments))

	for _, env := range environments {
		// 加载环境配置
		cfg, err := config.FromFile(configPath, env)
		if err != nil {
			fmt.Fprintf(os.Stderr, "加载%s环境配置失败: %v\n", env, err)
			continue
		}

		// 复制变量到闭包中使用
		env := env
		cfg := cfg

		wg.Add(1)
		go func() {
			defer wg.Done()

			// 构建Vue项目
			if err := build.BuildVueProject(projectDir, cfg.Scripts); err != nil {
				errorChan <- fmt.Errorf("构建Vue项目失败 (%s环境): %v", env, err)
				return
			}

			// 压缩产出目录
			outputDir := filepath.Join(projectDir, cfg.OutputDir)
			zipPath := fmt.Sprintf("%s.zip", outputDir)
			if err := build.ZipDirectory(outputDir, zipPath); err != nil {
				errorChan <- fmt.Errorf("压缩Vue项目失败 (%s环境): %v", env, err)
				return
			}

			// 上传zip文件
			remotePath := filepath.Join(cfg.RemoteBasePath, cfg.OutputDir)
			remotePath = strings.ReplaceAll(remotePath, "\\", "/") // 确保远程路径使用斜杠

			if err := upload.UploadFile(
				cfg.Server,
				cfg.Username,
				cfg.Password,
				zipPath,
				remotePath,
			); err != nil {
				errorChan <- fmt.Errorf("上传Vue项目失败 (%s环境): %v", env, err)
				return
			}

			fmt.Printf("上传成功: %s (%s环境)\n", cfg.OutputDir, env)
		}()
	}

	// 等待所有任务完成
	wg.Wait()
	close(errorChan)

	// 检查是否有错误
	var errors []string
	for err := range errorChan {
		errors = append(errors, err.Error())
	}

	if len(errors) > 0 {
		return fmt.Errorf("部署过程中出现错误:\n%s", strings.Join(errors, "\n"))
	}

	return nil
}

func main() {
	// 定义命令行参数
	var (
		env         = flag.String("env", "", "部署后端服务环境，多个环境用逗号分隔 (例如: dev,prod)")
		vue         = flag.String("vue", "", "部署web端环境，多个环境用逗号分隔 (例如: dev,prod)")
		model       = flag.String("model", "", "部署jar模块，多个模块用逗号分隔 (例如: admin,client,websocket)")
		initConfig  = flag.Bool("init-config", false, "创建示例配置文件")
		projectDir  = flag.String("project-dir", ".", "指定项目根目录路径")
	)

	// 添加简短形式的命令行参数
	flag.StringVar(env, "e", "", "部署后端服务环境，多个环境用逗号分隔 (例如: dev,prod)")
	flag.StringVar(vue, "v", "", "部署web端环境，多个环境用逗号分隔 (例如: dev,prod)")
	flag.StringVar(model, "m", "", "部署jar模块，多个模块用逗号分隔 (例如: admin,client,websocket)")
	flag.StringVar(projectDir, "p", ".", "指定项目根目录路径")

	// 解析命令行参数
	flag.Parse()

	// 测量执行时间
	measureExecutionTime(func() {
		fmt.Println("开始执行脚本程序")
		configPath := "./deploy.toml"

		// 如果指定了init-config参数，创建示例配置文件并退出
		if *initConfig {
			if err := config.CreateSpringbootConfig(configPath); err != nil {
				fmt.Fprintf(os.Stderr, "创建配置文件失败: %v\n", err)
				return
			}
			fmt.Printf("示例配置文件已创建: %s\n", configPath)
			fmt.Println("请修改配置文件中的参数后再运行部署。")
			return
		}

		// 解析环境列表
		var environments, vueEnvironments, models []string
		if *env != "" {
			environments = strings.Split(*env, ",")
		}
		if *vue != "" {
			vueEnvironments = strings.Split(*vue, ",")
		}
		if *model != "" {
			models = strings.Split(*model, ",")
		}

		fmt.Printf("1.项目根目录: %s\n", *projectDir)
		fmt.Printf("2.后端环境: %v\n", environments)
		fmt.Printf("3.web端环境: %v\n", vueEnvironments)
		fmt.Printf("4.部署模块: %v\n", models)

		// 根据命令行参数选择执行部署函数
		if len(environments) > 0 {
			fmt.Println("5.开始编译Java项目，请稍等...")
			if err := deployJavaProject(*projectDir, configPath, environments, models); err != nil {
				fmt.Fprintf(os.Stderr, "%v\n", err)
			}
		}

		if len(vueEnvironments) > 0 {
			fmt.Println("5.开始编译Vue项目，比较慢，请稍等...")
			if err := deployVueProject(*projectDir, configPath, vueEnvironments); err != nil {
				fmt.Fprintf(os.Stderr, "%v\n", err)
			}
		}
	})
} 