use anyhow::{bail, Context, Result};
use glob::glob;
use std::fs::File;
use std::io::prelude::*;
use std::io::{BufRead, BufReader};
use std::path::Path;
use std::process::{Command, Stdio};
use walkdir::WalkDir;
use zip::{write::FileOptions, ZipWriter};

/// 打包 Java 项目
pub fn build_java_project(project_dir: &str) -> Result<()> {
    use std::io::{self, Write};

    // 强制刷新 stdout 以确保日志即时显示
    let _ = io::stdout().flush();
    println!("正在初始化构建流程 (v2025.11.20)...");

    // 检测并设置合适的JDK版本
    let java_home = detect_java_version(project_dir)?;
    println!("使用 JAVA_HOME: {}", java_home);

    // 查找Maven可执行文件
    let maven_cmd = find_maven_executable()?;

    // 构建Maven命令参数
    let args = vec!["clean", "package", "-DskipTests"];

    println!("执行构建命令: {} {}", maven_cmd, args.join(" "));
    let _ = io::stdout().flush();

    let mut child = Command::new(&maven_cmd)
        .args(&args)
        .current_dir(project_dir)
        // 继承所有环境变量
        .envs(std::env::vars())
        // 设置JAVA_HOME
        .env("JAVA_HOME", &java_home)
        .stdout(Stdio::piped())
        .stderr(Stdio::piped())
        .spawn()
        .context("无法启动构建命令")?;

    // 读取并显示标准输出
    if let Some(stdout) = child.stdout.take() {
        let reader = BufReader::new(stdout);
        for line in reader.lines() {
            if let Ok(line) = line {
                println!("{}", line);
            }
        }
    }

    // 等待命令执行完成
    let status = child.wait().context("等待命令完成失败")?;

    if status.success() {
        println!("Java 项目构建成功!");
        Ok(())
    } else {
        // 读取错误输出
        let mut error_msg = String::new();
        if let Some(stderr) = child.stderr.take() {
            let reader = BufReader::new(stderr);
            error_msg = reader
                .lines()
                .filter_map(|line| line.ok())
                .collect::<Vec<String>>()
                .join("\n");
        }

        // 如果 stderr 为空，尝试给一些提示
        if error_msg.trim().is_empty() {
            error_msg =
                "未获取到错误输出 (stderr 为空)。可能是命令未找到或环境变量配置错误。".to_string();
        }

        bail!(
            "构建失败 (退出码: {:?})\n错误详情:\n{}\n\n建议:\n1. 检查 JAVA_HOME 是否正确: {}\n2. 检查 Maven 路径: {}\n3. 尝试手动执行: {} {}",
            status.code(),
            error_msg,
            java_home,
            maven_cmd,
            maven_cmd,
            args.join(" ")
        );
    }
}

/// 打包 Vue 项目
pub fn build_vue_project(project_dir: &str, scripts: &str) -> Result<()> {
    // 检测操作系统类型
    let is_windows = cfg!(target_os = "windows");

    // 根据操作系统类型选择适当的命令
    let mut child = if is_windows {
        Command::new("cmd")
            .args(["/c", "npm", "run", scripts])
            .current_dir(project_dir)
            .stdout(Stdio::piped())
            .stderr(Stdio::piped())
            .spawn()
    } else {
        Command::new("npm")
            .args(["run", scripts])
            .current_dir(project_dir)
            .stdout(Stdio::piped())
            .stderr(Stdio::piped())
            .spawn()
    }
    .context("执行npm命令失败")?;

    // 读取并显示标准输出
    if let Some(stdout) = child.stdout.take() {
        let reader = BufReader::new(stdout);
        for line in reader.lines() {
            if let Ok(line) = line {
                println!("{}", line);
            }
        }
    }

    // 等待命令执行完成
    let status = child.wait().context("等待命令完成失败")?;

    if status.success() {
        println!("{}环境下的Vue项目构建成功!", scripts);
        Ok(())
    } else {
        // 读取错误输出
        if let Some(stderr) = child.stderr.take() {
            let reader = BufReader::new(stderr);
            let error = reader
                .lines()
                .filter_map(|line| line.ok())
                .collect::<Vec<String>>()
                .join("\n");
            bail!("构建失败:请检查npm是否配置在环境变量中\n{}", error);
        } else {
            bail!("构建失败，无法获取错误信息");
        }
    }
}

