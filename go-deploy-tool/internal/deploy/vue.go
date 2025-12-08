package deploy

import (
	"fmt"
	"path/filepath"
	"strings"
	"sync"

	"deploy-tool/internal/compiler"
	"deploy-tool/internal/config"
	"deploy-tool/internal/upload"
)

// DeployVueProject 部署 Vue 项目
func DeployVueProject(projectDir, configPath string, environments []string, uploadOnly bool) error {
	// 使用 WaitGroup 等待所有部署任务完成
	var wg sync.WaitGroup
	errChan := make(chan error, 100)

	for _, env := range environments {
		cfg, err := config.FromFile(configPath, env)
		if err != nil {
			fmt.Printf("加载%s环境配置失败: %v\n", env, err)
			continue
		}

		wg.Add(1)
		go func(env string, cfg *config.DeployConfig) {
			defer wg.Done()

			// 构建 Vue 项目
			if err := compiler.BuildVueProject(projectDir, cfg.Scripts); err != nil {
				errChan <- fmt.Errorf("构建Vue项目失败 (%s环境): %v", env, err)
				return
			}

			// 压缩产出目录
			outputDir := filepath.Join(projectDir, cfg.OutputDir)
			zipPath := filepath.Join(projectDir, cfg.OutputDir+".zip")

			if err := compiler.ZipDir(outputDir, zipPath); err != nil {
				errChan <- fmt.Errorf("压缩失败 (%s环境): %v", env, err)
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
					errChan <- fmt.Errorf("上传失败 %s (%s环境): %v", cfg.OutputDir, env, err)
					return
				}
				fmt.Printf("上传成功: %s (%s环境)\n", cfg.OutputDir, env)
			} else {
				// 上传并解压
				if err := upload.UploadAndExtractZip(
					cfg.Server,
					cfg.Username,
					cfg.Password,
					zipPath,
					remotePath,
				); err != nil {
					errChan <- fmt.Errorf("上传失败 %s (%s环境): %v", cfg.OutputDir, env, err)
					return
				}
				fmt.Printf("上传成功: %s (%s环境)\n", cfg.OutputDir, env)
			}
		}(env, cfg)
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
