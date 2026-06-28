# Rust 原版 vs Go 重写版 部署工具差异分析报告

> **更新说明**：本报告已与 `go-deploy-tool`（`go` 分支）当前代码状态同步。原先标记的若干高风险缺陷与功能缺失已修复，相关章节以 ✅ 标注并注明实际修复方式；图例 🟢 完全对等 / 🟡 基本对等 / 🔴 存在差异保持不变。

## 1. 总体结论

Go 重写版在核心功能上**与 Rust 原版基本对等**，完整还原了 CLI 解析、配置管理、Java/Vue 编译链路、ZIP 打包、SFTP 上传、进程管理等全部主流程；同时通过拆分 `internal/discovery` 与 `internal/timeout` 包、引入错误聚合与输出互斥机制、扩展 JDK/Maven 搜索路径、统一上传重试等方式，在可维护性、环境适应性与可靠性上有所增强。

初版报告中标记的精细化进度展示、文件完整性校验、JAVA_HOME 注入、UploadZipOnly 后缀、KillProcess 错误传播、并发输出交错、错误链、统一重试等问题**已在 `go` 分支全部修复**（详见 §8.2）。本轮另完成错误链 `%v→%w` 统一、`SSHClient.session` 死字段清理、`MkdirAll` 静默吞错修复，并将 pom.xml 降级改为报错、`errChan` 缓冲改为任务总数（详见 §8.3）。当前仅余 SFTP 备份两步操作的非原子性等少量低风险点供后续评估。

---

## 2. 架构与代码组织差异

| 维度 | Rust 原版 | Go 重写版 |
|------|-----------|-----------|
| 文件结构 | `src/` 下 5 个文件（`compiler.rs`、`deploy.rs`、`upload.rs`、`main.rs`、`config.rs`），单文件大模块风格 | `internal/` 下 6 个包（`config`、`compiler`、`deploy`、`discovery`、`upload`、`timeout`）+ `pkg/utils`，职责进一步拆分 |
| 职责划分 | JDK/Maven 探测、版本检测、构建执行、ZIP 打包、部署编排、SSH 通信均集中在 `compiler.rs`（约 443 行）与 `deploy.rs` 中 | `discovery` 包独立承担 JDK/Maven 探测（`jdk.go` 273 行、`maven.go` 277 行），`compiler` 仅编排构建流程，`deploy` 专注部署并发，`upload` 专注 SFTP 通信，`timeout` 集中管理各环节超时 |
| 工具函数 | 散布在 `compiler.rs` 与 `main.rs` 中，无独立工具模块 | 集中到 `pkg/utils`，提供 `FormatBytes`、`BytesToMB`、`FormatDuration`、`FormatExecutionTime`、`PrintStep`、`PrintSuccess`、`SafePrintf` 等可复用函数 |
| 配置模型 | `config.rs` 中定义 `DeployConfig`，字段全蛇形命名，直接映射 TOML 键 | `internal/config` 中定义 `DeployConfig`，PascalCase 字段通过 `toml` struct tag 映射到 snake_case TOML 键 |

**关键差异解读：**

Rust 版将全部 Java 编译链路（JDK 探测、Maven 探测、pom.xml 解析、构建执行）耦合在 `compiler.rs` 单个文件中，约 443 行，阅读和维护成本较高。Go 版将其拆分为独立的 `discovery` 包（`jdk.go` + `maven.go`），`compiler` 包只负责调用发现结果并执行构建命令，职责边界清晰，单元测试可针对 `discovery` 包独立进行。但 Rust 版的单文件风格在项目规模较小时反而减少了跨文件跳转的认知负担。

---

## 3. 功能对等性总表

