package deploy

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"sync"

	"deploy-tool/internal/compiler"
	"deploy-tool/internal/config"
	"deploy-tool/internal/upload"
	"deploy-tool/pkg/utils"
)

// DeployJavaProject 部署 Java 项目。
// ctx 贯穿到本地构建（compiler.BuildJavaProject）以支持超时与取消（如 Ctrl+C）。
func DeployJavaProject(ctx context.Context, projectDir, configPath string, environments, models []string, uploadOnly bool) error {
	// 构建 Java 项目
	if err := compiler.BuildJavaProject(ctx, projectDir); err != nil {
		return err
	}

	// 先收集所有有效部署任务，再统一启动 goroutine。errChan 缓冲设为任务总数：
	// 每个 goroutine 至多发送 1 个错误（发送后立即 return），故缓冲永远够用，
	// 既不会阻塞 goroutine 导致 wg.Wait() 死锁，也不会丢失错误。
	type javaTask struct {
		jarName string
		jarPath string
		env     string
		cfg     *config.DeployConfig
	}
	var tasks []javaTask
	for _, env := range environments {
		cfg, err := config.FromFile(configPath, env)
		if err != nil {
			fmt.Printf("加载%s环境配置失败: %v\n", env, err)
			continue
		}

		// 获取 JAR 文件列表
		jarFiles := cfg.GetJarFilesList()
		if len(jarFiles) == 0 {
			fmt.Printf("配置文件中jar_files格式错误，必须是字符串或字符串数组\n")
			continue
		}

		for _, jarName := range jarFiles {
			// 如果指定了模块，检查是否需要部署
			if len(models) > 0 {
				moduleName := strings.Split(jarName, ".")[0]
				if !contains(models, moduleName) {
					fmt.Printf("%s模块不参与部署\n", jarName)
					continue
				}
			}

			// 确定 JAR 文件路径
			var jarPath string
			if len(jarFiles) > 1 {
				// 多模块项目
				moduleName := strings.Split(jarName, ".")[0]
				jarPath = filepath.Join(projectDir, moduleName, "target", jarName)
			} else {
				// 单模块项目
				jarPath = filepath.Join(projectDir, "target", jarName)
			}

			tasks = append(tasks, javaTask{jarName: jarName, jarPath: jarPath, env: env, cfg: cfg})
		}
	}

	// 使用 WaitGroup 等待所有部署任务完成
	var wg sync.WaitGroup
	errChan := make(chan error, len(tasks))

	for _, t := range tasks {
		wg.Add(1)
		go func(t javaTask) {
			defer wg.Done()

			remotePath := filepath.Join(t.cfg.RemoteBasePath, t.jarName)
			// 统一使用 Unix 风格路径
			remotePath = strings.ReplaceAll(remotePath, "\\", "/")

			// 确定是否仅上传
			finalUploadOnly := uploadOnly || t.cfg.UploadOnly

			if finalUploadOnly {
				utils.SafePrintf("开始上传 %s 到 %s 环境\n", t.jarName, t.env)
				if err := upload.UploadJarOnly(
					t.cfg.Server,
					t.cfg.Username,
					t.cfg.Password,
					t.jarPath,
					remotePath,
				); err != nil {
					errChan <- fmt.Errorf("上传失败 %s (%s环境): %w", t.jarName, t.env, err)
					return
				}
				utils.SafePrintf("上传成功: %s (%s环境)\n", t.jarName, t.env)
			} else {
				utils.SafePrintf("开始部署 %s 到 %s 环境\n", t.jarName, t.env)
				if err := upload.UploadAndRunJar(
					t.cfg.Server,
					t.cfg.Username,
					t.cfg.Password,
					t.jarPath,
					remotePath,
					t.cfg.JavaPath,
					t.env,
				); err != nil {
					errChan <- fmt.Errorf("部署失败 %s (%s环境): %w", t.jarName, t.env, err)
					return
				}
				utils.SafePrintf("部署成功: %s (%s环境)\n", t.jarName, t.env)
			}
		}(t)
	}

	// 等待所有任务完成
	wg.Wait()
	close(errChan)

	// 收集错误
	var errors []error
	for err := range errChan {
		errors = append(errors, err)
		fmt.Printf("错误: %v\n", err)
	}

	if len(errors) > 0 {
		return fmt.Errorf("部署过程中发生 %d 个错误", len(errors))
	}

	return nil
}

// contains 检查切片是否包含指定元素
func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}
