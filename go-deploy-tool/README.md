# Deploy Tool (Go 版本)

一键部署 Java 和 Vue 项目的命令行工具，支持多环境部署、多模块部署，以及 JDK/Maven 自动发现功能。

## 功能特性

### 核心功能
- ✅ Java Spring Boot 项目的编译、打包、上传、部署
- ✅ Vue 项目的构建、打包、上传、部署
- ✅ 多环境并发部署（dev/test/prod）
- ✅ 多模块支持
- ✅ SSH/SCP 文件上传
- ✅ 远程服务管理（停止旧进程、启动新服务）
- ✅ 配置文件管理（deploy.toml）
- ✅ 命令行参数解析

### 新增功能

#### JDK 自动发现
- 自动检测系统中已安装的 JDK，即使未配置环境变量
- 检测路径包括：
  - 环境变量 `JAVA_HOME`、`PATH`
  - IntelliJ IDEA 捆绑的 JDK
  - JetBrains Toolbox 安装的 IDEA 捆绑 JDK
  - 系统常见安装路径
    - macOS: `/Library/Java/JavaVirtualMachines/`
    - Windows: `C:\Program Files\Java\`、`C:\Program Files\Zulu\`
    - Linux: `/usr/lib/jvm/`、`/usr/java/`
  - Homebrew/Scoop/Chocolatey 安装的 JDK
  - SDKMAN 安装的 JDK
- 优先级：环境变量 > PATH > 自动发现路径

#### Maven 自动发现
- 自动检测系统中已安装的 Maven，即使未配置环境变量
- 检测路径包括：
  - 环境变量 `MAVEN_HOME`、`M2_HOME`、`PATH`
  - IntelliJ IDEA 捆绑的 Maven
  - 系统常见安装路径
    - macOS: `/opt/homebrew/bin/mvn`、`/usr/local/bin/mvn`
    - Windows: `C:\Program Files\Apache\maven*`
    - Linux: `/usr/bin/mvn`、`/usr/local/bin/mvn`
  - SDKMAN 安装的 Maven
  - Maven Wrapper (mvnw)
- 优先级：环境变量 > PATH > 自动发现路径

## 安装

### 从源码编译

```bash
# 克隆项目
cd go-deploy-tool

# 下载依赖
go mod tidy

# 编译
go build -o deploy-tool ./cmd/main.go

# 跨平台编译
# Windows
GOOS=windows GOARCH=amd64 go build -o deploy-tool.exe ./cmd/main.go

# macOS (Intel)
GOOS=darwin GOARCH=amd64 go build -o deploy-tool-darwin ./cmd/main.go

# macOS (Apple Silicon)
GOOS=darwin GOARCH=arm64 go build -o deploy-tool-darwin-arm64 ./cmd/main.go

# Linux
GOOS=linux GOARCH=amd64 go build -o deploy-tool-linux ./cmd/main.go
```

## 使用方法

### 初始化配置文件

```bash
./deploy-tool --init-config
```

这将在当前目录创建 `deploy.toml` 示例配置文件。

### 部署 Java 项目

```bash
# 部署到 dev 环境
./deploy-tool -e dev

# 部署到多个环境
./deploy-tool -e dev,test,prod

# 指定模块部署
./deploy-tool -e dev -m admin,client

# 仅上传，不执行启动命令
./deploy-tool -e dev -u

# 指定项目目录
./deploy-tool -e dev -p /path/to/project
```

### 部署 Vue 项目

```bash
# 部署到 dev 环境
./deploy-tool -v dev

# 部署到多个环境
./deploy-tool -v dev,prod
```

### 同时部署 Java 和 Vue 项目

```bash
./deploy-tool -e dev,prod -v dev,prod
```

## 配置文件格式

配置文件使用 TOML 格式，示例：

```toml
[environments.dev]
server = "192.168.1.100:22"
username = "root"
password = "your-password"
java_path = "/opt/soft/zulu11/bin/java"
remote_base_path = "/opt/apps"
jar_files = ["admin.jar", "client.jar", "websocket.jar"]
scripts = "prod:test"
output_dir = "dist-test"
upload_only = false

[environments.prod]
server = "prod-server:22"
username = "prod-user"
password = "prod-password"
java_path = "/usr/java/latest/bin/java"
remote_base_path = "/opt/prod/apps"
jar_files = ["admin.jar", "client.jar", "websocket.jar"]
scripts = "prod"
output_dir = "dist"
upload_only = false
```

### 配置项说明

| 配置项 | 类型 | 说明 |
|--------|------|------|
| server | string | SSH 服务器地址和端口 |
| username | string | SSH 用户名 |
| password | string | SSH 密码 |
| java_path | string | 远程服务器上的 Java 路径 |
| remote_base_path | string | 远程服务器上的部署基础路径 |
| jar_files | string/array | JAR 文件名（单模块用字符串，多模块用数组） |
| scripts | string | Vue 项目的构建脚本名称 |
| output_dir | string | Vue 项目的输出目录 |
| upload_only | bool | 是否仅上传不执行启动命令 |

## 命令行参数

| 参数 | 短参数 | 说明 |
|------|--------|------|
| --env | -e | 部署后端服务环境（多个用逗号分隔） |
| --vue | -v | 部署 Web 端环境（多个用逗号分隔） |
| --model | -m | 部署 JAR 模块（多个用逗号分隔） |
| --project-dir | -p | 项目根目录路径（默认当前目录） |
| --upload-only | -u | 仅上传文件，不执行命令 |
| --init-config | - | 创建示例配置文件 |
| --version | - | 显示版本信息 |
| --help | -h | 显示帮助信息 |

## 项目结构

```
go-deploy-tool/
├── cmd/
│   └── main.go              # 命令行入口
├── internal/
│   ├── config/
│   │   └── config.go        # 配置文件管理
│   ├── discovery/
│   │   ├── jdk.go           # JDK 自动发现
│   │   └── maven.go         # Maven 自动发现
│   ├── compiler/
│   │   ├── java.go          # Java 项目编译
│   │   ├── vue.go           # Vue 项目编译
│   │   └── zip.go           # ZIP 压缩
│   ├── upload/
│   │   └── ssh.go           # SSH/SFTP 上传
│   └── deploy/
│       ├── java.go          # Java 项目部署
│       └── vue.go           # Vue 项目部署
├── pkg/
│   └── utils/
│       └── utils.go         # 通用工具函数
├── go.mod
├── go.sum
└── README.md
```

## 依赖库

- [spf13/cobra](https://github.com/spf13/cobra) - 命令行参数解析
- [BurntSushi/toml](https://github.com/BurntSushi/toml) - TOML 配置文件解析
- [golang.org/x/crypto/ssh](https://pkg.go.dev/golang.org/x/crypto/ssh) - SSH 连接
- [pkg/sftp](https://github.com/pkg/sftp) - SFTP 文件传输
- [schollz/progressbar](https://github.com/schollz/progressbar) - 进度条显示

## 与 Rust 版本的兼容性

- ✅ 命令行参数完全兼容
- ✅ 配置文件格式完全兼容
- ✅ 部署行为一致

## License

MIT License
