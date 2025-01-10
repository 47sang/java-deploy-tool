use ssh2::Session;
use std::net::TcpStream;
use std::path::Path;
use std::io::Write;
use std::fs;

fn bytes_to_mb(bytes: u64) -> f64 {
    bytes as f64 / (1024.0 * 1024.0)
}

pub fn upload_jar(server: &str, username: &str, password: &str, local_path: &str, remote_path: &str) -> Result<(), String> {
    // 获取文件大小
    let file_size = fs::metadata(local_path)
        .map_err(|e| format!("无法获取文件大小: {}", e))?
        .len();

    println!("开始上传文件: {} (大小: {:.2} MB)", local_path, bytes_to_mb(file_size));

    let tcp = TcpStream::connect(server).map_err(|e| format!("连接失败: {}", e))?;
    let mut sess = Session::new().map_err(|e| format!("创建会话失败: {}", e))?;
    sess.set_tcp_stream(tcp);
    sess.handshake().map_err(|e| format!("握手失败: {}", e))?;
    sess.userauth_password(username, password)
        .map_err(|e| format!("认证失败: {}", e))?;

    let data = fs::read(local_path).map_err(|e| format!("读取本地文件失败: {}", e))?;
    if data.len() as u64 != file_size {
        return Err(format!("文件读取不完整: 预期大小 {:.2} MB, 实际读取 {:.2} MB", 
            bytes_to_mb(file_size), 
            bytes_to_mb(data.len() as u64)));
    }

    let mut remote_file = sess.scp_send(Path::new(remote_path), 0o644, file_size, None)
        .map_err(|e| format!("创建远程文件失败: {}", e))?;
    
    remote_file.write_all(&data).map_err(|e| format!("写入远程文件失败: {}", e))?;
    remote_file.send_eof().map_err(|e| format!("发送EOF失败: {}", e))?;
    remote_file.wait_eof().map_err(|e| format!("等待EOF失败: {}", e))?;
    remote_file.close().map_err(|e| format!("关闭远程文件失败: {}", e))?;
    remote_file.wait_close().map_err(|e| format!("等待远程文件关闭失败: {}", e))?;

    println!("JAR 文件上传成功! {} -> {} (大小: {:.2} MB)", 
        local_path, 
        remote_path, 
        bytes_to_mb(file_size));
    Ok(())
}