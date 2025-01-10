use ssh2::Session;
use std::net::TcpStream;
use std::io::Read;

pub fn run_jar(server: &str, username: &str, password: &str, jar_path: &str) -> Result<(), String> {
  let tcp = TcpStream::connect(server).map_err(|e| format!("Failed to connect: {}", e))?;
  let mut sess = Session::new().map_err(|e| format!("Failed to create session: {}", e))?;
  sess.set_tcp_stream(tcp);
  sess.handshake().map_err(|e| format!("Handshake failed: {}", e))?;
  sess.userauth_password(username, password)
      .map_err(|e| format!("Authentication failed: {}", e))?;

  let mut channel = sess.channel_session().map_err(|e| format!("Failed to open channel: {}", e))?;
  channel.exec(&format!("/opt/soft/zulu11/bin/java -jar {}", jar_path))
      .map_err(|e| format!("Failed to execute command: {}", e))?;

  let mut output = String::new();
  channel.read_to_string(&mut output).map_err(|e| format!("Failed to read output: {}", e))?;
  println!("Command output: {}", output);

  Ok(())
}

pub fn run_sh(server: &str, username: &str, password: &str, sh_path: &str) -> Result<(), String> {
  let tcp = TcpStream::connect(server).map_err(|e| format!("Failed to connect: {}", e))?;
  let mut sess = Session::new().map_err(|e| format!("Failed to create session: {}", e))?;
  sess.set_tcp_stream(tcp);
  sess.handshake().map_err(|e| format!("Handshake failed: {}", e))?;
  sess.userauth_password(username, password)
      .map_err(|e| format!("Authentication failed: {}", e))?;

  let mut channel = sess.channel_session().map_err(|e| format!("Failed to open channel: {}", e))?;
  channel.exec(&format!("/bin/bash {}", sh_path))
      .map_err(|e| format!("Failed to execute command: {}", e))?;

  let mut output = String::new();
  channel.read_to_string(&mut output).map_err(|e| format!("Failed to read output: {}", e))?;
  println!("Command output: {}", output);

  Ok(())
}
