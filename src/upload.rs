use ssh2::Session;
use std::io::{Read, Write};
use std::net::TcpStream;
use std::path::Path;
use std::time::{Duration, Instant};
use std::fs;
use indicatif::{ProgressBar, ProgressStyle};

/// 创建SSH会话最大重试次数
const MAX_RETRIES: u32 = 3;
/// 重试间隔(秒)
const RETRY_DELAY: Duration = Duration::from_secs(2);

/// 将字节转换为 MB
fn bytes_to_mb(bytes: u64) -> f64 {
    bytes as f64 / (1024.0 * 1024.0)
}

/// 格式化字节数为可读格式
fn format_bytes(bytes: u64) -> String {
    const UNITS: &[&str] = &["B", "KB", "MB", "GB", "TB"];
    let mut size = bytes as f64;
    let mut unit_index = 0;

    while size >= 1024.0 && unit_index < UNITS.len() - 1 {
        size /= 1024.0;
        unit_index += 1;
    }

    if unit_index == 0 {
        format!("{} {}", bytes, UNITS[unit_index])
    } else {
        format!("{:.1} {}", size, UNITS[unit_index])
    }
}

/// 创建SSH会话
fn create_ssh_session(server: &str, username: &str, password: &str) -> Result<Session, String> {
    let tcp = TcpStream::connect(server).map_err(|e| format!("ssh通信连接失败: {}", e))?;
    let mut sess = Session::new().map_err(|e| format!("创建ssh会话失败: {}", e))?;
    sess.set_tcp_stream(tcp);
    sess.handshake()
        .map_err(|e| format!("ssh通信握手失败: {}", e))?;
    sess.userauth_password(username, password)
        .map_err(|e| format!("ssh通信认证失败,可能密码错误: {}", e))?;

    Ok(sess)
}

/// 读取本地文件
fn read_local_file(local_path: &str) -> Result<(Vec<u8>, u64), String> {
    println!("读取本地文件: {}", local_path);
    let file_size = fs::metadata(local_path)
        .map_err(|e| format!("无法获取文件大小: {}", e))?
        .len();

    println!(
        "已读取的文件大小: {:.2} MB",
        bytes_to_mb(file_size)
    );

    let data = fs::read(local_path).map_err(|e| format!("读取本地文件失败: {}", e))?;
    if data.len() as u64 != file_size {
        return Err(format!(
            "文件读取不完整: 预期大小 {:.2} MB, 实际读取 {:.2} MB",
            bytes_to_mb(file_size),
            bytes_to_mb(data.len() as u64)
        ));
    }

    Ok((data, file_size))
}

/// 上传文件到远程服务器
fn upload_to_remote(
    sess: &Session,
    data: &[u8],
    file_size: u64,
    remote_path: &str,
) -> Result<(), String> {

    // 检查远程文件路径是否存在
    let check_path_cmd = format!("test -e {} && echo 'exists' || echo 'not exists'", remote_path);
    match execute_remote_command(sess, &check_path_cmd) {
        Ok(output) => {
            if output.trim() == "exists" {
                println!("远程文件已存在: {}", remote_path);
                // 删除已存在的文件
                let remove_cmd = format!("mv {} {}.bak", remote_path,remote_path);
                execute_remote_command(sess, &remove_cmd)
                    .map_err(|e| format!("标记远程文件为bak备份文件失败: {}", e))?;
                println!("已标记备份存在的文件,{}.bak",remote_path);
            }
            if output.trim() == "not exists"  {
                print!("远程文件不存在，或者路径错误，请检查配置文件remote_base_path属性是否正确: {}", remote_path)
            }
        }
        Err(e) => {
            return Err(format!("检查远程文件路径失败: {}", e));
        }
    }

    let mut remote_file = sess
        .scp_send(Path::new(remote_path), 0o644, file_size, None)
        .map_err(|e| format!("创建远程文件失败: {}", e))?;

    // 使用进度条写入器
    {
        let mut progress_writer = ProgressWriter::new(&mut remote_file, file_size);
        progress_writer
            .write_all(data)
            .map_err(|e| format!("写入远程文件失败: {}", e))?;
    }

    remote_file
        .send_eof()
        .map_err(|e| format!("发送EOF失败: {}", e))?;
    remote_file
        .wait_eof()
        .map_err(|e| format!("等待EOF失败: {}", e))?;
    remote_file
        .close()
        .map_err(|e| format!("关闭远程文件失败: {}", e))?;
    remote_file
        .wait_close()
        .map_err(|e| format!("等待远程文件关闭失败: {}", e))?;

    Ok(())
}