| 序号 | 功能维度 | 对等程度 | 核心差异一句话 |
|------|----------|----------|----------------|
| 1 | 入口与整体编排 | 🟡 基本对等 | CLI 框架从 `clap` 切换为 `cobra`，并发模型从 `thread::spawn` 改为 `sync.WaitGroup` + error channel，上传层剥离 MultiProgress 进度条参数 |
| 2 | 配置模型 | 🟢 完全对等 | Go 版通过 `GetJarFilesList()` 方法封装了 `jar_files` 多态归一化逻辑，消除了 Rust 版 `deploy.rs` 中的代码重复；其余字段语义完全一致 |
| 3 | Java 编译链路 | 🟢 完全对等 | Go 版增强了 Maven/JDK 发现能力（Maven Wrapper、MAVEN_HOME/M2_HOME、更广搜索路径、pom.xml 失败兜底）；JAVA_HOME 注入已通过 `overrideEnv` 修复，无重复 key；Java 8 的 `1.8` XML 变体 Go 版反而更完善 |
| 4 | Vue 编译链路 | 🟢 完全对等 | Go 版新增 Node.js 预检函数 `CheckNodeJS` 与并发错误收集机制；`UploadZipOnly` 的 `.zip` 后缀去除逻辑已修复，与 Rust 版一致 |
| 5 | 打包 zip | 🟡 基本对等 | Go 版使用流式 `io.Copy` 内存效率更高，但 `filepath.Walk` 遇到错误直接中止遍历，无 Rust 版 `filter_map(Result::ok)` 的跳过容错机制 |
| 6 | 部署逻辑 | 🟡 基本对等 | Go 版改用 SFTP 原生操作实现备份与移动，并发错误可聚合返回；文件读取完整性校验与 KillProcess 错误传播均已补齐 |
| 7 | 上传与 SSH | 🟡 基本对等 | Go 版基于 SFTP 协议，提供连接超时、自动目录创建、SSHClient 结构体封装，progressbar 内置速率/ETA 显示；Rust 版基于 SCP 协议，提供自定义 ProgressWriter（含当前/平均速度）与 MultiProgress 并发进度条 |
| 8 | 工具函数与错误处理风格 | 🟡 基本对等 | Go 版工具函数更集中封装，带 emoji 输出与并发安全锁；关键路径已改用 `%w` 保留错误链，Rust 版仍以 `anyhow` 链式 `.context()` 的便捷性见长 |

> **图例**：🟢 完全对等  🟡 基本对等  🔴 存在差异

---

## 4. Go 相对 Rust 的增强/改进

### 4.1 架构与代码组织

- **独立的 `discovery` 包**：将 JDK 与 Maven 发现逻辑拆分为 `jdk.go`（273 行）与 `maven.go`（277 行），`compiler` 包仅负责构建编排，职责分离清晰。
- **集中式工具模块 `pkg/utils`**：提供 `FormatBytes`、`BytesToMB`、`FormatDuration`、`FormatExecutionTime`、`PrintStep`、`PrintSuccess`、`PrintWarning`、`PrintError`、`PrintInfo`、`SafePrintf`、`SafePrintln` 等带 emoji 前缀的统一输出封装；`Print*` 系列与 `SafePrintf` 共享一把互斥锁，保证并发输出原子性。
- **`GetJarFilesList()` 方法**：将 `jar_files` 字段的 `string` / `[]interface{}` 多态归一化逻辑封装为可复用方法，消除了 Rust 版 `deploy.rs` 中重复的模式匹配代码。
- **流式 TOML 写入**：创建配置文件时使用 `toml.NewEncoder(file).Encode(config)` 流式编码，大配置场景内存更友好。

### 4.2 环境探测与容错

