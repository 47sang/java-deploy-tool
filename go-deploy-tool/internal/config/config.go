package config

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/BurntSushi/toml"
)

// DeployConfig 部署配置结构
type DeployConfig struct {
	// 服务器地址
	Server string `toml:"server"`
	// 用户名
	Username string `toml:"username"`
	// 密码
	Password string `toml:"password"`
	// Java 路径
	JavaPath string `toml:"java_path"`
	// 远程基础路径
	RemoteBasePath string `toml:"remote_base_path"`
	// JAR 文件 (可以是字符串或字符串数组)
	JarFiles interface{} `toml:"jar_files"`
	// Vue 打包执行命令脚本
	Scripts string `toml:"scripts"`
	// Vue 编译产物输出目录
	OutputDir string `toml:"output_dir"`
	// 是否仅上传文件，不执行命令 (可选，默认为false)
	UploadOnly bool `toml:"upload_only"`
}

// Environments 环境配置集合
type Environments struct {
	Environments map[string]DeployConfig `toml:"environments"`
}

// GetJarFilesList 获取 JAR 文件列表
func (c *DeployConfig) GetJarFilesList() []string {
	switch v := c.JarFiles.(type) {
	case string:
		return []string{v}
	case []interface{}:
		result := make([]string, 0, len(v))
		for _, item := range v {
			if s, ok := item.(string); ok {
				result = append(result, s)
			}
		}
		return result
	default:
		return nil
	}
}

// FromFile 从配置文件读取指定环境的配置
func FromFile(configPath, environment string) (*DeployConfig, error) {
	content, err := os.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("无法读取配置文件: %v", err)
	}

	var envs Environments
	if _, err := toml.Decode(string(content), &envs); err != nil {
		return nil, fmt.Errorf("解析配置文件失败: %v", err)
	}

	config, ok := envs.Environments[environment]
	if !ok {
		return nil, fmt.Errorf("环境 '%s' 未在配置文件中找到", environment)
	}

	return &config, nil
}

// CreateSpringBootConfig 创建示例配置文件
func CreateSpringBootConfig(path string) error {
	config := Environments{
		Environments: map[string]DeployConfig{
			"dev": {
				Server:         "192.168.31.60:22",
				Username:       "root",
				Password:       "lykj",
				JavaPath:       "/opt/soft/zulu11/bin/java",
				RemoteBasePath: "/opt/xinxuan1v1",
				JarFiles:       []string{"admin.jar", "client.jar", "websocket.jar"},
				Scripts:        "prod:test",
				OutputDir:      "dist-test",
				UploadOnly:     false,
			},
			"test": {
				Server:         "test-server:22",
				Username:       "test-user",
				Password:       "test-password",
				JavaPath:       "/usr/bin/java",
				RemoteBasePath: "/opt/test/apps",
				JarFiles:       []string{"admin.jar", "client.jar", "websocket.jar"},
				Scripts:        "prod:test",
				OutputDir:      "dist-test",
				UploadOnly:     false,
			},
			"prod": {
				Server:         "prod-server:22",
				Username:       "prod-user",
				Password:       "prod-password",
				JavaPath:       "/usr/java/latest/bin/java",
				RemoteBasePath: "/opt/prod/apps",
				JarFiles:       []string{"admin.jar", "client.jar", "websocket.jar"},
				Scripts:        "prod",
				OutputDir:      "dist",
				UploadOnly:     false,
			},
		},
	}

	// 确保目录存在
	dir := filepath.Dir(path)
	if dir != "" && dir != "." {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("创建配置目录失败: %v", err)
		}
	}

	file, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("创建配置文件失败: %v", err)
	}
	defer file.Close()

	encoder := toml.NewEncoder(file)
	if err := encoder.Encode(config); err != nil {
		return fmt.Errorf("写入配置文件失败: %v", err)
	}

	return nil
}