// 将目录打包成zip文件
pub fn zip_dir(zip: &mut ZipWriter<File>, src_dir: &str, options: FileOptions) -> Result<()> {
    let src_path = Path::new(src_dir);

    // 确保源目录存在
    if !src_path.exists() || !src_path.is_dir() {
        bail!("源目录不存在或不是一个目录: {}", src_dir);
    }

    let walkdir = WalkDir::new(src_dir);

    for entry in walkdir.into_iter().filter_map(Result::ok) {
        let path = entry.path();

        // 跳过源目录本身
        if path == src_path {
            continue;
        }

        // 计算相对路径
        let rel_path = path.strip_prefix(src_path).context("计算相对路径失败")?;

        // 直接使用相对路径，不添加顶级目录
        let zip_path_str = rel_path.to_str().context("路径转换失败")?;

        // 替换Windows路径分隔符为ZIP标准的/
        let zip_path_str = zip_path_str.replace('\\', "/");

        if path.is_file() {
            zip.start_file(&zip_path_str, options)
                .context("添加文件到ZIP失败")?;
            let mut f = File::open(path).context("打开文件失败")?;
            let mut buffer = Vec::new();
            f.read_to_end(&mut buffer).context("读取文件失败")?;
            zip.write_all(&buffer).context("写入ZIP失败")?;
        } else if path.is_dir() {
            // 确保目录路径以/结尾
            let dir_path = if zip_path_str.ends_with('/') {
                zip_path_str
            } else {
                format!("{}/", zip_path_str)
            };

            zip.add_directory(&dir_path, options)
                .context("添加目录到ZIP失败")?;
        }
    }

    Ok(())
}

/// 检查指定路径的Java版本
fn check_java_version(java_home: &Path) -> Option<String> {
    let java_bin = if cfg!(target_os = "windows") {
        java_home.join("bin").join("java.exe")
    } else {
        java_home.join("bin").join("java")
    };

    if !java_bin.exists() {
        return None;
    }

    let output = Command::new(&java_bin).arg("-version").output().ok()?;

    let version_output = String::from_utf8_lossy(&output.stderr);

    // 解析版本号,支持多种格式
    if version_output.contains("\"1.8.") || version_output.contains("\"8.") {
        Some("8".to_string())
    } else if version_output.contains("\"11.") {
        Some("11".to_string())
    } else if version_output.contains("\"17.") {
        Some("17".to_string())
    } else if version_output.contains("\"21.") {
        Some("21".to_string())
    } else {
        None
    }
}