- **Maven Wrapper (mvnw/mvnw.cmd) 自动检测**：第 4 步优先检查当前目录的 Maven Wrapper，失败时才回退到全局 Maven。
- **MAVEN_HOME / M2_HOME 环境变量支持**：兼容旧版 Maven 安装方式，Rust 版仅检查 PATH。
- ~~**pom.xml 读取失败降级**~~ **✅ 已改为报错**：原实现读取 `pom.xml` 失败时打印警告并调用 `FindAnyJDK()` 兜底；现已改为直接返回错误（fail-fast），与 Rust 版 `bail!` 行为对齐（见 §7 风险 5）。
- **更丰富的搜索路径**：JDK 搜索覆盖 JetBrains Toolbox、Snap（Linux）、SDKMAN、Jabba、Zulu、Temurin、Adoptium、Amazon Corretto、Microsoft Build 等；Maven 搜索覆盖 JetBrains Toolbox、Snap、SDKMAN、Scoop、Chocolatey、Apache 官方路径等。每 OS 约 15-20 个路径（Rust 约 3-5 个）。

### 4.3 并发与错误处理

- **错误聚合机制**：`deploy` 层使用 `sync.WaitGroup` + 带缓冲 `errorChan`（容量 100），所有子任务错误被收集后统一返回 `fmt.Errorf("部署过程中发生 %d 个错误")`，调用方可感知部署结果；Rust 版子线程错误仅 `eprintln!` 输出，主函数始终返回 `Ok(())`。
- **并发 stdout/stderr 读取**：Java 与 Vue 构建阶段均使用两个 goroutine 分别读取 stdout 和 stderr，stderr 实时输出且写入 `strings.Builder`，避免 Rust 版 stderr 仅在失败时读取导致的管道缓冲区阻塞风险。
- **并发输出互斥锁**：`pkg/utils` 引入 `outputMu` 互斥锁，`SafePrintf`/`SafePrintln` 及 `Print*` 系列共享该锁，`deploy` 层并发 goroutine 的状态行改用 `SafePrintf`，避免多环境/多模块并发部署时日志文本交错。

### 4.4 上传与 SSH

- **SFTP 协议**：基于 `golang.org/x/crypto/ssh` + `github.com/pkg/sftp`，支持 `MkdirAll` 自动创建远程目录、`Stat` 检查文件存在性；备份通过 `Remove` + `Rename` 两步操作实现（非原子，见 §7 风险 6），最终以 `Rename` 将临时文件移动到目标路径实现原子上线。
- **SSHClient 结构体封装**：封装 `ssh.Client` 作为持久连接，`ExecuteCommand`/`KillProcess` 等方法通过 `client.NewSession()` 按需创建 session 并以 `defer` 管理生命周期。（原结构体中声明的 `session` 死字段已清理）
- **TestConnection 函数**：独立的 SSH 连通性测试（先 TCP Dial 再 SSH 认证）。
- **30 秒连接超时**：`ssh.Dial` 配置 `Timeout=30s`，自动补全缺失的 `:22` 端口。
- **流式文件读取**：`UploadFile` 使用 `os.Open` + `io.Copy` 流式读取，大文件不占内存，每次重试重新打开文件。

### 4.5 其他

- **CheckNodeJS 工具函数**：独立检测 `node --version` 与 `npm --version`，虽未被 main 调用但提供独立能力。
- **结构化的发现返回类型**：`JDKInfo`（含 `Path`、`Version`、`Source` 字段）与 `MavenInfo`（含 `Path`、`Version`、`Source` 字段），比 Rust 版纯 `String` 返回携带更多上下文。
- **更鲁棒的 Java 版本解析**：`parseJavaVersion` 将输出转小写后匹配，额外支持 `version "8`、`openjdk version "1.8` 等格式。
- **更完善的 Java 8 版本识别**：`containsJavaVersion` 为版本 8 追加检查 `1.8` 格式 XML 标签（`<java.version>1.8</java.version>` 等），Rust 版仅检查 `8` 格式。

---

## 5. Go 相对 Rust 的缺失或退化

### 5.1 功能缺失（影响功能或用户体验）

