mod build;
mod config;
mod run;
mod upload;

use build::{build_java_project, build_vue_project};
use clap::{Arg, Command};
use config::DeployConfig;
use run::run_jar;
use std::fs::File;
use std::io::prelude::*;
use std::path::Path;
use std::thread;
use std::time::{Duration, Instant};
use upload::{upload_file, upload_jar};
use walkdir::WalkDir;
use zip::CompressionMethod;
use zip::{write::FileOptions, ZipWriter};

fn main() {
    let matches = Command::new("deploy-tool")
        .version("1.0")
        .author("士钰 <zhoushiyu92@gmail.com>")
        .about("一键部署Java和Vue项目,支持多环境部署,支持多模块部署")
        .arg(
            Arg::new("env")
                .short('e')
                .long("env")
                .value_name("ENVIRONMENT")
                .help("部署后端服务环境，多个环境用逗号分隔 (例如: dev,prod)")
                .value_delimiter(',')
                .required(false),
        )
        .arg(
            Arg::new("vue")
                .short('v')
                .long("vue")
                .value_name("ENVIRONMENT")
                .help("部署web端环境，多个环境用逗号分隔 (例如: dev,prod)")
                .value_delimiter(',')
                .required(false),
        )
        .arg(
            Arg::new("init-config")
                .long("init-config")
                .help("创建示例配置文件")
                .action(clap::ArgAction::SetTrue),
        )
        .arg(
            Arg::new("project-dir")
                .short('p')
                .long("project-dir")
                .value_name("PROJECT_DIR")
                .help("指定项目根目录路径")
                .required(false)
                .default_value("."),
        )
        .get_matches();

    // 调用方法并测量执行时间
    measure_execution_time(|| {
        println!("开始执行脚本程序");
        let config_path = "./deploy.toml".to_string();

        // 如果指定了init-config参数，创建示例配置文件并退出
        if matches.get_flag("init-config") {
            match DeployConfig::create_springboot_config(&config_path) {
                Ok(_) => {
                    println!("示例配置文件已创建: {}", &config_path);
                    println!("请修改配置文件中的参数后再运行部署。");
                    return;
                }
                Err(e) => {
                    eprintln!("创建配置文件失败: {}", e);
                    return;
                }
            }
        }

        let project_dir = matches
            .get_one::<String>("project-dir")
            .unwrap_or(&".".to_string())
            .to_string();
        let environments: Vec<String> = matches
            .get_many::<String>("env")
            .unwrap_or_default()
            .map(|s| s.to_string())
            .collect();

        let vue_environments: Vec<String> = matches
            .get_many::<String>("vue")
            .unwrap_or_default()
            .map(|s| s.to_string())
            .collect();

        println!("project_dir: {}", project_dir);
        println!("environments: {:?}", environments);
        println!("vue_environments: {:?}", vue_environments);

        // 根据命令行参数选择执行部署函数
        if !environments.is_empty() {
            println!("开始编译Java项目,请稍等...");
            // 部署Java项目
            if let Err(e) = deploy_java_project(&project_dir, &config_path, &environments) {
                eprintln!("{}", e);
            }
        }

        if !vue_environments.is_empty() {
            println!("开始编译Vue项目,比较慢,请稍等...");
            // 部署Vue项目
            if let Err(e) = deploy_vue_project(&project_dir, &config_path, &vue_environments) {
                eprintln!("{}", e);
            }
        }
    });
}

