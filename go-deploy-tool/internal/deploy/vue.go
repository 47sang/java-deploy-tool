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

	"github.com/vbauerster/mpb/v8"
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

	// 进度容器：一次部署共享一个 mpb 容器，每个上传 goroutine 注册一条独立进度条，
	// 由容器统一刷新、各占一行（对标 Rust 版 MultiProgress）。结果性日志推迟到容器
	// 收尾后打印，避免与进度条渲染交错。
	p := mpb.New(mpb.WithWidth(64))
	sink := newMPBSink(p)

	// vueResult 记录单个 Vue 部署任务的最终结果，供容器收尾后统一打印。
	type vueResult struct {
		outputDir string
		env       string
		err       error
	}

	// 使用 WaitGroup 等待所有部署任务完成
	var wg sync.WaitGroup
	// resultChan 缓冲设为任务总数：每个 goroutine 恰好发送 1 个结果后即 return，
	// 缓冲永远够用，既不会阻塞 goroutine 导致 wg.Wait() 死锁，也不会丢失结果。
	resultChan := make(chan vueResult, len(tasks))

	for _, t := range tasks {
		wg.Add(1)
		go func(t vueTask) {
			defer wg.Done()
			env, cfg := t.env, t.cfg

			// 构建 Vue 项目
			if err := compiler.BuildVueProject(ctx, projectDir, cfg.Scripts); err != nil {
				resultChan <- vueResult{cfg.OutputDir, env, fmt.Errorf("构建Vue项目失败 (%s环境): %w", env, err)}
				return
			}

			// 压缩产出目录
			outputDir := filepath.Join(projectDir, cfg.OutputDir)
			zipPath := filepath.Join(projectDir, cfg.OutputDir+".zip")

			if err := compiler.ZipDir(outputDir, zipPath); err != nil {
				resultChan <- vueResult{cfg.OutputDir, env, fmt.Errorf("压缩失败 (%s环境): %w", env, err)}
				return
			}

			// 上传 ZIP 文件
			remotePath := filepath.Join(cfg.RemoteBasePath, cfg.OutputDir)
			// 统一使用 Unix 风格路径
			remotePath = strings.ReplaceAll(remotePath, "\\", "/")

			// label 作为进度条行首标签，多环境时用 "输出目录(环境)" 区分
			label := fmt.Sprintf("%s(%s)", cfg.OutputDir, env)

			// 确定是否仅上传
			finalUploadOnly := uploadOnly || cfg.UploadOnly

			var err error
			if finalUploadOnly {
				// 仅上传文件
				err = upload.UploadZipOnly(
					cfg.Server,
					cfg.Username,
					cfg.Password,
					zipPath,
					remotePath,
					label,
					sink,
				)
			} else {
				// 上传并解压
				err = upload.UploadAndExtractZip(
					cfg.Server,
					cfg.Username,
					cfg.Password,
					zipPath,
					remotePath,
					label,
					sink,
				)
			}
			resultChan <- vueResult{cfg.OutputDir, env, err}
		}(t)
	}

	// 等待所有 goroutine 完成，再让进度容器收尾刷新
	wg.Wait()
	p.Wait()
	close(resultChan)

	// 容器收尾后统一打印结果
	var errs []error
	for r := range resultChan {
		if r.err != nil {
			e := fmt.Errorf("上传失败 %s (%s环境): %w", r.outputDir, r.env, r.err)
			errs = append(errs, e)
			utils.PrintError("%v", e)
		} else {
			utils.PrintSuccess("上传成功: %s (%s环境)", r.outputDir, r.env)
		}
	}

	if len(errs) > 0 {
		return fmt.Errorf("部署过程中发生 %d 个错误", len(errs))
	}

	return nil
}
