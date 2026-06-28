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

	// 进度容器：一次部署共享一个 mpb 容器，每个上传 goroutine 在其中注册一条独立进度条，
	// 由容器统一刷新、各占一行，互不交错（对标 Rust 版 MultiProgress）。过程中的状态全部
	// 反映在进度条上（文件名/百分比/速率/ETA），结果性日志统一推迟到容器收尾后打印，
	// 避免日志与进度条在同一终端流上互相打乱。
	p := mpb.New(mpb.WithWidth(64))
	sink := newMPBSink(p)

	// deployResult 记录单个部署任务的最终结果（成功或失败），供容器收尾后统一打印。
	type deployResult struct {
		jarName string
		env     string
		action  string // "上传" 或 "部署"
		err     error
	}

	var wg sync.WaitGroup
	// resultChan 缓冲设为任务总数：每个 goroutine 恰好发送 1 个结果后即 return，
	// 缓冲永远够用，既不会阻塞 goroutine 导致 wg.Wait() 死锁，也不会丢失结果。
	resultChan := make(chan deployResult, len(tasks))

	for _, t := range tasks {
		wg.Add(1)
		go func(t javaTask) {
			defer wg.Done()

			remotePath := filepath.Join(t.cfg.RemoteBasePath, t.jarName)
			// 统一使用 Unix 风格路径
			remotePath = strings.ReplaceAll(remotePath, "\\", "/")

			// label 作为进度条行首标签，多环境部署时用 "jar名(环境)" 区分，无歧义
			label := fmt.Sprintf("%s(%s)", t.jarName, t.env)

			// 确定是否仅上传
			finalUploadOnly := uploadOnly || t.cfg.UploadOnly

			var err error
			action := "部署"
			if finalUploadOnly {
				action = "上传"
				err = upload.UploadJarOnly(
					t.cfg.Server,
					t.cfg.Username,
					t.cfg.Password,
					t.jarPath,
					remotePath,
					label,
					sink,
				)
			} else {
				err = upload.UploadAndRunJar(
					t.cfg.Server,
					t.cfg.Username,
					t.cfg.Password,
					t.jarPath,
					remotePath,
					t.cfg.JavaPath,
					t.env,
					label,
					sink,
				)
			}
			resultChan <- deployResult{jarName: t.jarName, env: t.env, action: action, err: err}
		}(t)
	}

	// 等待所有上传 goroutine 完成（此时各进度条已完成或中止），再让进度容器收尾刷新
	wg.Wait()
	p.Wait()
	close(resultChan)

	// 容器收尾后统一打印每个任务的结果，避免日志与进度条渲染交错
	var errs []error
	for r := range resultChan {
		if r.err != nil {
			e := fmt.Errorf("%s失败 %s (%s环境): %w", r.action, r.jarName, r.env, r.err)
			errs = append(errs, e)
			utils.PrintError("%v", e)
		} else {
			utils.PrintSuccess("%s成功: %s (%s环境)", r.action, r.jarName, r.env)
		}
	}

	if len(errs) > 0 {
		return fmt.Errorf("部署过程中发生 %d 个错误", len(errs))
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
