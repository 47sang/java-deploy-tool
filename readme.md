# 项目部署脚本

- 将编译后的程序放到springboot项目目录下，再执行命令,会从当前项目目录下读取配置文件，并根据配置文件中的环境变量，将对应的jar包上传到远程服务器，并运行jar包

## 编译

```bash
cargo run --release
```
## 创建配置文件

```bash
java-deploy-tool --init-config
```

# 开发环境（默认）
```bash
java-deploy-tool -e dev
```

# 测试环境
```bash
java-deploy-tool -e test
```

# 生产环境
```bash
java-deploy-tool -e prod
```