/// 智能查找合适版本的JAVA_HOME
fn find_java_home(required_version: &str) -> Result<String> {
    println!("🔍 正在搜索Java {}...", required_version);

    // 1. 首先检查环境变量JAVA_HOME
    if let Ok(java_home) = std::env::var("JAVA_HOME") {
        let path = Path::new(&java_home);
        if path.exists() {
            if let Some(version) = check_java_version(path) {
                if version == required_version {
                    println!("✅ 使用环境变量JAVA_HOME: {}", java_home);
                    return Ok(java_home);
                } else {
                    println!(
                        "⚠️  JAVA_HOME版本不匹配(需要{},当前{}),继续搜索...",
                        required_version, version
                    );
                }
            }
        }
    }

    // 2. 定义搜索路径
    let search_paths: Vec<String> = if cfg!(target_os = "macos") {
        vec![
            // IDEA自带的JDK
            "/Applications/IntelliJ IDEA.app/Contents/jbr/Contents/Home".to_string(),
            "/Applications/IntelliJ IDEA CE.app/Contents/jbr/Contents/Home".to_string(),
            // 系统安装的JDK
            "/Library/Java/JavaVirtualMachines/*/Contents/Home".to_string(),
            // Homebrew安装的JDK
            "/opt/homebrew/opt/openjdk@*/libexec/openjdk.jdk/Contents/Home".to_string(),
            "/usr/local/opt/openjdk@*/libexec/openjdk.jdk/Contents/Home".to_string(),
        ]
    } else if cfg!(target_os = "windows") {
        vec![
            // IDEA自带的JDK
            "C:\\Program Files\\JetBrains\\IntelliJ IDEA*\\jbr".to_string(),
            // Zulu JDK
            "C:\\Program Files\\Zulu\\zulu-*".to_string(),
            // Oracle JDK
            "C:\\Program Files\\Java\\jdk*".to_string(),
            "C:\\Program Files\\Java\\jre*".to_string(),
        ]
    } else {
        // Linux
        vec![
            // IDEA自带的JDK (Toolbox)
            format!(
                "{}/.local/share/JetBrains/Toolbox/apps/IDEA-*/*/jbr",
                std::env::var("HOME").unwrap_or_default()
            ),
            // 系统安装的JDK
            "/usr/lib/jvm/*".to_string(),
            "/usr/java/*".to_string(),
        ]
    };

    // 3. 搜索所有可能的路径
    let mut found_jdks: Vec<(String, String)> = Vec::new();

    for pattern in search_paths {
        if let Ok(paths) = glob(&pattern) {
            for entry in paths.filter_map(Result::ok) {
                if let Some(version) = check_java_version(&entry) {
                    let path_str = entry.to_string_lossy().to_string();
                    println!("   发现 Java {} at {}", version, path_str);
                    found_jdks.push((version, path_str));
                }
            }
        }
    }

    // 4. 查找匹配版本的JDK
    for (version, path) in &found_jdks {
        if version == required_version {
            println!("✅ 找到匹配的Java {}: {}", required_version, path);
            return Ok(path.clone());
        }
    }

    // 5. 如果没找到,返回详细错误信息
    if found_jdks.is_empty() {
        bail!(
            "❌ 未找到任何Java安装。请安装Java {}。\n建议:\n  - macOS: brew install openjdk@{}\n  - Windows: 下载并安装Zulu JDK {}\n  - Linux: sudo apt install openjdk-{}-jdk",
            required_version, required_version, required_version, required_version
        );
    } else {
        let available_versions: Vec<String> = found_jdks
            .iter()
            .map(|(v, p)| format!("  - Java {} at {}", v, p))
            .collect();
        bail!(
            "❌ 未找到Java {},但发现以下版本:\n{}\n\n请安装Java {}或修改项目配置使用已有版本。",
            required_version,
            available_versions.join("\n"),
            required_version
        );
    }
}