| 缺失项 | 影响程度 | 说明 |
|--------|----------|------|
| ✅ **文件读取完整性校验（已修复）** | 原 🔴 高 | 初版 `UploadFile` 仅通过 `os.Stat` 获取大小用于日志，不校验实际读取字节数。现已在 `io.Copy` 后校验 `copied == fileSize`，不一致时返回错误，防止静默上传不完整文件（`upload/ssh.go`）。 |
| ✅ **ProgressWriter 速率/ETA 统计（已覆盖）** | 原 🟡 中 | 初版认为 `progressbar` 仅展示基础字节进度。实测 `progressbar v3.14.1` 在 `OptionShowBytes(true)` 下默认启用 `predictTime/elapsedTime`，已内置传输速率（如 MB/s）与剩余时间 ETA 显示，并配置 `OptionThrottle(500ms)` 对齐 Rust 版刷新频率。Rust 版 `ProgressWriter` 仍以「同时显示当前速度+平均速度」略胜，但 Go 版速率/ETA 能力已具备。 |
| ⚠️ **MultiProgress 并发进度条（部分缓解）** | 🟡 中 | Rust 版 `indicatif` 的 `MultiProgress` 支持多线程共享统一进度条；Go 版 `progressbar` 不支持 `MultiProgress`。文本日志的并发交错已通过 `SafePrintf` 互斥锁解决，但多个 `progressbar` 实例的进度条视觉元素在并发上传时仍可能交错。 |
| ✅ **JAVA_HOME 注入存在 bug（已修复）** | 原 🔴 高 | 初版使用 `append` 注入 `JAVA_HOME`，父进程已有该变量时产生重复 key，Unix 取第一个导致新值不生效。现已改用 `overrideEnv()` 先剔除同名条目再追加，确保唯一生效（`compiler/build.go`、`compiler/java.go`）。 |
| ✅ **UploadZipOnly .zip 后缀处理缺陷（已修复）** | 原 🔴 高 | 初版 `TrimSuffix` 后紧跟恒为真的 `if` 回退，导致后缀实际未去除。现已删除该回退，`remoteZipPath` 直接使用 `TrimSuffix` 结果（`upload/ssh.go`）。 |
| ✅ **KillProcess 静默吞错（已修复）** | 原 🔴 高 | 初版 `findPidCmd` 失败时 `return nil`，会把 SSH 异常误判为"无进程可杀"。现已在命令尾部加 `|| true` 兜底退出码，`err != nil` 时通过 `%w` 向上传播（`upload/ssh.go`）。 |

> **说明**：原表第 7 项「Java 8 的 `1.8` XML 变体处理」实为 **Go 版相对 Rust 的增强**（Go 额外支持 `1.8` 格式，Rust 不支持），不属于 Go 缺失，已移至 §4.5。

### 5.2 行为差异（功能对等但实现不同）

| 差异项 | 说明 |
|--------|------|
| ✅ **上传层重试职责划分（已统一）** | 初版重试循环在 `UploadAndRunJar` / `UploadJarOnly` 中各自重复实现（`UploadZipOnly` / `UploadAndExtractZip` 原为单次上传，无重试）。现统一提取为 `SSHClient.uploadWithRetry` 共享方法，四个顶层上传函数均调用该方法，使全部上传路径具备一致的重试行为；Rust 版重试逻辑内聚在 `upload_to_remote` 内部。 |
| **zip 遍历容错** | Go 版 `filepath.Walk` 遇到错误直接传播并中止遍历；Rust 版 `walkdir::WalkDir` + `filter_map(Result::ok)` 静默跳过权限不足等错误项。 |
| **jar_files 多态处理** | Go 版封装为 `GetJarFilesList()` 方法（代码复用更好）；Rust 版在 `deploy.rs` 中 inline 做 `Value::Array` / `Value::String` 模式匹配，存在重复的模型过滤与路径拼接逻辑。 |
| **upload_only 优先级** | Go 版 `finalUploadOnly := uploadOnly || cfg.UploadOnly`（布尔 OR）；Rust 版先判断命令行再判断配置文件（if/else 显式优先）。二者行为等价，但 OR 逻辑在未来增加三态标志时可能产生歧义。 |
| **远程路径分隔符** | Go 版在 Java/Vue 部署中均 `strings.ReplaceAll(remotePath, "\\", "/")` 统一为 Unix 风格；Rust 版直接 `format!("{}/{}", ...)` 拼接，依赖配置书写规范。 |

