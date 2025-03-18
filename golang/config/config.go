package config

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/BurntSushi/toml"
)

// DeployConfig 包含部署配置的结构体
type DeployConfig struct {
	// 服务器地址
	Server string `toml:"server"`
	// 用户名
	Username string `toml:"username"`
	// 密码
	Password string `toml:"password"`
	// Java路径
	JavaPath string `toml:"java_path"`
	// 远程基础路径
	RemoteBasePath string `toml:"remote_base_path"`
	// Jar文件列表
	JarFiles []string `toml:"jar_files"`
	// Vue打包执行命令脚本
	Scripts string `toml:"scripts"`
	// Vue编译产物输出目录
	OutputDir string `toml:"output_dir"`
}

// Environments 包含多环境配置
type Environments struct {
	Environments map[string]DeployConfig `toml:"environments"`
}

// CreateSpringbootConfig 创建示例配置文件
func CreateSpringbootConfig(path string) error {
	environments := make(map[string]DeployConfig)

	// 开发环境配置
	environments["dev"] = DeployConfig{
		Server:         "192.168.31.60:22",
		Username:       "root",
		Password:       "lykj",
		JavaPath:       "/opt/soft/zulu11/bin/java",
		RemoteBasePath: "/opt/xinxuan1v1",
		JarFiles:       []string{"admin.jar", "client.jar", "websocket.jar"},
		Scripts:        "prod:test",
		OutputDir:      "dist-test",
	}

	// 测试环境配置
	environments["test"] = DeployConfig{
		Server:         "test-server:22",
		Username:       "test-user",
		Password:       "test-password",
		JavaPath:       "/usr/bin/java",
		RemoteBasePath: "/opt/test/apps",
		JarFiles:       []string{"admin.jar", "client.jar", "websocket.jar"},
		Scripts:        "prod:test",
		OutputDir:      "dist-test",
	}

	// 生产环境配置
	environments["prod"] = DeployConfig{
		Server:         "prod-server:22",
		Username:       "prod-user",
		Password:       "prod-password",
		JavaPath:       "/usr/java/latest/bin/java",
		RemoteBasePath: "/opt/prod/apps",
		JarFiles:       []string{"admin.jar", "client.jar", "websocket.jar"},
		Scripts:        "prod",
		OutputDir:      "dist",
	}

	config := Environments{Environments: environments}

	// 确保父目录存在
	if dir := filepath.Dir(path); dir != "." {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("创建配置目录失败: %v", err)
		}
	}

	// 打开文件并写入配置
	file, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("创建配置文件失败: %v", err)
	}
	defer file.Close()

	encoder := toml.NewEncoder(file)
	if err := encoder.Encode(config); err != nil {
		return fmt.Errorf("编码配置文件失败: %v", err)
	}

	return nil
}

// FromFile 从文件中加载特定环境的配置
func FromFile(configPath, environment string) (*DeployConfig, error) {
	// 读取配置文件内容
	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("无法读取配置文件: %v", err)
	}

	// 解析TOML格式
	var environments Environments
	if err := toml.Unmarshal(data, &environments); err != nil {
		return nil, fmt.Errorf("解析配置文件失败: %v", err)
	}

	// 获取指定环境的配置
	config, exists := environments.Environments[environment]
	if !exists {
		return nil, fmt.Errorf("环境 '%s' 未在配置文件中找到", environment)
	}

	return &config, nil
} 