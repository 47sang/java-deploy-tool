use ssh2::Session;
use std::fs;
use std::io::{Read, Write};
use std::net::TcpStream;
use std::path::Path;
use std::time::Duration;

/// 将字节转换为 MB
fn bytes_to_mb(bytes: u64) -> f64 {
    bytes as f64 / (1024.0 * 1024.0)
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
    let file_size = fs::metadata(local_path)
        .map_err(|e| format!("无法获取文件大小: {}", e))?
        .len();

    println!(
        "开始上传文件: {} (大小: {:.2} MB)",
        local_path,
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
    let mut remote_file = sess
        .scp_send(Path::new(remote_path), 0o644, file_size, None)
        .map_err(|e| format!("创建远程文件失败: {}", e))?;

    remote_file
        .write_all(data)
        .map_err(|e| format!("写入远程文件失败: {}", e))?;
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
    channel.read_to_string(&mut output)
        .map_err(|e| format!("读取命令输出失败: {}", e))?;
    
    channel.wait_close()
        .map_err(|e| format!("等待通道关闭失败: {}", e))?;
    
    // 检查命令退出状态
    let exit_status = channel.exit_status()
        .map_err(|e| format!("获取退出状态失败: {}", e))?;
    
    if exit_status != 0 {
        return Err(format!("远程命令执行失败，退出状态: {}", exit_status));
    }

    Ok(output)
}

/// 杀死远程服务器上的进程
fn kill_process(sess: &Session, jar_path: &str) -> Result<(), String> {
    let kill_cmd = format!(
        "kill $(ps -ef | grep {} | grep -v grep | awk '{{print $2}}')",
        jar_path
    );
    
    let output = execute_remote_command(sess, &kill_cmd)?;
    
    if !output.trim().is_empty() {
        println!("杀死进程命令输出: {}", output);
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
    
    println!("程序已在后台成功启动: {},进程id {}", jar_path, output.trim());
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
    
    // 创建SSH会话（只创建一次会话用于所有操作）
    let sess = create_ssh_session(server, username, password)?;
    
    // 上传文件
    upload_to_remote(&sess, &data, file_size, remote_path)?;

    println!(
        "JAR 文件上传成功! {} -> {} (大小: {:.2} MB)",
        local_path,
        remote_path,
        bytes_to_mb(file_size)
    );
    
    // 杀死已存在的进程
    if let Err(e) = kill_process(&sess, remote_path) {
        println!("警告: 杀死旧进程失败: {}", e);
        // 继续执行，不返回错误
    }
    
    // 启动JAR包
    start_jar(&sess, remote_path, java_path, env)?;
    
    println!("{}环境JAR包部署和启动成功: {}",env, remote_path);
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

    // 解压命令：先删除目标目录，然后解压zip文件
    // 使用-o选项覆盖现有文件，不提示
    let unzip_cmd = format!(
        "rm -rf {} && mkdir -p {} && cd {} && /usr/bin/unzip -o {}",
        remote_path, remote_path, remote_path, remote_zip_path
    );
    
    // 执行解压命令
    execute_remote_command(&sess, &unzip_cmd)?;

    println!(
        "文件上传成功! {} -> {} (大小: {:.2} MB)",
        local_path,
        remote_zip_path,
        bytes_to_mb(file_size)
    );
    Ok(())
}