### 5.3 无功能影响但机制不同

- **`upload_only` 默认值机制**：Go 版依赖 `bool` 零值 `false`；Rust 版使用 `#[serde(default)]` 显式声明。功能等价，但 Go 版若未来改为非零默认值（如 `true`），缺失字段行为会不一致。
- **`from_file` 返回值**：Go 版返回 `*DeployConfig`（指针）；Rust 版返回 `DeployConfig`（值）+ `String` 错误。Go 版指针方式无需 `Clone`，Rust 版需派生 `Clone`。
- **构建版本标识**：Go 版打印 `v2025.12.08`，Rust 版打印 `v2025.11.20`，日期版本号不同。
- **TOML 序列化**：Go 版 `toml.NewEncoder` 流式写入；Rust 版 `toml::to_string_pretty` 先序列化为字符串再写入。

---

## 6. 技术栈差异

### 6.1 核心依赖映射

| 能力 | Rust 原版 | Go 重写版 |
|------|-----------|-----------|
| CLI 框架 | `clap` v4.4.11（builder 模式，`value_delimiter(',')`） | `cobra` v1.8.0（`StringSliceVarP` 原生支持逗号分隔） |
| 配置解析 | `serde` + `toml` + `serde_json`（`Value` 多态） | `BurntSushi/toml`（`interface{}` + 类型断言） |
| SSH 通信 | `ssh2`（libssh2 C 绑定，SCP 协议） | `golang.org/x/crypto/ssh` + `github.com/pkg/sftp`（纯 Go 实现，SFTP 协议） |
| 进度条 | `indicatif`（`ProgressBar` + `MultiProgress`，自定义 `ProgressWriter`，含当前/平均速度与 ETA） | `schollz/progressbar/v3` v3.14.1（`OptionShowBytes` 内置速率/ETA 显示，`OptionThrottle(500ms)` 节流刷新；无 `MultiProgress`） |
| 错误处理 | `anyhow::Result<T>` + `.context()` + `bail!` | 原生 `error` 接口 + `fmt.Errorf`（关键路径用 `%w` 保留错误链） |
| 时间处理 | `chrono`（`Local::now()`、格式化） | 标准库 `time`（`time.Now()`、`time.Since`） |
| 目录遍历 | `walkdir` crate | 标准库 `filepath.Walk` |
| Glob 匹配 | `glob` crate | 标准库 `filepath.Glob` |
| ZIP 打包 | `zip` crate + `walkdir` | 标准库 `archive/zip` + `filepath.Walk` |
| 序列化 | `serde` derive（`Serialize`、`Deserialize`、`Clone`） | 无 derive，手动 toml 编码/指针语义 |

### 6.2 错误处理哲学

- **Rust 原版**：统一使用 `anyhow::Result<T>`，通过 `.context("...")?` 链式附加操作上下文，`bail!` 宏提前返回。错误链清晰，调试时能追溯到失败步骤的环境信息（如 `JAVA_HOME` 值、Maven 路径）。
- **Go 重写版**：原生 `error` 接口。关键步骤（SSH 连接、会话创建、远程命令、SFTP 客户端创建、远程文件写入、Java 构建失败等）已使用 `fmt.Errorf("...: %w", err)` 保留错误链，`NewSSHClient` 连接失败错误还附加了服务器地址，支持 `errors.Is/As` 追溯根因。少量非关键路径（打开本地文件、获取文件信息、备份/移动远程文件等）仍使用 `%v`；整体为手动拼接格式，无 Rust 版 `anyhow` 结构化 `.context()` 的链式便捷性，但错误链能力已基本对等。

### 6.3 交叉编译

