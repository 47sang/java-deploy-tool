use ssh2::Session;
use std::io::Read;
use std::net::TcpStream;
use std::time::Duration;

/// 运行 JAR 包
pub fn run_jar(server: &str, username: &str, password: &str, jar_path: &str, java_path: &str, env: &str) -> Result<(), String> {
    let tcp = TcpStream::connect(server).map_err(|e| format!("ssh通信连接失败: {}", e))?;
    let mut sess = Session::new().map_err(|e| format!("创建ssh会话失败: {}", e))?;
    sess.set_tcp_stream(tcp);
    sess.handshake()
        .map_err(|e| format!("ssh通信握手失败: {}", e))?;
    sess.userauth_password(username, password)
        .map_err(|e| format!("认证失败,可能密码错误: {}", e))?;

    let mut channel = sess
        .channel_session()
        .map_err(|e| format!("打开通道失败: {}", e))?;

    // 杀死进程
    channel
        .exec(&format!(
            "kill $(ps -ef | grep {} | grep -v grep | awk '{{print $2}}')",
            jar_path
        ))
        .map_err(|e| format!("执行杀死进程命令失败: {}", e))?;

    let mut output = String::new();
    channel
        .read_to_string(&mut output)
        .map_err(|e| format!("读取输出失败: {}", e))?;
      
    if !output.trim().is_empty() {
        println!("杀死进程命令输出: {}", output);
    }

    // 创建新的channel运行jar
    let mut channel = sess
        .channel_session()
        .map_err(|e| format!("打开通道失败: {}", e))?;

    channel
        .exec(&format!("nohup {} -jar {} --spring.profiles.active={} > /dev/null 2>&1 &", java_path, jar_path, env))
        .map_err(|e| format!("执行运行jar命令失败: {}", e))?;

    // 等待一小段时间确保进程已启动
    std::thread::sleep(Duration::from_secs(2));

    // 检查进程是否成功启动
    let mut check_channel = sess
        .channel_session()
        .map_err(|e| format!("打开通道失败: {}", e))?;

    check_channel
        .exec(&format!("ps -ef | grep {} | grep -v grep | awk '{{print $2}}'", jar_path))
        .map_err(|e| format!("检查进程状态失败: {}", e))?;

    let mut output = String::new();
    check_channel
        .read_to_string(&mut output)
        .map_err(|e| format!("读取输出失败: {}", e))?;

    if output.trim().is_empty() {
        return Err(format!("程序启动失败: {}", jar_path));
    }

    println!("程序已在后台成功启动: {},进程id {}", jar_path, output.trim());
    Ok(())
}