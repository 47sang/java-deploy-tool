package config

import (
	"os"
	"path/filepath"
	"testing"
)

// TestGetJarFilesList 覆盖 jar_files 的字符串、数组、混入非字符串、非法类型、nil 等形态。
func TestGetJarFilesList(t *testing.T) {
	t.Parallel()
	t.Run("字符串", func(t *testing.T) {
		t.Parallel()
		c := &DeployConfig{JarFiles: "admin.jar"}
		got := c.GetJarFilesList()
		if len(got) != 1 || got[0] != "admin.jar" {
			t.Errorf("got %v, want [admin.jar]", got)
		}
	})
	t.Run("数组", func(t *testing.T) {
		t.Parallel()
		c := &DeployConfig{JarFiles: []interface{}{"a.jar", "b.jar", "c.jar"}}
		got := c.GetJarFilesList()
		if len(got) != 3 || got[2] != "c.jar" {
			t.Errorf("got %v, want 3 项", got)
		}
	})
	t.Run("混入非字符串元素应跳过", func(t *testing.T) {
		t.Parallel()
		c := &DeployConfig{JarFiles: []interface{}{"a.jar", 123, true, "b.jar"}}
		got := c.GetJarFilesList()
		if len(got) != 2 {
			t.Errorf("非字符串元素应被跳过，got %v", got)
		}
	})
	t.Run("非法类型返回 nil", func(t *testing.T) {
		t.Parallel()
		c := &DeployConfig{JarFiles: 12345}
		if got := c.GetJarFilesList(); got != nil {
			t.Errorf("非法类型应返回 nil，got %v", got)
		}
	})
	t.Run("nil 返回 nil", func(t *testing.T) {
		t.Parallel()
		c := &DeployConfig{}
		if got := c.GetJarFilesList(); got != nil {
			t.Errorf("nil 应返回 nil，got %v", got)
		}
	})
}

// TestFromFile 验证 TOML 解析与字段映射（含数组形态的 jar_files）。
func TestFromFile(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := filepath.Join(dir, "deploy.toml")
	content := `
[environments.dev]
server = "1.2.3.4:22"
username = "root"
password = "pw"
java_path = "/usr/bin/java"
remote_base_path = "/opt/app"
jar_files = ["admin.jar", "client.jar"]
scripts = "prod"
output_dir = "dist"
upload_only = false
`
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
	cfg, err := FromFile(path, "dev")
	if err != nil {
		t.Fatalf("FromFile 失败: %v", err)
	}
	if cfg.Server != "1.2.3.4:22" {
		t.Errorf("Server = %q", cfg.Server)
	}
	if cfg.UploadOnly != false {
		t.Errorf("UploadOnly = %v, want false", cfg.UploadOnly)
	}
	jars := cfg.GetJarFilesList()
	if len(jars) != 2 || jars[0] != "admin.jar" || jars[1] != "client.jar" {
		t.Errorf("jar_files = %v", jars)
	}
}

func TestFromFile_FileNotFound(t *testing.T) {
	t.Parallel()
	if _, err := FromFile(filepath.Join(t.TempDir(), "nope.toml"), "dev"); err == nil {
		t.Fatal("文件不存在应返回错误")
	}
}

func TestFromFile_EnvMissing(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "deploy.toml")
	os.WriteFile(path, []byte("[environments.dev]\nserver=\"x\"\n"), 0644)
	if _, err := FromFile(path, "prod"); err == nil {
		t.Fatal("环境不存在应返回错误")
	}
}

// TestCreateSpringBootConfig 验证示例配置能被生成（含嵌套目录创建）并回读解析。
func TestCreateSpringBootConfig(t *testing.T) {
	t.Parallel()
	// 嵌套目录，验证 MkdirAll
	path := filepath.Join(t.TempDir(), "sub", "deploy.toml")
	if err := CreateSpringBootConfig(path); err != nil {
		t.Fatalf("CreateSpringBootConfig 失败: %v", err)
	}
	cfg, err := FromFile(path, "dev")
	if err != nil {
		t.Fatalf("生成的配置回读失败: %v", err)
	}
	if cfg.Server == "" {
		t.Error("生成的 dev 环境应有 server")
	}
	if len(cfg.GetJarFilesList()) == 0 {
		t.Error("生成的 dev 环境应有 jar_files")
	}
}