- **Go 版**：无需额外配置文件，通过环境变量 `GOOS=windows GOARCH=amd64 go build` 原生支持交叉编译。
- **Rust 版**：需在 `.cargo/config.toml` 中显式配置 `[target.x86_64-pc-windows-gnu]` 段，指定 `linker` 与 `ar`，依赖系统预装 `mingw` 工具链。

### 6.4 外部依赖数量

- **Rust 原版**：11 个外部 crate（`clap`、`ssh2`、`serde`、`serde_json`、`toml`、`indicatif`、`anyhow`、`walkdir`、`glob`、`zip`、`chrono`，见根 `Cargo.toml`）。
- **Go 重写版**：5 个外部依赖（`cobra`、`golang.org/x/crypto/ssh`、`github.com/pkg/sftp`、`github.com/schollz/progressbar/v3`、`github.com/BurntSushi/toml`），其余使用标准库。

---

## 7. 潜在风险与注意事项

按严重程度从高到低排序。其中原 🔴 高风险 1–4 已在 `go` 分支修复，以 ✅ 标注。

### ✅ 已修复的高风险（原 1–4）

1. ~~**JAVA_HOME 环境变量注入 bug**~~ **✅ 已修复**：`compiler/java.go` 已改用 `overrideEnv()`（`compiler/build.go`）覆盖式设置，先剔除父进程同名变量再追加，不再产生重复 key。
2. ~~**UploadZipOnly .zip 后缀去除逻辑缺陷**~~ **✅ 已修复**：已删除 `TrimSuffix` 后恒为真的 `if` 回退，`remoteZipPath` 直接使用 `TrimSuffix` 结果，后缀正确去除（`upload/ssh.go`）。
3. ~~**KillProcess 静默吞错**~~ **✅ 已修复**：`findPidCmd` 末尾添加 `|| true` 兜底退出码，`err != nil` 时通过 `fmt.Errorf("...: %w", err)` 向上传播，不再误判为"无进程可杀"而跳过停服导致双实例运行（`upload/ssh.go`）。
4. ~~**文件读取完整性校验缺失**~~ **✅ 已修复**：`UploadFile` 已捕获 `io.Copy` 写入字节数，与 `os.Stat` 的 `fileSize` 比对，不一致时报错，防止静默上传不完整文件（`upload/ssh.go`）。

### ✅ 已调整的高风险（原 5）

5. ~~**pom.xml 读取失败降级风险**~~ **✅ 已改为报错**：`detectJavaVersion` 读取 `pom.xml` 失败时不再降级到 `FindAnyJDK()`，而是直接返回错误（fail-fast），与 Rust 版行为对齐，避免误用与项目要求不符的 JDK 版本（如项目要求 Java 17 却用 Java 8）导致编译/运行时问题。

### 🟡 中等风险（建议评估）

6. **SFTP 备份竞态条件**：Go 版使用 `sftpClient.Remove(backupPath)` + `sftpClient.Rename(remotePath, backupPath)` 两步操作完成备份，非原子，两步之间若有其他部署流程介入存在竞态。Rust 版通过 shell 命令 `rm -rf {backup}.bak && mv {remote} {backup}.bak` 同样为两步（亦非严格原子）。
7. ~~**errChan 缓冲区固定为 100**~~ **✅ 已修复（缓冲=任务总数）**：`deploy` 层先收集有效任务到切片，`errChan` 缓冲设为 `len(tasks)`；每个 goroutine 至多发送 1 个错误后即返回，缓冲永远够用，既不阻塞 goroutine 也不丢失错误。（注：原"改为无缓冲"的建议不可取——发送发生在 `wg.Wait()`/`close()` 之前、此时无接收方，无缓冲会必然死锁。）
8. **部署层错误收集后继续执行**：Go 版收集所有 goroutine 错误后继续执行剩余步骤，可能掩盖部分失败但保证整体流程完整；Rust 版在线程内直接 `eprintln!` 并 `return`，同样不传播但更早放弃当前模块。

