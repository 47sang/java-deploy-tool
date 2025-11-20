use std::fs::File;
use std::io::prelude::*;
use std::io::{BufRead, BufReader};
use std::path::Path;
use std::process::{Command, Stdio};
use walkdir::WalkDir;
use zip::{write::FileOptions, ZipWriter};

/// 打包 Java 项目
/// 打包 Java 项目
pub fn build_java_project(project_dir: &str) -> Result<(), String> {
    use std::io::{self, Write};

    // 强制刷新 stdout 以确保日志即时显示
    let _ = io::stdout().flush();
    println!("正在初始化构建流程 (v2025.11.20)...");

    // 检测操作系统类型
    let is_windows = cfg!(target_os = "windows");

    // 检测并设置合适的JDK版本
    let java_home = detect_java_version(project_dir)?;
    println!("使用 JAVA_HOME: {}", java_home);

    // 构建命令和参数
    let (cmd, args) = if is_windows {
        ("cmd", vec!["/c", "mvn", "clean", "package", "-DskipTests"])
    } else {
        ("mvn", vec!["clean", "package", "-DskipTests"])
    };

    println!("执行构建命令: {} {}", cmd, args.join(" "));
    let _ = io::stdout().flush();

    let mut child = Command::new(cmd)
        .args(&args)
        .current_dir(project_dir)
        // 继承所有环境变量，特别是PATH（包含mvn的路径）
        .envs(std::env::vars())
        // 同时设置JAVA_HOME（如果冲突会被覆盖）
        .env("JAVA_HOME", &java_home)
        .stdout(Stdio::piped())
        .stderr(Stdio::piped())
        .spawn()
        .map_err(|e| format!("无法启动构建命令: {}", e))?;

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
    let status = child
        .wait()
        .map_err(|e| format!("等待命令完成失败: {}", e))?;

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

        Err(format!(
            "构建失败 (退出码: {:?})\n错误详情:\n{}\n\n建议:\n1. 检查 JAVA_HOME 是否正确: {}\n2. 检查 mvn 是否在 PATH 环境变量中\n3. 尝试手动执行: {} {}",
            status.code(),
            error_msg,
            java_home,
            cmd,
            args.join(" ")
        ))
    }
}

/// 打包 Vue 项目
pub fn build_vue_project(project_dir: &str, scripts: &str) -> Result<(), String> {
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
    .map_err(|e| format!("执行npm命令失败: {}", e))?;

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
    let status = child
        .wait()
        .map_err(|e| format!("等待命令完成失败: {}", e))?;

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
            Err(format!("构建失败:请检查npm是否配置在环境变量中\n{}", error))
        } else {
            Err("构建失败，无法获取错误信息".to_string())
        }
    }
}

// 将目录打包成zip文件
pub fn zip_dir(
    zip: &mut ZipWriter<File>,
    src_dir: &str,
    options: FileOptions,
) -> Result<(), String> {
    let src_path = Path::new(src_dir);

    // 确保源目录存在
    if !src_path.exists() || !src_path.is_dir() {
        return Err(format!("源目录不存在或不是一个目录: {}", src_dir));
    }

    let walkdir = WalkDir::new(src_dir);

    for entry in walkdir.into_iter().filter_map(Result::ok) {
        let path = entry.path();

        // 跳过源目录本身
        if path == src_path {
            continue;
        }

        // 计算相对路径
        let rel_path = path.strip_prefix(src_path).map_err(|e| e.to_string())?;

        // 直接使用相对路径，不添加顶级目录
        let zip_path_str = rel_path.to_str().ok_or("路径转换失败")?;

        // 替换Windows路径分隔符为ZIP标准的/
        let zip_path_str = zip_path_str.replace('\\', "/");

        if path.is_file() {
            zip.start_file(&zip_path_str, options)
                .map_err(|e| e.to_string())?;
            let mut f = File::open(path).map_err(|e| e.to_string())?;
            let mut buffer = Vec::new();
            f.read_to_end(&mut buffer).map_err(|e| e.to_string())?;
            zip.write_all(&buffer).map_err(|e| e.to_string())?;
        } else if path.is_dir() {
            // 确保目录路径以/结尾
            let dir_path = if zip_path_str.ends_with('/') {
                zip_path_str
            } else {
                format!("{}/", zip_path_str)
            };

            zip.add_directory(&dir_path, options)
                .map_err(|e| e.to_string())?;
        }
    }

    Ok(())
}

