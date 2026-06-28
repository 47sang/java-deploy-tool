package compiler

import (
	"archive/zip"
	"io"
	"os"
	"path/filepath"
	"testing"
)

// TestZipDir 真实打包一个临时目录，再读回 zip 验证文件内容与目录结构是否完整保留。
func TestZipDir(t *testing.T) {
	t.Parallel()
	srcDir := t.TempDir()
	files := map[string]string{
		"a.txt":            "hello",
		"sub/b.txt":        "world",
		"sub/deeper/c.txt": "nested",
	}
	for path, content := range files {
		full := filepath.Join(srcDir, filepath.FromSlash(path))
		if err := os.MkdirAll(filepath.Dir(full), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte(content), 0644); err != nil {
			t.Fatal(err)
		}
	}

	destZip := filepath.Join(t.TempDir(), "out.zip")
	if err := ZipDir(srcDir, destZip); err != nil {
		t.Fatalf("ZipDir 失败: %v", err)
	}

	r, err := zip.OpenReader(destZip)
	if err != nil {
		t.Fatalf("打开 zip 失败: %v", err)
	}
	defer r.Close()

	got := map[string]string{}
	for _, f := range r.File {
		if f.FileInfo().IsDir() {
			continue
		}
		rc, err := f.Open()
		if err != nil {
			t.Fatal(err)
		}
		b, err := io.ReadAll(rc)
		rc.Close()
		if err != nil {
			t.Fatal(err)
		}
		got[f.Name] = string(b)
	}

	for name, content := range files {
		if got[name] != content {
			t.Errorf("zip 内 %q 内容 = %q, want %q", name, got[name], content)
		}
	}
}

// TestZipDir_SourceNotExist 源目录不存在应返回错误。
func TestZipDir_SourceNotExist(t *testing.T) {
	t.Parallel()
	err := ZipDir(filepath.Join(t.TempDir(), "nope"), filepath.Join(t.TempDir(), "x.zip"))
	if err == nil {
		t.Fatal("源目录不存在应返回错误")
	}
}

// TestZipDir_SourceNotDir 源路径是文件而非目录应返回错误。
func TestZipDir_SourceNotDir(t *testing.T) {
	t.Parallel()
	f := filepath.Join(t.TempDir(), "file.txt")
	if err := os.WriteFile(f, []byte("x"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := ZipDir(f, filepath.Join(t.TempDir(), "x.zip")); err == nil {
		t.Fatal("源路径非目录应返回错误")
	}
}