// 部署Java项目的函数
fn deploy_java_project(
    project_dir: &str,
    config_path: &str,
    environments: &[String],
) -> Result<(), String> {
    // 构建Java项目
    if let Err(e) = build_java_project(project_dir) {
        return Err(e);
    }

    // 为每个环境创建部署任务
    let mut handles = vec![];

    for env in environments {
        let env = env.to_string();
        let config_path = config_path.to_string();
        let project_dir = project_dir.to_string();

        let config = match DeployConfig::from_file(&config_path, &env) {
            Ok(config) => config,
            Err(e) => {
                eprintln!("加载{}环境配置失败: {}", env, e);
                continue;
            }
        };
        for jar_name in &config.jar_files {
            // 应用命令行参数覆盖
            let config = config.clone();

            let jar_name = jar_name.to_string();
            let project_dir = project_dir.to_string();
            let env = env.clone();
            // 获取编译产物文件名称,组装上传路径
            let jar_path = if config.jar_files.len() == 1 {
                format!("{}/target/{}", project_dir, jar_name)
            } else {
                format!(
                    "{}/{}/target/{}",
                    project_dir,
                    jar_name.split(".").next().unwrap(),
                    jar_name
                )
            };

            let handle = thread::spawn(move || {
                let remote_path = format!("{}/{}", config.remote_base_path, jar_name);

                println!("开始部署 {} 到 {} 环境", jar_name, env);

                // 上传 JAR 包
                if let Err(e) = upload_jar(
                    &config.server,
                    &config.username,
                    &config.password,
                    &jar_path,
                    &remote_path,
                ) {
                    eprintln!("上传失败 {} ({}环境): {}", jar_name, env, e);
                    return;
                }
                println!("上传成功: {} ({}环境)", jar_name, env);

                // 运行 JAR 包
                if let Err(e) = run_jar(
                    &config.server,
                    &config.username,
                    &config.password,
                    &remote_path,
                    &config.java_path,
                    &env,
                ) {
                    eprintln!("运行失败 {} ({}环境): {}", jar_name, env, e);
                    return;
                }
                println!("运行成功: {} ({}环境)", jar_name, env);
            });
            handles.push(handle);
        }
    }

    // 等待所有线程完成
    for handle in handles {
        handle.join().unwrap();
    }

    Ok(())
}

// 定义一个测量执行时间的函数
fn measure_execution_time<F>(func: F) -> Duration
where
    F: FnOnce(), // 接受一个闭包作为参数
{
    let start = Instant::now(); // 记录开始时间
    func(); // 执行传入的函数
    let elapsed = start.elapsed(); // 返回执行时间
    let formatted_time = format!(
        "{:02}:{:02}:{:02}",
        elapsed.as_secs() / 3600,
        (elapsed.as_secs() % 3600) / 60,
        elapsed.as_secs() % 60
    );
    println!("本次部署执行时间: {}", formatted_time);
    let now = chrono::Local::now();
    println!("当前系统时间: {}", now.format("%Y-%m-%d %H:%M:%S"));
    elapsed // 返回执行时间
}

// 部署Vue项目的函数
fn deploy_vue_project(
    project_dir: &str,
    config_path: &str,
    environments: &[String],
) -> Result<(), String> {
    // 为每个环境创建部署任务
    let mut handles = vec![];

    for env in environments {
        let env = env.to_string();
        let config_path = config_path.to_string();
        let project_dir = project_dir.to_string();

        let config = match DeployConfig::from_file(&config_path, &env) {
            Ok(config) => config,
            Err(e) => {
                eprintln!("加载{}环境配置失败: {}", env, e);
                continue;
            }
        };

        let handle = thread::spawn(move || {
            // 构建Vue项目
            build_vue_project(&project_dir, &config.scripts).expect("构建Vue项目失败");

            // 压缩产出目录文件zip
            let output_dir = format!("{}\\{}", project_dir, config.output_dir);
            let zip_path = format!("{}\\{}.zip", project_dir, config.output_dir);
            let zip_file = File::create(&zip_path).expect("创建新的zip文件失败");
            let mut zip = ZipWriter::new(zip_file);
            let options = FileOptions::default().compression_method(CompressionMethod::Deflated);
            zip_dir(&mut zip, &output_dir, options).expect("压缩失败");
            zip.finish().expect("完成ZIP文件失败");

            // 上传zip文件
            let remote_path = format!("{}/{}", config.remote_base_path, config.output_dir);

            if let Err(e) = upload_file(
                &config.server,
                &config.username,
                &config.password,
                &zip_path,
                &remote_path,
            ) {
                eprintln!("上传失败 {} ({}环境): {}", config.output_dir, env, e);
                return;
            }
            println!("上传成功: {} ({}环境)", config.output_dir, env);
        });
        handles.push(handle);
    }
        // 等待所有线程完成
    for handle in handles {
        handle.join().unwrap();
    }
    Ok(())
}

// 将目录打包成zip文件
fn zip_dir(zip: &mut ZipWriter<File>, src_dir: &str, options: FileOptions) -> Result<(), String> {
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