/// 在远程服务器执行命令并返回输出
fn execute_remote_command(sess: &Session, command: &str) -> Result<String, String> {
    let mut channel = sess
        .channel_session()
        .map_err(|e| format!("创建SSH通道失败: {}", e))?;

    println!("执行远程命令: {}", command);

    channel
        .exec(command)
        .map_err(|e| format!("执行远程命令失败: {}", e))?;

    // 读取命令输出
    let mut output = String::new();
    channel
        .read_to_string(&mut output)
        .map_err(|e| format!("读取命令输出失败: {}", e))?;

    channel
        .wait_close()
        .map_err(|e| format!("等待通道关闭失败: {}", e))?;

    // 检查命令退出状态
    let exit_status = channel
        .exit_status()
        .map_err(|e| format!("获取退出状态失败: {}", e))?;

    if exit_status != 0 {
        return Err(format!("远程命令执行失败，退出状态: {}", exit_status));
    }

    Ok(output)
}

/// 杀死远程服务器上的进程
fn kill_process(sess: &Session, jar_path: &str, env: &str) -> Result<(), String> {
    // 1. 先获取进程ID列表
    let find_pid_cmd = format!(
        "ps -ef | grep {} | grep -v grep | awk '{{print $2}}'",
        jar_path
    );
    let pids = execute_remote_command(sess, &find_pid_cmd)?;

    if pids.trim().is_empty() {
        // 没有找到进程，说明已经不存在
        println!("没有找到需要杀死的进程: {}", jar_path);
        return Ok(());
    }

    // 2. 根据部署环境，执行优雅关闭或者强制kill命令
    let kill_cmd = if env == "prod" {
        format!("kill {}", pids.trim())
    } else {
        format!("kill -9 {}", pids.trim())
    };
    let output = execute_remote_command(sess, &kill_cmd)?;

    if !output.trim().is_empty() {
        println!("杀死进程命令输出: {}", output);
    }

    // 3. 检查进程是否还存在
    std::thread::sleep(Duration::from_secs(1)); // 等待1秒让进程结束
    let check_cmd = format!(
        "ps -p {} > /dev/null 2>&1; echo $?",
        pids.trim().replace('\n', ",")
    );

    let mut success = false;
    for attempt in 0..MAX_RETRIES {
        if attempt > 0 {
            println!("检查进程状态 (第{}次重试)...", attempt);
            std::thread::sleep(Duration::from_secs(10) * (attempt + 1));
        }

        match execute_remote_command(sess, &check_cmd) {
            Ok(exit_code) => {
                if exit_code.trim() == "1" {
                    success = true;
                    println!("进程已成功杀死: {}", pids.trim());
                    break;
                }
            }
            Err(e) => {
                println!("检查进程状态失败: {}", e);
            }
        }
    }

    if !success {
        println!(
            "进程杀死失败，进程可能仍在运行或检查超过最大重试次数({}次)，执行强制杀死进程命令: {}",
            MAX_RETRIES,
            pids.trim()
        );

        // 直接发送 kill -9 命令
        let force_kill_cmd = format!("kill -9 {}", pids.trim());
        match execute_remote_command(sess, &force_kill_cmd) {
            Ok(output) => {
                if !output.trim().is_empty() {
                    println!("强制杀死命令输出: {}", output);
                }
                // 最后再检查一次
                if let Ok(final_check) = execute_remote_command(sess, &check_cmd) {
                    if final_check.trim() == "1" {
                        println!("强制杀死成功");
                    } else {
                        return Err(format!(
                            "最终进程检查失败，进程可能仍在运行: {}",
                            pids.trim()
                        ));
                    }
                }
            }
            Err(e) => {
                return Err(format!("强制杀死命令执行失败: {}", e));
            }
        }
    }

    Ok(())
}

