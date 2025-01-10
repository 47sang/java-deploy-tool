mod build;
mod upload;
mod run;

use clap::{Arg, Command};
use build::build_java_project;
use upload::upload_jar;
use run::{run_jar, run_sh};

fn main() {

  let matches = Command::new("java-deploy-tool")
      .version("1.0")
      .author("Your Name <your.email@example.com>")
      .about("Deploys Java projects")
      .arg(Arg::new("project_dir")
           .short('p')
           .long("project-dir")
           .value_name("DIR")
           .help("Sets the Java project directory")
           .required(false)
           .default_value("."))
      .arg(Arg::new("server")
           .short('s')
           .long("server")
           .value_name("HOST")
           .help("Sets the server address")
           .required(false)
           .default_value("192.168.31.60:22"))
      .get_matches();

  let project_dir = matches.get_one::<String>("project_dir").unwrap();
  let server = matches.get_one::<String>("server").unwrap();

  // 打包 Java 项目
  // if let Err(e) = build_java_project(project_dir) {
  //     eprintln!("{}", e);
  //     return;
  // }

  // 上传 JAR 包
  // let jar_path1 = format!("{}/admin/target/admin.jar", project_dir);
  // let jar_path2 = format!("{}/client/target/client.jar", project_dir);
  // let jar_path3 = format!("{}/websocket/target/websocket.jar", project_dir);
  // let remote_path1 = "/opt/xinxuan1v1/admin.jar";
  // let remote_path2 = "/opt/xinxuan1v1/client.jar";
  // let remote_path3 = "/opt/xinxuan1v1/websocket.jar";
  // if let Err(e) = upload_jar(server, "root", "lykj", &jar_path1, remote_path1) {
  //     eprintln!("{}", e);
  //     return;
  // }
  // if let Err(e) = upload_jar(server, "root", "lykj", &jar_path2, remote_path2) {
  //     eprintln!("{}", e);
  //     return;
  // }
  // if let Err(e) = upload_jar(server, "root", "lykj", &jar_path3, remote_path3) {
  //     eprintln!("{}", e);
  //     return;
  // }

  // 运行 JAR 包
  // if let Err(e) = run_jar(server, "root", "lykj", remote_path1) {
  //     eprintln!("{}", e);
  //     return;
  // }
  // if let Err(e) = run_jar(server, "root", "lykj", remote_path2) {
  //     eprintln!("{}", e);
  //     return;
  // }
  // if let Err(e) = run_jar(server, "root", "lykj", remote_path3) {
  //     eprintln!("{}", e);
  //     return;
  // }

  // 运行 sh 脚本
  if let Err(e) = run_sh(server, "root", "lykj", "/opt/xinxuan1v1/run2.sh") {
      eprintln!("{}", e);
      return;
  }

  println!("部署完成!");
}
