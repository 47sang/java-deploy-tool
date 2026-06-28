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

// DeployVueProject 部署 Vue 项目。
// ctx 贯穿到每个环境的本地构建（compiler.BuildVueProject）以支持超时与取消（如 Ctrl+C）。
func DeployVueProject(ctx context.Context, projectDir, configPath string, environments []string, uploadOnly bool) error {
	// 先收集各环境配置（有效任务），再统一启动 goroutine。errChan 缓冲设为任务总数：
	// 每个 goroutine 至多发送 1 个错误（发送后立即 return），故缓冲永远够用，
	// 既不会阻塞 goroutine 导致 wg.Wait() 死锁，也不会丢失错误。
	type vueTask struct {
		env string
		cfg *config.DeployConfig
	}
	var tasks []vueTask
	for _, env := range environments {
		cfg, err := config.FromFile(configPath, env)
		if err != nil {
			fmt.Printf("加载%s环境配置失败: %v\n", env, err)
			continue
		}
		tasks = append(tasks, vueTask{env: env, cfg: cfg})
	}

	// 使用 WaitGroup 等待所有部署任务完成
	var wg sync.WaitGroup
	errChan := make(chan error, len(tasks))

	for _, t := range tasks {
		wg.Add(1)
		go func(t vueTask) {
			defer wg.Done()
			env, cfg := t.env, t.cfg

			// 构建 Vue 项目
			if err := compiler.BuildVueProject(ctx, projectDir, cfg.Scripts); err != nil {
				errChan <- fmt.Errorf("构建Vue项目失败 (%s环境): %w", env, err)
				return
			}

			// 压缩产出目录
			outputDir := filepath.Join(projectDir, cfg.OutputDir)
			zipPath := filepath.Join(projectDir, cfg.OutputDir+".zip")

			if err := compiler.ZipDir(outputDir, zipPath); err != nil {
				errChan <- fmt.Errorf("压缩失败 (%s环境): %w", env, err)
				return
			}

			// 上传 ZIP 文件
			remotePath := filepath.Join(cfg.RemoteBasePath, cfg.OutputDir)
			// 统一使用 Unix 风格路径
			remotePath = strings.ReplaceAll(remotePath, "\\", "/")

			// 确定是否仅上传
			finalUploadOnly := uploadOnly || cfg.UploadOnly

			if finalUploadOnly {
				// 仅上传文件
				if err := upload.UploadZipOnly(
					cfg.Server,
					cfg.Username,
					cfg.Password,
					zipPath,
					remotePath,
				); err != nil {
					errChan <- fmt.Errorf("上传失败 %s (%s环境): %w", cfg.OutputDir, env, err)
					return
				}
				utils.SafePrintf("上传成功: %s (%s环境)\n", cfg.OutputDir, env)
			} else {
				// 上传并解压
				if err := upload.UploadAndExtractZip(
					cfg.Server,
					cfg.Username,
					cfg.Password,
					zipPath,
					remotePath,
				); err != nil {
					errChan <- fmt.Errorf("上传失败 %s (%s环境): %w", cfg.OutputDir, env, err)
					return
				}
				utils.SafePrintf("上传成功: %s (%s环境)\n", cfg.OutputDir, env)
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
