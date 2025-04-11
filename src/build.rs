use std::process::{Command, Stdio};
use std::io::{BufRead, BufReader};
use std::io::prelude::*;
use std::path::Path;
use walkdir::WalkDir;
use zip::{write::FileOptions, ZipWriter};
use std::fs::File;

/// 打包 Java 项目
pub fn build_java_project(project_dir: &str) -> Result<(), String> {
    // 检测操作系统类型
    let is_windows = cfg!(target_os = "windows");
    
    // 根据操作系统类型选择适当的命令
    let mut child = if is_windows {
        Command::new("cmd")
            .args(["/c", "mvn", "clean", "package"])
            .current_dir(project_dir)
            .stdout(Stdio::piped())
            .stderr(Stdio::piped())
            .spawn()
    } else {
        Command::new("mvn")
            .args(["clean", "package"])
            .current_dir(project_dir)
            .stdout(Stdio::piped())
            .stderr(Stdio::piped())
            .spawn()
    }
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

    let status = child.wait()
        .map_err(|e| format!("等待命令完成失败: {}", e))?;

    if status.success() {
        println!("{}环境下的Vue项目构建成功!",scripts);
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


// 将目录打包成zip文件
pub fn zip_dir(zip: &mut ZipWriter<File>, src_dir: &str, options: FileOptions) -> Result<(), String> {
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
          zip.start_file(&zip_path_str, options).map_err(|e| e.to_string())?;
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
          
          zip.add_directory(&dir_path, options).map_err(|e| e.to_string())?;
      }
  }
  
  Ok(())
}

