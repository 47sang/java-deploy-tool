# 项目部署脚本

- 支持将编译后的程序放到springboot项目目录下，再执行命令,会从当前项目目录下读取配置文件，并根据配置文件中的环境变量，将对应的jar包上传到远程服务器，并运行jar包

- 支持将vue项目编译打包压缩zip发送到服务器,并自动解压部署

## 编译

```bash
cargo build --release
```
## 创建配置文件

```bash
deploy-tool --init-config
```

- 配置环境说明
```toml
[environments.test]
# 远程服务器地址和端口
server = "test-server:22"
# 远程服务器用户名
username = "test-user"
# 远程服务器密码
password = "test-password"
# 远程服务器java程序路径
java_path = "/usr/bin/java"
# 远程服务器jar包部署的目录路径
remote_base_path = "/opt/test/apps"
# 当前项目中的jar包文件名,如果单项目则只有一个,多模块项目则有多个,要保持数组类型
jar_files = [
    "admin.jar",
    "client.jar",
    "websocket.jar",
]
# npm run 后面跟随的后缀命令
scripts = "prod:test"
# 这个编译产物的输出文件夹名称
output_dir = "dist-test"
```


然后配置系统中mvn到系统path路径,不然找不到mvn命令

# vue项目多环境部署
```bash
deploy-tool -v dev,prod
```

# springboot项目多环境部署
```bash
deploy-tool -e dev,prod
```