/// 智能查找Maven可执行文件
fn find_maven_executable() -> Result<String> {
    println!("🔍 正在搜索Maven...");

    // 1. 首先检查PATH中是否有mvn
    let mvn_cmd = if cfg!(target_os = "windows") {
        "mvn.cmd"
    } else {
        "mvn"
    };

    if let Ok(output) = Command::new(mvn_cmd).arg("--version").output() {
        if output.status.success() {
            println!("✅ 使用PATH中的Maven");
            return Ok(mvn_cmd.to_string());
        }
    }

    // 2. 定义搜索路径
    let search_paths: Vec<String> = if cfg!(target_os = "macos") {
        vec![
            // IDEA自带的Maven
            "/Applications/IntelliJ IDEA.app/Contents/plugins/maven/lib/maven3/bin/mvn".to_string(),
            "/Applications/IntelliJ IDEA CE.app/Contents/plugins/maven/lib/maven3/bin/mvn"
                .to_string(),
            // Homebrew安装的Maven
            "/opt/homebrew/bin/mvn".to_string(),
            "/usr/local/bin/mvn".to_string(),
            // 手动安装的Maven
            "/usr/local/maven*/bin/mvn".to_string(),
        ]
    } else if cfg!(target_os = "windows") {
        vec![
            // IDEA自带的Maven
            "C:\\Program Files\\JetBrains\\IntelliJ IDEA*\\plugins\\maven\\lib\\maven3\\bin\\mvn.cmd".to_string(),
            // 手动安装的Maven
            "C:\\Program Files\\Apache\\maven*\\bin\\mvn.cmd".to_string(),
            "C:\\Program Files\\Maven\\apache-maven*\\bin\\mvn.cmd".to_string(),
        ]
    } else {
        // Linux
        vec![
            // IDEA自带的Maven (Toolbox)
            format!(
                "{}/.local/share/JetBrains/Toolbox/apps/IDEA-*/*/plugins/maven/lib/maven3/bin/mvn",
                std::env::var("HOME").unwrap_or_default()
            ),
            // 系统安装的Maven
            "/usr/bin/mvn".to_string(),
            "/usr/local/bin/mvn".to_string(),
            "/usr/local/maven*/bin/mvn".to_string(),
        ]
    };

    // 3. 搜索所有可能的路径
    for pattern in search_paths {
        if let Ok(paths) = glob(&pattern) {
            for entry in paths.filter_map(Result::ok) {
                if entry.exists() {
                    let path_str = entry.to_string_lossy().to_string();
                    // 验证Maven是否可用
                    if let Ok(output) = Command::new(&path_str).arg("--version").output() {
                        if output.status.success() {
                            println!("✅ 找到Maven: {}", path_str);
                            return Ok(path_str);
                        }
                    }
                }
            }
        }
    }

    // 4. 如果没找到,返回错误信息
    bail!(
        "❌ 未找到Maven安装。请安装Maven。\n建议:\n  - macOS: brew install maven\n  - Windows: 下载并安装Apache Maven\n  - Linux: sudo apt install maven\n或者使用IntelliJ IDEA(自带Maven)"
    );
}

/// 检测项目需要的Java版本并返回对应的JAVA_HOME路径
fn detect_java_version(project_dir: &str) -> Result<String> {
    use std::fs;

    // 读取pom.xml文件
    let pom_path = format!("{}/pom.xml", project_dir);
    let pom_content = fs::read_to_string(&pom_path).context("无法读取pom.xml文件")?;

    // 检查Java版本配置
    let java_version = if pom_content.contains("<java.version>8</java.version>")
        || pom_content.contains("<maven.compiler.source>8</maven.compiler.source>")
        || pom_content.contains("<maven.compiler.target>8</maven.compiler.target>")
    {
        "8"
    } else if pom_content.contains("<java.version>11</java.version>")
        || pom_content.contains("<maven.compiler.source>11</maven.compiler.source>")
        || pom_content.contains("<maven.compiler.target>11</maven.compiler.target>")
    {
        "11"
    } else if pom_content.contains("<java.version>17</java.version>")
        || pom_content.contains("<maven.compiler.source>17</maven.compiler.source>")
        || pom_content.contains("<maven.compiler.target>17</maven.compiler.target>")
    {
        "17"
    } else if pom_content.contains("<java.version>21</java.version>")
        || pom_content.contains("<maven.compiler.source>21</maven.compiler.source>")
        || pom_content.contains("<maven.compiler.target>21</maven.compiler.target>")
    {
        "21"
    } else {
        // 默认使用Java 8
        println!("⚠️  未检测到明确的Java版本配置，默认使用Java 8");
        "8"
    };

    // 使用智能检测函数查找对应版本的JDK
    find_java_home(java_version)
}
