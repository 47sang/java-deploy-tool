use serde::{Deserialize, Serialize};
use std::collections::HashMap;
use std::fs;
use std::path::Path;

#[derive(Clone, Serialize, Deserialize)]
pub struct DeployConfig {
    /// 服务器地址
    pub server: String,
    /// 用户名
    pub username: String,
    /// 密码
    pub password: String,
    /// java路径
    pub java_path: String,
    /// 远程基础路径
    pub remote_base_path: String,
    /// jar文件
    pub jar_files: Vec<String>,
    /// vue打包执行命令脚本
    pub scripts: String,
    /// vue编译产物输出目录
    pub output_dir: String,
}

#[derive(Serialize, Deserialize)]
pub struct Environments {
    pub environments: HashMap<String, DeployConfig>,
}

impl DeployConfig {
    pub fn create_springboot_config(path: &str) -> Result<(), String> {
        let mut environments = HashMap::new();
        environments.insert(
            "dev".to_string(),
            DeployConfig {
                server: "192.168.31.60:22".to_string(),
                username: "root".to_string(),
                password: "lykj".to_string(),
                java_path: "/opt/soft/zulu11/bin/java".to_string(),
                remote_base_path: "/opt/xinxuan1v1".to_string(),
                jar_files: vec!["admin.jar", "client.jar", "websocket.jar"].iter().map(|s| s.to_string()).collect(),
                scripts: "prod:test".to_string(),
                output_dir: "dist-test".to_string(),
            },
        );
        environments.insert(
            "test".to_string(),
            DeployConfig {
                server: "test-server:22".to_string(),
                username: "test-user".to_string(),
                password: "test-password".to_string(),
                java_path: "/usr/bin/java".to_string(),
                remote_base_path: "/opt/test/apps".to_string(),
                jar_files: vec!["admin.jar", "client.jar", "websocket.jar"].iter().map(|s| s.to_string()).collect(),
                scripts: "prod:test".to_string(),
                output_dir: "dist-test".to_string(),
            },
        );
        environments.insert(
            "prod".to_string(),
            DeployConfig {
                server: "prod-server:22".to_string(),
                username: "prod-user".to_string(),
                password: "prod-password".to_string(),
                java_path: "/usr/java/latest/bin/java".to_string(),
                remote_base_path: "/opt/prod/apps".to_string(),
                jar_files: vec!["admin.jar", "client.jar", "websocket.jar"].iter().map(|s| s.to_string()).collect(),
                scripts: "prod".to_string(),
                output_dir: "dist".to_string(),
            },
        );

        let config = Environments { environments };
        let toml_string = toml::to_string_pretty(&config)
            .map_err(|e| format!("序列化配置失败: {}", e))?;

        if let Some(parent) = Path::new(path).parent() {
            fs::create_dir_all(parent)
                .map_err(|e| format!("创建配置目录失败: {}", e))?;
        }

        fs::write(path, toml_string)
            .map_err(|e| format!("写入配置文件失败: {}", e))?;

        Ok(())
    }

    pub fn from_file(config_path: &str, environment: &str) -> Result<Self, String> {
        let config_content = fs::read_to_string(config_path)
            .map_err(|e| format!("无法读取配置文件: {}", e))?;
        
        let environments: Environments = toml::from_str(&config_content)
            .map_err(|e| format!("解析配置文件失败: {}", e))?;
        
        environments
            .environments
            .get(environment)
            .cloned()
            .ok_or_else(|| format!("环境 '{}' 未在配置文件中找到", environment))
    }
} 