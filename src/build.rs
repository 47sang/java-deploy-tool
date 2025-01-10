use std::process::{Command, Stdio};
use std::io::{BufRead, BufReader};

pub fn build_java_project(project_dir: &str) -> Result<(), String> {
    let mut child = Command::new("cmd")
        .args(["/c", "mvn"])
        .arg("clean")
        .arg("package")
        .current_dir(project_dir)
        .stdout(Stdio::piped())
        .stderr(Stdio::piped())
        .spawn()
        .map_err(|e| format!("命令执行失败: mvn package: {}", e))?;

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
            Err(format!("构建失败:\n{}", error))
        } else {
            Err("构建失败，无法获取错误信息".to_string())
        }
    }
}