/// 启动JAR包并检查进程状态
fn start_jar(sess: &Session, jar_path: &str, java_path: &str, env: &str) -> Result<(), String> {
    // 启动JAR包
    let start_cmd = format!(
        "nohup {} -jar {} --spring.profiles.active={} > /dev/null 2>&1 &",
        java_path, jar_path, env
    );

    execute_remote_command(sess, &start_cmd)?;

    // 等待一小段时间确保进程已启动
    std::thread::sleep(Duration::from_secs(2));

    // 检查进程是否成功启动
    let check_cmd = format!(
        "ps -ef | grep {} | grep -v grep | awk '{{print $2}}'",
        jar_path
    );

    let output = execute_remote_command(sess, &check_cmd)?;

    if output.trim().is_empty() {
        return Err(format!("程序启动失败: {}", jar_path));
    }

    println!(
        "程序已在后台成功启动: {},进程id {}",
        jar_path,
        output.trim()
    );
    Ok(())
}

/// 上传并运行 JAR 包（整合上传和运行功能）
pub fn upload_and_run_jar(
    server: &str,
    username: &str,
    password: &str,
    local_path: &str,
    remote_path: &str,
    java_path: &str,
    env: &str,
) -> Result<(), String> {
    // 读取本地文件
    let (data, file_size) = read_local_file(local_path)?;

    let sess = (0..MAX_RETRIES)
        .find_map(|attempt| {
            if attempt > 0 {
                println!("尝试重新创建SSH会话 (第{}次重试)...", attempt);
                std::thread::sleep(RETRY_DELAY);
            }
            create_ssh_session(server, username, password).ok()
        })
        .ok_or_else(|| format!("创建SSH会话失败，已达到最大重试次数({}次)", MAX_RETRIES))?;

    // 上传文件（带重试机制）
    let upload_progress = ProgressBar::new_spinner();
    upload_progress.set_style(
        ProgressStyle::default_spinner()
            .template("{spinner:.green} {msg}")
            .unwrap(),
    );

    (0..MAX_RETRIES)
        .find_map(|attempt| {
            if attempt > 0 {
                upload_progress.set_message(format!("尝试重新上传文件 (第{}次重试)...", attempt));
                std::thread::sleep(RETRY_DELAY);
            }
            match upload_to_remote(&sess, &data, file_size, remote_path) {
                Ok(_) => Some(()),
                Err(e) => {
                    upload_progress.set_message(format!("文件上传失败: {}，正在重试...", e));
                    None
                }
            }
        })
        .ok_or_else(|| {
            upload_progress.finish_with_message("文件上传失败，已达到最大重试次数");
            format!("文件上传失败，已达到最大重试次数({}次)", MAX_RETRIES)
        })?;

    upload_progress.finish_with_message(format!(
        "JAR 文件上传成功! {} -> {} (大小: {:.2} MB)",
        local_path,
        remote_path,
        bytes_to_mb(file_size)
    ));

    // 杀死已存在的进程
    (0..MAX_RETRIES)
        .find_map(|attempt| {
            if attempt > 0 {
                println!("尝试重新杀死进程 (第{}次重试)...", attempt);
                std::thread::sleep(RETRY_DELAY);
            }
            kill_process(&sess, remote_path, &env).ok()
        })
        .ok_or_else(|| format!("进程杀死失败，已达到最大重试次数({}次)", MAX_RETRIES))?;

    // 启动JAR包
    start_jar(&sess, remote_path, java_path, env)?;

    Ok(())
}

