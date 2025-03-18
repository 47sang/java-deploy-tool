# Java 和 Vue 项目部署工具（Go版本）

一个用Go语言实现的部署工具，用于简化Java和Vue项目的部署过程。支持多环境部署和多模块部署。

## 功能特点

- 自动构建Java项目（使用Maven）
- 自动构建Vue项目
- 上传编译产物到目标服务器
- 支持多环境部署（如开发环境、测试环境、生产环境）
- 支持Java项目的多模块部署
- 远程启动和管理Java应用
- 并行处理多个部署任务，提高效率

## 安装

### 直接下载二进制文件

从[Releases](https://github.com/user/deploy-tool/releases)页面下载对应操作系统的二进制文件。

### 从源码构建

```bash
git clone https://github.com/user/deploy-tool.git
cd deploy-tool/golang
go build -o deploy-tool
```

## 使用方法

### 初始化配置文件

```bash
./deploy-tool --init-config
```

这将创建一个默认的 `deploy.toml` 配置文件，您需要修改其中的配置参数。

### 配置文件示例

```toml
[environments.dev]
server = "192.168.31.60:22"
username = "root"
password = "lykj"
java_path = "/opt/soft/zulu11/bin/java"
remote_base_path = "/opt/xinxuan1v1"
jar_files = [
    "admin.jar",
    "client.jar",
    "websocket.jar",
]
scripts = "prod:test"
output_dir = "dist-test"

[environments.test]
server = "test-server:22"
username = "test-user"
password = "test-password"
java_path = "/usr/bin/java"
remote_base_path = "/opt/test/apps"
jar_files = [
    "admin.jar",
    "client.jar",
    "websocket.jar",
]
scripts = "prod:test"
output_dir = "dist-test"

[environments.prod]
server = "prod-server:22"
username = "prod-user"
password = "prod-password"
java_path = "/usr/java/latest/bin/java"
remote_base_path = "/opt/prod/apps"
jar_files = [
    "admin.jar",
    "client.jar",
    "websocket.jar",
]
scripts = "prod"
output_dir = "dist"
```

### 部署后端服务

```bash
# 部署到单个环境
./deploy-tool --env dev

# 部署到多个环境
./deploy-tool --env dev,prod

# 部署特定模块
./deploy-tool --env dev --model admin,client

# 简短命令形式
./deploy-tool -e dev -m admin,client
```

### 部署前端服务

```bash
# 部署到单个环境
./deploy-tool --vue dev

# 部署到多个环境
./deploy-tool --vue dev,prod

# 简短命令形式
./deploy-tool -v dev,prod
```

### 部署前后端服务

```bash
./deploy-tool --env dev --vue dev

# 简短命令形式
./deploy-tool -e dev -v dev
```

### 指定项目根目录

```bash
./deploy-tool --env dev --project-dir /path/to/your/project

# 简短命令形式
./deploy-tool -e dev -p /path/to/your/project
```

## 命令行参数

- `--env, -e`: 部署后端服务环境，多个环境用逗号分隔（例如: dev,prod）
- `--vue, -v`: 部署web端环境，多个环境用逗号分隔（例如: dev,prod）
- `--model, -m`: 部署jar模块，多个模块用逗号分隔（例如: admin,client,websocket）
- `--init-config`: 创建示例配置文件
- `--project-dir, -p`: 指定项目根目录路径（默认为当前目录）

## 许可证

MIT 