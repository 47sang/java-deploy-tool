#[derive(Clone)]
pub struct DeployConfig {
    pub server: String,
    pub username: String,
    pub password: String,
    pub java_path: String,
    pub remote_base_path: String,
}

impl Default for DeployConfig {
    fn default() -> Self {
        Self {
            server: String::from("192.168.31.60:22"),
            username: String::from("root"),
            password: String::from("lykj"),
            java_path: String::from("/opt/soft/zulu11/bin/java"),
            remote_base_path: String::from("/opt/xinxuan1v1"),
        }
    }
}


impl DeployConfig {
    #[allow(dead_code)]
    pub fn new(
        server: String,
        username: String,
        password: String,
        java_path: String,
        remote_base_path: String,
    ) -> Self {
        Self {
            server,
            username,
            password,
            java_path,
            remote_base_path,
        }
    }
} 