/// 上传zip文件
pub fn upload_file(
    server: &str,
    username: &str,
    password: &str,
    local_path: &str,
    remote_path: &str,
) -> Result<(), String> {
    // 读取本地文件
    let (data, file_size) = read_local_file(local_path)?;

    // 创建SSH会话
    let sess = create_ssh_session(server, username, password)?;

    // 构建远程zip路径
    let remote_zip_path = format!("{}.zip", remote_path);

    // 上传文件
    upload_to_remote(&sess, &data, file_size, &remote_zip_path)?;

    // 解压进度条
    let unzip_progress = ProgressBar::new_spinner();
    unzip_progress.set_style(
        ProgressStyle::default_spinner()
            .template("{spinner:.green} {msg}")
            .unwrap(),
    );
    unzip_progress.set_message("正在解压文件...");

    // 解压命令：先删除目标目录，然后解压zip文件
    // 使用-o选项覆盖现有文件，不提示
    let unzip_cmd = format!(
        "rm -rf {} && mkdir -p {} && cd {} && /usr/bin/unzip -o {}",
        remote_path, remote_path, remote_path, remote_zip_path
    );

    // 执行解压命令
    match execute_remote_command(&sess, &unzip_cmd) {
        Ok(_) => {
            unzip_progress.finish_with_message(format!(
                "文件上传并解压成功! {} -> {} (大小: {:.2} MB)",
                local_path,
                remote_zip_path,
                bytes_to_mb(file_size)
            ));
        }
        Err(e) => {
            unzip_progress.finish_with_message(format!("解压失败: {}", e));
            return Err(e);
        }
    }

    Ok(())
}





/// 自定义写入器，用于跟踪上传进度
struct ProgressWriter<'a> {
    inner: &'a mut dyn Write,
    progress_bar: ProgressBar,
    bytes_written: u64,
    start_time: Instant,
    last_update_time: Instant,
    last_bytes_written: u64,
}

impl<'a> ProgressWriter<'a> {
    fn new(inner: &'a mut dyn Write, total_size: u64) -> Self {
        let progress_bar = ProgressBar::new(total_size);
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

    /// 更新上传速率信息
    fn update_speed(&mut self) {
        let now = Instant::now();
        let elapsed_since_last_update = now.duration_since(self.last_update_time);

        // 每500毫秒更新一次速率显示，避免过于频繁的更新
        if elapsed_since_last_update.as_millis() >= 500 {
            let bytes_since_last_update = self.bytes_written - self.last_bytes_written;
            let elapsed_seconds = elapsed_since_last_update.as_secs_f64();

            // 计算当前速率 (bytes/second)
            let current_speed = if elapsed_seconds > 0.0 {
                bytes_since_last_update as f64 / elapsed_seconds
            } else {
                0.0
            };

            // 计算平均速率
            let total_elapsed = now.duration_since(self.start_time).as_secs_f64();
            let average_speed = if total_elapsed > 0.0 {
                self.bytes_written as f64 / total_elapsed
            } else {
                0.0
            };

            // 格式化速率信息
            let speed_msg = format!(
                "当前: {}/s | 平均: {}/s",
                format_bytes(current_speed as u64),
                format_bytes(average_speed as u64)
            );

            self.progress_bar.set_message(speed_msg);

            // 更新记录
            self.last_update_time = now;
            self.last_bytes_written = self.bytes_written;
        }
    }
}

impl<'a> Write for ProgressWriter<'a> {
    fn write(&mut self, buf: &[u8]) -> std::io::Result<usize> {
        let result = self.inner.write(buf);
        if let Ok(size) = result {
            self.bytes_written += size as u64;
            self.progress_bar.set_position(self.bytes_written);

            // 更新速率信息
            self.update_speed();
        }
        result
    }

    fn flush(&mut self) -> std::io::Result<()> {
        self.inner.flush()
    }
}

impl<'a> Drop for ProgressWriter<'a> {
    fn drop(&mut self) {
        // 计算最终的速率信息
        let now = Instant::now();
        let total_elapsed = now.duration_since(self.start_time).as_secs_f64();
        let final_average_speed = if total_elapsed > 0.0 {
            self.bytes_written as f64 / total_elapsed
        } else {
            0.0
        };

        // 组合最终消息：上传完成 + 最终速率信息
        let final_message = format!(
            "上传完成 | 平均速度: {} | 总用时: {:.1}s",
            format_bytes(final_average_speed as u64),
            total_elapsed
        );

        self.progress_bar.finish_with_message(final_message);
    }
}