/// 检测项目需要的Java版本并返回对应的JAVA_HOME路径
fn detect_java_version(project_dir: &str) -> Result<String, String> {
    use std::fs;

    // 读取pom.xml文件
    let pom_path = format!("{}/pom.xml", project_dir);
    let pom_content =
        fs::read_to_string(&pom_path).map_err(|_| "无法读取pom.xml文件".to_string())?;

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

    // 根据操作系统和Java版本返回对应的JAVA_HOME路径
    let java_home = if cfg!(target_os = "macos") {
        match java_version {
            "8" => {
                // 检查是否安装了JDK 8
                let jdk8_path = "/Library/Java/JavaVirtualMachines/zulu-8.jdk/Contents/Home";
                if Path::new(jdk8_path).exists() {
                    println!(
                        "🔧 检测到项目需要Java {}，使用: {}",
                        java_version, jdk8_path
                    );
                    jdk8_path.to_string()
                } else {
                    // 尝试其他可能的JDK 8路径
                    let alt_paths = vec![
                        "/Library/Java/JavaVirtualMachines/adoptopenjdk-8.jdk/Contents/Home",
                        "/Library/Java/JavaVirtualMachines/temurin-8.jdk/Contents/Home",
                        "/Library/Java/JavaVirtualMachines/jdk1.8.0_*.jdk/Contents/Home",
                    ];

                    for path in alt_paths {
                        if Path::new(path).exists() {
                            println!("🔧 检测到项目需要Java {}，使用: {}", java_version, path);
                            return Ok(path.to_string());
                        }
                    }

                    return Err(format!(
                        "❌ 项目需要Java {}，但系统中未找到对应的JDK安装。请安装JDK 8",
                        java_version
                    ));
                }
            }
            "11" => {
                let jdk11_path = "/Library/Java/JavaVirtualMachines/zulu-11.jdk/Contents/Home";
                if Path::new(jdk11_path).exists() {
                    println!(
                        "🔧 检测到项目需要Java {}，使用: {}",
                        java_version, jdk11_path
                    );
                    jdk11_path.to_string()
                } else {
                    return Err(format!(
                        "❌ 项目需要Java {}，但系统中未找到对应的JDK安装",
                        java_version
                    ));
                }
            }
            "17" => {
                let jdk17_path = "/Library/Java/JavaVirtualMachines/zulu-17.jdk/Contents/Home";
                if Path::new(jdk17_path).exists() {
                    println!(
                        "🔧 检测到项目需要Java {}，使用: {}",
                        java_version, jdk17_path
                    );
                    jdk17_path.to_string()
                } else {
                    return Err(format!(
                        "❌ 项目需要Java {}，但系统中未找到对应的JDK安装",
                        java_version
                    ));
                }
            }
            "21" => {
                let jdk21_path = "/Library/Java/JavaVirtualMachines/zulu-21.jdk/Contents/Home";
                if Path::new(jdk21_path).exists() {
                    println!(
                        "🔧 检测到项目需要Java {}，使用: {}",
                        java_version, jdk21_path
                    );
                    jdk21_path.to_string()
                } else {
                    return Err(format!(
                        "❌ 项目需要Java {}，但系统中未找到对应的JDK安装",
                        java_version
                    ));
                }
            }
            _ => return Err(format!("❌ 不支持的Java版本: {}", java_version)),
        }
    } else if cfg!(target_os = "windows") {
        // Windows路径处理
        // Windows路径处理 - 适配 Zulu JDK
        let jdk_path = match java_version {
            "8" => "C:\\Program Files\\Zulu\\zulu-8",
            "11" => "C:\\Program Files\\Zulu\\zulu-11",
            "17" => "C:\\Program Files\\Zulu\\zulu-17",
            "21" => "C:\\Program Files\\Zulu\\zulu-21",
            _ => return Err(format!("❌ 不支持的Java版本: {}", java_version)),
        };

        if Path::new(jdk_path).exists() {
            println!("🔧 检测到项目需要Java {}，使用: {}", java_version, jdk_path);
            jdk_path.to_string()
        } else {
            return Err(format!(
                "❌ 项目需要Java {}，但系统中未找到对应的JDK安装: {}",
                java_version, jdk_path
            ));
        }
    } else {
        // Linux路径处理
        match java_version {
            "8" => "/usr/lib/jvm/java-8-openjdk".to_string(),
            "11" => "/usr/lib/jvm/java-11-openjdk".to_string(),
            "17" => "/usr/lib/jvm/java-17-openjdk".to_string(),
            "21" => "/usr/lib/jvm/java-21-openjdk".to_string(),
            _ => return Err(format!("❌ 不支持的Java版本: {}", java_version)),
        }
    };

    Ok(java_home)
}
