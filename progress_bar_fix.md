# 进度条重叠问题修复

## 问题描述

在同时上传3个文件时，进度条显示异常：
- 前两个进度条显示出来但不更新状态
- 所有进度都堆叠到第三个进度条中
- 多个进度条相互干扰，导致显示混乱

## 问题原因

原有代码中，每个上传线程都独立创建 `ProgressBar` 实例，当多个线程同时执行时，这些进度条会在终端中相互干扰，导致显示混乱。

## 解决方案

使用 `indicatif::MultiProgress` 来管理多个进度条的并发显示：

### 1. 添加 MultiProgress 导入

```rust
use indicatif::{ProgressBar, ProgressStyle, MultiProgress};
```

### 2. 修改 ProgressWriter 构造函数

```rust
impl<'a> ProgressWriter<'a> {
    fn new(inner: &'a mut dyn Write, total_size: u64, multi_progress: Option<&MultiProgress>) -> Self {
        let progress_bar = if let Some(mp) = multi_progress {
            mp.add(ProgressBar::new(total_size))
        } else {
            ProgressBar::new(total_size)
        };
        
        progress_bar.set_style(
            ProgressStyle::default_bar()
                .template("{spinner:.green} [{elapsed_precise}] [{bar:40.cyan/blue}] {bytes}/{total_bytes} ({eta}) {msg}")
                .unwrap()
                .progress_chars("#>-"),
        );

        let now = Instant::now();
        ProgressWriter {
            inner,
            progress_bar,
            bytes_written: 0,
            start_time: now,
            last_update_time: now,
            last_bytes_written: 0,
        }
    }
}
```

### 3. 修改上传函数签名

所有上传函数都添加了 `multi_progress` 参数：

```rust
fn upload_to_remote(
    sess: &Session,
    data: &[u8],
    file_size: u64,
    remote_path: &str,
    multi_progress: Option<&MultiProgress>,
) -> Result<(), String>

pub fn upload_and_run_jar(
    server: &str,
    username: &str,
    password: &str,
    local_path: &str,
    remote_path: &str,
    java_path: &str,
    env: &str,
    multi_progress: Option<&MultiProgress>,
) -> Result<(), String>

pub fn upload_jar_only(
    server: &str,
    username: &str,
    password: &str,
    local_path: &str,
    remote_path: &str,
    multi_progress: Option<&MultiProgress>,
) -> Result<(), String>

pub fn upload_zip_only(
    server: &str,
    username: &str,
    password: &str,
    local_path: &str,
    remote_path: &str,
    multi_progress: Option<&MultiProgress>,
) -> Result<(), String>

pub fn upload_file(
    server: &str,
    username: &str,
    password: &str,
    local_path: &str,
    remote_path: &str,
    multi_progress: Option<&MultiProgress>,
) -> Result<(), String>
```

### 4. 修改主函数调用

在 `deploy_java_project` 和 `deploy_vue_project` 函数中创建 `MultiProgress` 实例：

```rust
fn deploy_java_project(
    project_dir: &str,
    config_path: &str,
    environments: &[String],
    models: &[String],
    upload_only: bool,
) -> Result<(), String> {
    // ... 构建Java项目代码 ...
    
    // 为每个环境创建部署任务
    let mut handles = vec![];
    let multi_progress = MultiProgress::new();

    for env in environments {
        // ... 配置处理代码 ...
        
        spawn_deploy_thread(jar_name, jar_path, config.clone(), env, upload_only, &mut handles, &multi_progress);
    }

    // 等待所有线程完成
    for handle in handles {
        handle.join().unwrap();
    }

    Ok(())
}
```

### 5. 修改线程生成函数

```rust
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
        // ... 处理逻辑 ...
        
        if final_upload_only {
            if let Err(e) = upload::upload_jar_only(
                &config.server,
                &config.username,
                &config.password,
                &jar_path,
                &remote_path,
                Some(&multi_progress),
            ) {
                // ... 错误处理 ...
            }
        } else {
            if let Err(e) = upload_and_run_jar(
                &config.server,
                &config.username,
                &config.password,
                &jar_path,
                &remote_path,
                &config.java_path,
                &env,
                Some(&multi_progress),
            ) {
                // ... 错误处理 ...
            }
        }
    });
    handles.push(handle);
}
```

## 效果

修复后的进度条将：
- 每个文件上传都有独立的进度条
- 多个进度条不会相互干扰
- 正确显示每个文件的上传进度
- 支持并发上传时的清晰进度展示

## 技术要点

1. **MultiProgress 管理**: 使用 `MultiProgress` 统一管理多个进度条
2. **线程安全**: 通过 `clone()` 将 `MultiProgress` 传递给各个线程
3. **可选参数**: 通过 `Option<&MultiProgress>` 保持向后兼容性
4. **进度条创建**: 通过 `mp.add(ProgressBar::new(total_size))` 创建受管理的进度条