### 🟢 低风险（可接受或需关注）

9. **进度条无 MultiProgress 并发支持**：文本日志并发交错已通过 `SafePrintf` 互斥锁解决，但多个 `progressbar` 实例的进度条视觉元素在并发部署时仍可能交错，用户体验不如 Rust 版统一进度条。
10. ~~**缺失 ProgressWriter 速率/ETA 展示**~~ **✅ 已覆盖**：`progressbar v3.14.1` 在 `OptionShowBytes(true)` 下默认显示传输速率与剩余时间 ETA，并配置 `OptionThrottle(500ms)` 节流刷新。Rust 版仍以「当前速度+平均速度」双指标略胜。
11. **错误链 ergonomics 差异**：Go 关键路径已用 `%w` 保留可解包错误链，`errors.Is/As` 可追溯根因，但 `UploadFile` 等函数仍有 `%v` 残留导致局部链断裂；此外 Rust 版 `anyhow` `.context()` 自动上下文附加的便捷性仍无直接等价物，调试时上下文仍需手动拼接。
12. **SSH 连接超时行为差异**：Go 版显式配置 30s 超时，Rust 版依赖 OS 默认 TCP 超时（可能无限阻塞）。若服务器响应慢，Rust 版可能 hanging。
13. ~~**MkdirAll 忽略目录已存在错误**~~ **✅ 已修复**：`UploadFile` 中 `MkdirAll` 现仅容忍 `os.ErrExist`，其余错误（权限不足、SFTP 连接异常等）通过 `%w` 向上传播，不再静默失败导致后续上传出错。
14. **每次重试重新读取文件**：对大文件增加磁盘 IO 开销，且若文件在重试间隙被修改可能导致版本不一致。
15. **远程路径 Windows 分隔符处理差异**：Go 版 `filepath.Join` 后 `ReplaceAll("\\", "/")`，在 Windows 本地运行时与 Rust 版（始终用 `/`）存在差异，但最终结果一致。

---

## 8. 最终评价与建议

### 8.1 是否可作为 Rust 版的替代

**可以作为 Rust 版的完整替代。**

Go 重写版完整还原了 Rust 原版的核心业务逻辑（Java 编译、Vue 构建、ZIP 打包、SFTP 上传、进程管理），且在以下方面表现优于 Rust 版：

- 代码组织更清晰（`discovery`/`timeout` 包解耦、`utils` 工具集中化）
- 环境适应性更强（Maven Wrapper、MAVEN_HOME/M2_HOME、更广 JDK/Maven 搜索路径）
- 错误处理更友好（并发错误聚合返回、关键路径 `%w` 错误链、连接失败附服务器地址）
- 并发输出更可控（`SafePrintf` 互斥锁抑制日志交错）
- 交叉编译更简单（环境变量 vs `.cargo/config.toml`）
- 外部依赖更少（5 个 vs 11 个）

初版报告指出的 3 个高风险 bug（JAVA_HOME 注入、UploadZipOnly 后缀处理、KillProcess 静默吞错）和 1 个功能缺失（文件完整性校验）**已全部修复**；并发输出锁、错误链保留、统一上传重试、进度条节流等 P2/P3 改进也已落地。本轮另完成错误链 `%v→%w` 全量统一、`session` 死字段清理、`MkdirAll` 吞错修复，并将 pom.xml 降级改为报错、`errChan` 缓冲改为任务总数（§8.3）。剩余的 SFTP 备份两步操作等少量低风险点不阻塞生产使用，可按需逐步完善。

### 8.2 优先完善清单（✅ 全部已完成）

下列清单在 `go` 分支已逐项落地，每项对应一次独立提交：

