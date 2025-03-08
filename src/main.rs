mod build;
mod config;
mod run;
mod upload;

use build::build_java_project;
use clap::{Arg, Command};
use config::DeployConfig;
use run::run_jar;
use std::thread;
use std::time::{Duration, Instant};
use upload::upload_jar;

fn main() {
    let matches = Command::new("java-deploy-tool")
        .version("1.0")
        .author("士钰 <zhoushiyu92@gmail.com>")
        .about("一键部署Java项目,支持多环境部署,支持多模块部署")        
        .arg(
            Arg::new("env")
                .short('e')
                .long("env")
                .value_name("ENVIRONMENT")
                .help("部署环境，多个环境用逗号分隔 (例如: dev,prod)")
                .required(false)
                .default_value("dev")
                .value_delimiter(','),
        )
        .arg(
            Arg::new("init-config")
                .long("init-config")
                .help("创建示例配置文件")
                .action(clap::ArgAction::SetTrue),
        )
        // 这里已移除命令行覆盖配置文件的参数
        .get_matches();

    // 调用方法并测量执行时间
    measure_execution_time(|| {
        println!("开始执行脚本程序");
        let config_path = "./deploy.toml".to_string();

        // 如果指定了init-config参数，创建示例配置文件并退出
        if matches.get_flag("init-config") {
            match DeployConfig::create_springboot_config(&config_path) {
                Ok(_) => {
                    println!("示例配置文件已创建: {}", config_path);
                    println!("请修改配置文件中的值后再运行部署。");
                    return;
                }
                Err(e) => {
                    eprintln!("创建配置文件失败: {}", e);
                    return;
                }
            }
        }

        let project_dir = ".".to_string();
        let environments: Vec<String> = matches
            .get_many::<String>("env")
            .unwrap()
            .map(|s| s.to_string())
            .collect();

        // 从命令行获取可能的覆盖值
        let config_path = config_path.to_string();

        // 构建Java项目
        if let Err(e) = build_java_project(&project_dir) {
            eprintln!("{}", e);
            return;
        }

        // 为每个环境创建部署任务
        let mut handles = vec![];

        for env in &environments {
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
                let project_dir = project_dir.clone();
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
    });
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
