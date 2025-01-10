mod build;
mod config;
mod run;
mod upload;

use build::build_java_project;
use clap::{Arg, Command};
use config::DeployConfig;
use run::run_jar;
use std::thread;
use upload::upload_jar;

fn main() {
    let matches = Command::new("java-deploy-tool")
        .version("1.0")
        .author("Your Name <your.email@example.com>")
        .about("Deploys Java projects")
        .arg(
            Arg::new("project_dir")
                .short('p')
                .long("project-dir")
                .value_name("DIR")
                .help("Sets the Java project directory")
                .required(false)
                .default_value("."),
        )
        .arg(
            Arg::new("config")
                .short('c')
                .long("config")
                .value_name("CONFIG_FILE")
                .help("配置文件路径")
                .required(false)
                .default_value("config/deploy.toml"),
        )
        .arg(
            Arg::new("env")
                .short('e')
                .long("env")
                .value_name("ENVIRONMENT")
                .help("部署环境 (dev/test/prod)")
                .required(false)
                .default_value("dev"),
        )
        .arg(
            Arg::new("init-config")
                .long("init-config")
                .help("创建示例配置文件")
                .action(clap::ArgAction::SetTrue),
        )
        // 保留原有的参数，用于命令行覆盖配置文件的值
        .arg(
            Arg::new("server")
                .short('s')
                .long("server")
                .value_name("HOST")
                .help("Sets the server address")
                .required(false),
        )
        .arg(
            Arg::new("username")
                .short('u')
                .long("username")
                .value_name("USERNAME")
                .help("Sets the server username")
                .required(false),
        )
        .arg(
            Arg::new("password")
                .short('w')
                .long("password")
                .value_name("PASSWORD")
                .help("Sets the server password")
                .required(false),
        )
        .arg(
            Arg::new("java_path")
                .short('j')
                .long("java-path")
                .value_name("JAVA_PATH")
                .help("Sets the Java executable path")
                .required(false),
        )
        .arg(
            Arg::new("remote_base_path")
                .short('r')
                .long("remote-base-path")
                .value_name("REMOTE_PATH")
                .help("Sets the remote base path for deployment")
                .required(false),
        )
        .get_matches();

    let config_path = matches.get_one::<String>("config").unwrap();
    
    // 如果指定了init-config参数，创建示例配置文件并退出
    if matches.get_flag("init-config") {
        match DeployConfig::create_example_config(config_path) {
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

    let project_dir = matches.get_one::<String>("project_dir").unwrap();
    let environment = matches.get_one::<String>("env").unwrap();
    
    // 从配置文件加载环境配置
    let mut config = match DeployConfig::from_file(config_path, environment) {
        Ok(config) => config,
        Err(e) => {
            eprintln!("加载配置失败: {}", e);
            eprintln!("提示: 使用 --init-config 参数可以创建示例配置文件");
            return;
        }
    };
    
    // 命令行参数覆盖配置文件的值
    if let Some(server) = matches.get_one::<String>("server") {
        config.server = server.clone();
    }
    if let Some(username) = matches.get_one::<String>("username") {
        config.username = username.clone();
    }
    if let Some(password) = matches.get_one::<String>("password") {
        config.password = password.clone();
    }
    if let Some(java_path) = matches.get_one::<String>("java_path") {
        config.java_path = java_path.clone();
    }
    if let Some(remote_base_path) = matches.get_one::<String>("remote_base_path") {
        config.remote_base_path = remote_base_path.clone();
    }

    if let Err(e) = build_java_project(project_dir) {
        eprintln!("{}", e);
        return;
    }

    // 定义所有需要部署的 JAR 包
    let deployments = vec![
        "admin.jar",
        "client.jar",
        "websocket.jar",
    ];

    // 创建线程处理每个 JAR 包的上传和运行
    let mut handles = vec![];

    for jar_name in deployments {
        let config = config.clone();
        let project_dir = project_dir.clone();
        let environment = environment.clone();
        let handle = thread::spawn(move || {
            let jar_path = format!("{}/{}/target/{}", project_dir, jar_name.split(".").next().unwrap(), jar_name);
            let remote_path = format!("{}/{}", config.remote_base_path, jar_name);

            // 上传 JAR 包
            if let Err(e) = upload_jar(&config.server, &config.username, &config.password, &jar_path, &remote_path) {
                eprintln!("上传失败 {}: {}", jar_name, e);
                return;
            }
            println!("上传成功: {}", jar_name);

            // 运行 JAR 包
            if let Err(e) = run_jar(&config.server, &config.username, &config.password, &remote_path, &config.java_path, &environment) {
                eprintln!("运行失败 {}: {}", jar_name, e);
                return;
            }
            println!("运行成功: {}", jar_name);
        });
        handles.push(handle);
    }

    // 等待所有线程完成
    for handle in handles {
        handle.join().unwrap();
    }
}
