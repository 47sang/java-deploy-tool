use ssh2::Session;
use std::fs;
use std::io::{Read, Write};
use std::net::TcpStream;
use std::path::Path;

/// 将字节转换为 MB
fn bytes_to_mb(bytes: u64) -> f64 {
    bytes as f64 / (1024.0 * 1024.0)
}

/// 上传 JAR 包
pub fn upload_jar(
    server: &str,
    username: &str,
    password: &str,
    local_path: &str,
    remote_path: &str,
) -> Result<(), String> {
    // 获取文件大小
    let file_size = fs::metadata(local_path)
        .map_err(|e| format!("无法获取文件大小: {}", e))?
        .len();

    println!(
        "开始上传文件: {} (大小: {:.2} MB)",
        local_path,
        bytes_to_mb(file_size)
    );

    let tcp = TcpStream::connect(server).map_err(|e| format!("ssh通信连接失败: {}", e))?;
    let mut sess = Session::new().map_err(|e| format!("创建ssh会话失败: {}", e))?;
    sess.set_tcp_stream(tcp);
    sess.handshake()
        .map_err(|e| format!("ssh通信握手失败: {}", e))?;
    sess.userauth_password(username, password)
        .map_err(|e| format!("ssh通信认证失败,可能密码错误: {}", e))?;

    let data = fs::read(local_path).map_err(|e| format!("读取本地文件失败: {}", e))?;
    if data.len() as u64 != file_size {
        return Err(format!(
            "文件读取不完整: 预期大小 {:.2} MB, 实际读取 {:.2} MB",
            bytes_to_mb(file_size),
            bytes_to_mb(data.len() as u64)
        ));
    }

    let mut remote_file = sess
        .scp_send(Path::new(remote_path), 0o644, file_size, None)
        .map_err(|e| format!("创建远程文件失败: {}", e))?;

    remote_file
        .write_all(&data)
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

    println!(
        "JAR 文件上传成功! {} -> {} (大小: {:.2} MB)",
        local_path,
        remote_path,
        bytes_to_mb(file_size)
    );
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
    let file_size = fs::metadata(local_path)
        .map_err(|e| format!("无法获取文件大小: {}", e))?
        .len();

    println!(
        "开始上传文件: {} (大小: {:.2} MB)",
        local_path,
        bytes_to_mb(file_size)
    );

    let tcp = TcpStream::connect(server).map_err(|e| format!("ssh通信连接失败: {}", e))?;
    let mut sess = Session::new().map_err(|e| format!("创建ssh会话失败: {}", e))?;
    sess.set_tcp_stream(tcp);
    sess.handshake()
        .map_err(|e| format!("ssh通信握手失败: {}", e))?;
    sess.userauth_password(username, password)
        .map_err(|e| format!("ssh通信认证失败,可能密码错误: {}", e))?;

    let data = fs::read(local_path).map_err(|e| format!("读取本地文件失败: {}", e))?;
    if data.len() as u64 != file_size {
        return Err(format!(
            "文件读取不完整: 预期大小 {:.2} MB, 实际读取 {:.2} MB",
            bytes_to_mb(file_size),
            bytes_to_mb(data.len() as u64)
        ));
    }

    let remote_zip_path = format!("{}.zip", remote_path);
    let mut remote_file = sess
        .scp_send(Path::new(&remote_zip_path), 0o644, file_size, None)
        .map_err(|e| format!("创建远程文件失败: {}", e))?;

    remote_file
        .write_all(&data)
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

    let mut channel = sess
        .channel_session()
        .map_err(|e| format!("创建SSH通道失败: {}", e))?;

    // 解压命令：先删除目标目录，然后解压zip文件
    // 使用-o选项覆盖现有文件，不提示
    let unzip_cmd = format!(
        "rm -rf {} && mkdir -p {} && cd {} && /usr/bin/unzip -o {} && echo '解压完成，检查目录内容:' && ls -la",
        remote_path, remote_path, remote_path, remote_zip_path
    );
    
    println!("执行解压命令: {}", unzip_cmd);
    
    channel
        .exec(&unzip_cmd)
        .map_err(|e| format!("执行远程命令失败: {}", e))?;

    // 读取命令输出
    let mut output = Vec::new();
    channel.read_to_end(&mut output)
        .map_err(|e| format!("读取命令输出失败: {}", e))?;

    // let output_str = String::from_utf8_lossy(&output).to_string();

    // println!("解压命令输出: {}", output_str);
    
    channel.wait_close()
        .map_err(|e| format!("等待通道关闭失败: {}", e))?;
    
    // 检查命令退出状态
    let exit_status = channel.exit_status()
        .map_err(|e| format!("获取退出状态失败: {}", e))?;
    
    if exit_status != 0 {
        return Err(format!("解压命令执行失败，退出状态: {}", exit_status));
    }

    println!(
        "文件上传成功! {} -> {} (大小: {:.2} MB)",
        local_path,
        remote_zip_path,
        bytes_to_mb(file_size)
    );
    Ok(())
}
