use crate::compiler::{build_java_project, build_vue_project, zip_dir};
use crate::config::DeployConfig;
use crate::upload;
use anyhow::Result;
use indicatif::MultiProgress;
use serde_json::Value;
use std::fs::File;
use std::thread;
use zip::write::FileOptions;
use zip::{CompressionMethod, ZipWriter};

/// 部署Java项目的函数
pub fn deploy_java_project(
    project_dir: &str,
    config_path: &str,
    environments: &[String],
    models: &[String],
    upload_only: bool,
) -> Result<()> {
    // 构建Java项目
    build_java_project(project_dir)?;

    // 为每个环境创建部署任务
    let mut handles = vec![];
    let multi_progress = MultiProgress::new();

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

        // 处理jar_files字段，根据类型确定是单模块还是多模块
        match &config.jar_files {
            Value::Array(jar_array) => {
                // 多模块项目情况
                for jar_value in jar_array {
                    if let Value::String(jar_name) = jar_value {
                        // 应用命令行参数覆盖
                        let config = config.clone();

                        if !models.is_empty()
                            && !models.contains(
                                &jar_name
                                    .split('.')
                                    .next()
                                    .expect("配置文件中jar_name格式错误,无法匹配模块名称")
                                    .to_string(),
                            )
                        {
                            println!("{}模块不参与部署", jar_name);
                            continue;
                        }

                        let project_dir = project_dir.to_string();
                        let env = env.clone();
                        // 多模块项目，获取编译产物路径
                        let jar_path = format!(
                            "{}/{}/target/{}",
                            project_dir,
                            jar_name.split('.').next().unwrap(),
                            jar_name
                        );

                        spawn_deploy_thread(
                            jar_name,
                            jar_path,
                            config.clone(),
                            env,
                            upload_only,
                            &mut handles,
                            &multi_progress,
                        );
                    }
                }
            }
            Value::String(jar_name) => {
                // 单模块项目情况
                let config = config.clone();

                if !models.is_empty()
                    && !models.contains(
                        &jar_name
                            .split('.')
                            .next()
                            .expect("配置文件中jar_name格式错误,无法匹配模块名称")
                            .to_string(),
                    )
                {
                    println!("{}模块不参与部署", jar_name);
                    continue;
                }

                let project_dir = project_dir.to_string();
                let env = env.clone();
                // 单模块项目，获取编译产物路径
                let jar_path = format!("{}/target/{}", project_dir, jar_name);

                spawn_deploy_thread(
                    jar_name,
                    jar_path,
                    config.clone(),
                    env,
                    upload_only,
                    &mut handles,
                    &multi_progress,
                );
            }
            _ => {
                eprintln!("配置文件中jar_files格式错误，必须是字符串或字符串数组");
                continue;
            }
        }
    }

    // 等待所有线程完成
    for handle in handles {
        handle.join().unwrap();
    }

    Ok(())
}

// 创建并运行部署线程的辅助函数
fn spawn_deploy_thread(
    jar_name: &str,
    jar_path: String,
    config: DeployConfig,
    env: String,
    upload_only: bool,
    handles: &mut Vec<thread::JoinHandle<()>>,
    multi_progress: &MultiProgress,
) {
    let jar_name = jar_name.to_string();
    let multi_progress = multi_progress.clone();

    let handle = thread::spawn(move || {
        let remote_path = format!("{}/{}", config.remote_base_path, jar_name);

        // 命令行参数优先，如果命令行没有指定则使用配置文件中的设置
        let final_upload_only = if upload_only {
            // 如果命令行指定了 --upload-only，则使用命令行的值
            true
        } else {
            // 如果命令行没有指定 --upload-only，则使用配置文件中的值
            config.upload_only
        };

        if final_upload_only {
            println!("开始上传 {} 到 {} 环境", jar_name, env);
            // 仅上传文件，不执行任何命令
            if let Err(e) = upload::upload_jar_only(
                &config.server,
                &config.username,
                &config.password,
                &jar_path,
                &remote_path,
                Some(&multi_progress),
            ) {
                eprintln!("上传失败 {} ({}环境): {}", jar_name, env, e);
                return;
            }
            println!("上传成功: {} ({}环境)", jar_name, env);
        } else {
            println!("开始部署 {} 到 {} 环境", jar_name, env);
            // 上传并运行 JAR 包
            if let Err(e) = upload::upload_and_run_jar(
                &config.server,
                &config.username,
                &config.password,
                &jar_path,
                &remote_path,
                &config.java_path,
                &env,
                Some(&multi_progress),
            ) {
                eprintln!("部署失败 {} ({}环境): {}", jar_name, env, e);
                return;
            }
            println!("部署成功: {} ({}环境)", jar_name, env);
        }
    });
    handles.push(handle);
}

/// 部署Vue项目的函数
pub fn deploy_vue_project(
    project_dir: &str,
    config_path: &str,
    environments: &[String],
    upload_only: bool,
) -> Result<()> {
    // 为每个环境创建部署任务
    let mut handles = vec![];
    let multi_progress = MultiProgress::new();

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

        let multi_progress = multi_progress.clone();
        let handle = thread::spawn(move || {
            // 构建Vue项目
            build_vue_project(&project_dir, &config.scripts).expect("构建Vue项目失败");

            // 压缩产出目录文件zip
            let output_dir = format!("{}/{}", project_dir, config.output_dir);
            let zip_path = format!("{}/{}.zip", project_dir, config.output_dir);
            let zip_file = File::create(&zip_path).expect("创建新的zip文件失败");
            let mut zip = ZipWriter::new(zip_file);
            let options = FileOptions::default().compression_method(CompressionMethod::Deflated);
            zip_dir(&mut zip, &output_dir, options).expect("压缩失败");
            zip.finish().expect("完成ZIP文件失败");

            // 上传zip文件
            let remote_path = format!("{}/{}", config.remote_base_path, config.output_dir);

            // 命令行参数优先，如果命令行没有指定则使用配置文件中的设置
            let final_upload_only = if upload_only {
                // 如果命令行指定了 --upload-only，则使用命令行的值
                true
            } else {
                // 如果命令行没有指定 --upload-only，则使用配置文件中的值
                config.upload_only
            };

            if final_upload_only {
                // 仅上传文件，不执行解压等命令
                if let Err(e) = upload::upload_zip_only(
                    &config.server,
                    &config.username,
                    &config.password,
                    &zip_path,
                    &remote_path,
                    Some(&multi_progress),
                ) {
                    eprintln!("上传失败 {} ({}环境): {}", config.output_dir, env, e);
                    return;
                }
                println!("上传成功: {} ({}环境)", config.output_dir, env);
            } else {
                // 上传并解压
                if let Err(e) = upload::upload_file(
                    &config.server,
                    &config.username,
                    &config.password,
                    &zip_path,
                    &remote_path,
                    Some(&multi_progress),
                ) {
                    eprintln!("上传失败 {} ({}环境): {}", config.output_dir, env, e);
                    return;
                }
                println!("上传成功: {} ({}环境)", config.output_dir, env);
            }
        });
        handles.push(handle);
    }
    // 等待所有线程完成
    for handle in handles {
        handle.join().unwrap();
    }
    Ok(())
}
