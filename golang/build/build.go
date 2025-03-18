package build

import (
	"archive/zip"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// BuildJavaProject 构建Java项目
func BuildJavaProject(projectDir string) error {
	// 确保在项目目录中执行命令
	cmd := exec.Command("cmd", "/c", "mvn", "clean", "package")
	cmd.Dir = projectDir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	fmt.Println("开始执行Maven构建命令...")
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("构建失败: 请检查mvn是否配置在环境变量中\n%v", err)
	}

	fmt.Println("Java项目构建成功!")
	return nil
}

// BuildVueProject 构建Vue项目
func BuildVueProject(projectDir, scripts string) error {
	// 执行npm构建命令
	cmd := exec.Command("cmd", "/c", "npm", "run", scripts)
	cmd.Dir = projectDir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	fmt.Printf("开始执行Vue项目构建命令 'npm run %s'...\n", scripts)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("构建失败: 请检查npm是否配置在环境变量中\n%v", err)
	}

	fmt.Printf("%s环境下的Vue项目构建成功!\n", scripts)
	return nil
}

// ZipDirectory 打包目录成zip文件
func ZipDirectory(sourceDir, zipFilePath string) error {
	// 创建目标zip文件
	zipFile, err := os.Create(zipFilePath)
	if err != nil {
		return fmt.Errorf("创建ZIP文件失败: %v", err)
	}
	defer zipFile.Close()

	// 创建一个zip writer
	zipWriter := zip.NewWriter(zipFile)
	defer zipWriter.Close()

	// 检查源目录是否存在
	info, err := os.Stat(sourceDir)
	if err != nil {
		return fmt.Errorf("源目录不存在: %v", err)
	}
	if !info.IsDir() {
		return fmt.Errorf("源路径不是一个目录: %s", sourceDir)
	}

	// 遍历源目录
	err = filepath.Walk(sourceDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// 创建zip头信息
		header, err := zip.FileInfoHeader(info)
		if err != nil {
			return err
		}

		// 计算相对路径
		relPath, err := filepath.Rel(sourceDir, path)
		if err != nil {
			return err
		}

		// 跳过根目录
		if relPath == "." {
			return nil
		}

		// 设置头信息中的名称为相对路径，并使用正斜杠作为分隔符
		header.Name = strings.ReplaceAll(relPath, "\\", "/")

		// 如果是目录，添加尾随斜杠
		if info.IsDir() {
			header.Name += "/"
			// 添加空目录条目
			_, err = zipWriter.CreateHeader(header)
			return err
		}

		// 创建文件条目
		writer, err := zipWriter.CreateHeader(header)
		if err != nil {
			return err
		}

		// 如果是普通文件，复制内容
		file, err := os.Open(path)
		if err != nil {
			return err
		}
		defer file.Close()

		_, err = io.Copy(writer, file)
		return err
	})

	if err != nil {
		return fmt.Errorf("压缩目录时出错: %v", err)
	}

	fmt.Printf("压缩成功: %s -> %s\n", sourceDir, zipFilePath)
	return nil
} 