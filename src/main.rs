mod compiler;
mod config;
mod deploy;
mod upload;

use clap::{Arg, Command};
use config::DeployConfig;
use deploy::{deploy_java_project, deploy_vue_project};
use std::time::{Duration, Instant};

fn main() {
    let matches = Command::new("deploy-tool")
        .version("1.4")
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
            Arg::new("model")
                .short('m')
                .long("model")
                .value_name("MODEL")
                .help("部署jar模块，多个模块用逗号分隔 (例如: admin,client,websocket)")
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
        .arg(
            Arg::new("upload-only")
                .short('u')
                .long("upload-only")
                .help("仅上传文件到服务器，不执行命令")
                .action(clap::ArgAction::SetTrue),
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

        let models: Vec<String> = matches
            .get_many::<String>("model")
            .unwrap_or_default()
            .map(|s| s.to_string())
            .collect();

        let upload_only = matches.get_flag("upload-only");

        println!("1.项目根目录: {}", project_dir);
        println!("2.后端环境: {:?}", environments);
        println!("3.web端环境: {:?}", vue_environments);
        println!("4.部署模块: {:?}", models);

        // 显示upload_only的实际配置情况
        if upload_only {
            println!("5.仅上传模式: {} (来自命令行参数)", upload_only);
        } else {
            println!("5.仅上传模式: (将根据各环境配置文件决定)");
            // 如果有环境参数，显示每个环境的upload_only配置
            if !environments.is_empty() || !vue_environments.is_empty() {
                let all_envs: std::collections::HashSet<String> = environments
                    .iter()
                    .chain(vue_environments.iter())
                    .cloned()
                    .collect();

                for env in &all_envs {
                    match DeployConfig::from_file(&config_path, env) {
                        Ok(config) => {
                            println!(
                                "   - {} 环境配置: upload_only = {}",
                                env, config.upload_only
                            );
                        }
                        Err(_) => {
                            println!("   - {} 环境配置: 读取失败", env);
                        }
                    }
                }
            }
        }

        // 根据命令行参数选择执行部署函数
        if !environments.is_empty() {
            println!("6.开始编译Java项目,请稍等...");
            // 部署Java项目
            if let Err(e) = deploy_java_project(
                &project_dir,
                &config_path,
                &environments,
                &models,
                upload_only,
            ) {
                eprintln!("部署失败: {:?}", e);
            }
        }

        if !vue_environments.is_empty() {
            println!("6.开始编译Vue项目,比较慢,请稍等...");
            // 部署Vue项目
            if let Err(e) =
                deploy_vue_project(&project_dir, &config_path, &vue_environments, upload_only)
            {
                eprintln!("部署失败: {:?}", e);
            }
        }
    });
}

/// 定义一个测量执行时间的函数
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