1. ~~【P0】修复 JAVA_HOME 环境变量注入 bug~~ **✅ 已完成**：新增 `overrideEnv()` 先剔除父进程同名条目再追加新值，确保注入值唯一生效（`compiler/build.go`、`compiler/java.go`）；附单元测试覆盖去重与唯一性。
2. ~~【P0】修复 UploadZipOnly .zip 后缀处理逻辑~~ **✅ 已完成**：删除 `TrimSuffix` 后恒为真的 `if` 回退，直接使用 `strings.TrimSuffix(remotePath, ".zip")` 的结果（`upload/ssh.go`）。
3. ~~【P0】修复 KillProcess 错误传播~~ **✅ 已完成**：`findPidCmd` 尾部加 `|| true` 兜底退出码，`err != nil` 时向上传播而非 `return nil`（`upload/ssh.go`）。
4. ~~【P1】增加文件读取完整性校验~~ **✅ 已完成**：`UploadFile` 捕获 `io.Copy` 写入字节数，与 `os.Stat` 的 `fileSize` 比对，不一致时报错（`upload/ssh.go`）。
5. ~~【P1】抑制并发输出交错~~ **✅ 已完成**：`pkg/utils` 新增 `outputMu` + `SafePrintf`/`SafePrintln`，`Print*` 系列共享该锁，`deploy` 层并发 goroutine 状态行改用 `SafePrintf`；附 200 goroutine 并发测试（`utils.go`、`deploy/*.go`）。
6. ~~【P2】增加 anyhow 风格的错误链或统一错误包装格式~~ **✅ 已完成**：`NewSSHClient` 连接失败补充服务器地址；SSH 会话/远程命令/SFTP 客户端/远程文件写入、Java 构建失败等关键步骤 `%v`→`%w` 保留错误链（`upload/ssh.go`、`compiler/java.go`）。
7. ~~【P2】统一上传重试逻辑~~ **✅ 已完成**：新增 `SSHClient.uploadWithRetry` 共享方法，`UploadAndRunJar`/`UploadJarOnly` 原重复重试循环改为调用它，`UploadZipOnly`/`UploadAndExtractZip`（原单次上传）也接入重试（`upload/ssh.go`）。
8. ~~【P3】考虑 Rust 版 ProgressWriter 的速率/ETA 统计~~ **✅ 已完成**：`UploadFile` 进度条配置 `OptionThrottle(500ms)` 节流刷新，与 Rust 版刷新频率对齐；`progressbar v3.14.1` 在 `OptionShowBytes(true)` 下已内置传输速率与剩余时间显示。

### 8.3 后续可选优化（✅ 本轮已全部完成）

下列各项在 `go` 分支本轮修复中逐项落地：

1. ~~将 `UploadFile` 等函数中残留的 `%v` 错误包装统一为 `%w`~~ **✅ 已完成**：upload/compiler/config/deploy 包内对应 `err` 的 `%v` 统一改为 `%w`（仅保留 `build.go`、`ssh.go` 中 `%v` 实为 `time.Duration` 的 3 处），错误链全程可 `errors.Is/As` 解包。
2. ~~清理 `SSHClient` 中未被赋值的 `session` 死字段~~ **✅ 已完成**：删除 `SSHClient.session` 字段及 `Close()` 中恒为 false 的判空分支。
3. ~~评估 pom.xml 失败时是否改为报错~~ **✅ 已完成（改为报错）**：`detectJavaVersion` 读取 `pom.xml` 失败不再降级到 `FindAnyJDK()`，直接返回错误（fail-fast），与 Rust 版一致。
4. ~~评估 `errChan` 缓冲 100 是否需改为无缓冲或动态扩容~~ **✅ 已完成（缓冲=任务总数）**：`errChan` 缓冲改为 `len(tasks)`（先收集任务再启动）。说明：原报告"改为无缓冲"的建议不可取——发送发生在 `wg.Wait()`/`close()` 之前、此时无接收方，无缓冲会必然死锁，故采用"缓冲=任务总数"。

---

*报告基于 9 个功能维度的结构化对比生成，并已与 `go` 分支当前代码逐项核对同步；所有事实均来源于代码与配置文件，未引入外部假设。*
