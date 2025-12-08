package compiler

import (
	"archive/zip"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// ZipDir 将目录打包成 zip 文件
func ZipDir(srcDir, destZip string) error {
	// 确保源目录存在
	info, err := os.Stat(srcDir)
	if err != nil {
		return fmt.Errorf("源目录不存在: %v", err)
	}
	if !info.IsDir() {
		return fmt.Errorf("源路径不是一个目录: %s", srcDir)
	}

	// 创建 zip 文件
	zipFile, err := os.Create(destZip)
	if err != nil {
		return fmt.Errorf("创建zip文件失败: %v", err)
	}
	defer zipFile.Close()

	zipWriter := zip.NewWriter(zipFile)
	defer zipWriter.Close()

	// 遍历目录
	return filepath.Walk(srcDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// 跳过源目录本身
		if path == srcDir {
			return nil
		}

		// 计算相对路径
		relPath, err := filepath.Rel(srcDir, path)
		if err != nil {
			return fmt.Errorf("计算相对路径失败: %v", err)
		}

		// 替换 Windows 路径分隔符为 ZIP 标准的 /
		zipPath := strings.ReplaceAll(relPath, string(os.PathSeparator), "/")

		if info.IsDir() {
			// 确保目录路径以 / 结尾
			if !strings.HasSuffix(zipPath, "/") {
				zipPath += "/"
			}
			_, err := zipWriter.Create(zipPath)
			return err
		}

		// 创建文件头
		header, err := zip.FileInfoHeader(info)
		if err != nil {
			return fmt.Errorf("创建文件头失败: %v", err)
		}
		header.Name = zipPath
		header.Method = zip.Deflate

		// 写入文件
		writer, err := zipWriter.CreateHeader(header)
		if err != nil {
			return fmt.Errorf("添加文件到ZIP失败: %v", err)
		}

		file, err := os.Open(path)
		if err != nil {
			return fmt.Errorf("打开文件失败: %v", err)
		}
		defer file.Close()

		_, err = io.Copy(writer, file)
		if err != nil {
			return fmt.Errorf("写入ZIP失败: %v", err)
		}

		return nil
	})
}
