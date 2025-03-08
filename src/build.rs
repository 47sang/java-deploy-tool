use std::process::{Command, Stdio};
use std::io::{BufRead, BufReader};

/// 打包 Java 项目
pub fn build_java_project(project_dir: &str) -> Result<(), String> {
    // 尝试执行mvn命令，如果命令不存在会在spawn时返回错误
    let mut child = Command::new("cmd")
        .args(["/c", "mvn"])
        .arg("clean")
        .arg("package")
        .current_dir(project_dir)
        .stdout(Stdio::piped())
        .stderr(Stdio::piped())
        .spawn()
        .map_err(|e| format!("执行mvn命令失败: {}", e))?;

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
    let status = child.wait()
        .map_err(|e| format!("等待命令完成失败: {}", e))?;

    if status.success() {
        println!("Java 项目构建成功!");
        Ok(())
    } else {
        // 读取错误输出
        if let Some(stderr) = child.stderr.take() {
            let reader = BufReader::new(stderr);
            let error = reader.lines()
                .filter_map(|line| line.ok())
                .collect::<Vec<String>>()
                .join("\n");            
            Err(format!("构建失败:请检查mvn是否配置在环境变量中\n{}", error))
        } else {
            Err("构建失败，无法获取错误信息".to_string())
        }
    }
}

/// 打包 Vue 项目
pub fn build_vue_project(project_dir: &str,scripts: &str) -> Result<(), String> {
    let mut child = Command::new("cmd")
        .args(["/c", "npm", "run", scripts])
        .current_dir(project_dir)
        .stdout(Stdio::piped())
        .stderr(Stdio::piped())
        .spawn()
        .map_err(|e| format!("执行npm命令失败: {}", e))?;

    let status = child.wait()
        .map_err(|e| format!("等待命令完成失败: {}", e))?;

    if status.success() {
        println!("Vue 项目构建成功!");
        Ok(())
    } else {
        // 读取错误输出
        if let Some(stderr) = child.stderr.take() {
            let reader = BufReader::new(stderr);
            let error = reader.lines()
                .filter_map(|line| line.ok())
                .collect::<Vec<String>>()
                .join("\n");
            Err(format!("构建失败:请检查npm是否配置在环境变量中\n{}", error))
        } else {
            Err("构建失败，无法获取错误信息".to_string())
        }
    }
}


