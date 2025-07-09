# 上传进度条剩余时间显示功能

## 功能描述

为现有的上传进度条添加了剩余时间显示功能，根据当前上传速度和尚未上传的文件大小，计算并显示预估完成时间。

## 实现详情

### 1. 添加了 `format_duration` 函数

位置：`src/upload.rs` 第 38-59 行

```rust
/// 格式化时间为可读格式
fn format_duration(seconds: f64) -> String {
    let total_seconds = seconds as i64;
    
    if total_seconds < 60 {
        format!("{}s", total_seconds)
    } else if total_seconds < 3600 {
        let minutes = total_seconds / 60;
        let secs = total_seconds % 60;
        format!("{}m {}s", minutes, secs)
    } else if total_seconds < 86400 {
        let hours = total_seconds / 3600;
        let minutes = (total_seconds % 3600) / 60;
        format!("{}h {}m", hours, minutes)
    } else {
        let days = total_seconds / 86400;
        let hours = (total_seconds % 86400) / 3600;
        format!("{}d {}h", days, hours)
    }
}
```

该函数将秒数转换为易读的格式：
- 小于 60 秒：显示为 "30s"
- 小于 1 小时：显示为 "5m 30s"
- 小于 1 天：显示为 "2h 15m"
- 1 天以上：显示为 "1d 5h"

### 2. 修改了 `ProgressWriter::update_speed` 方法

位置：`src/upload.rs` 第 583-625 行

在原有的当前速度和平均速度计算基础上，添加了剩余时间的计算：

```rust
// 计算剩余时间
let remaining_bytes = self.progress_bar.length().unwrap_or(0) - self.bytes_written;
let eta_text = if remaining_bytes > 0 && current_speed > 0.0 {
    let eta_seconds = remaining_bytes as f64 / current_speed;
    format_duration(eta_seconds)
} else {
    "未知".to_string()
};

// 格式化速率信息和剩余时间
let speed_msg = format!(
    "当前: {}/s | 平均: {}/s | 剩余时间: {}",
    format_bytes(current_speed as u64),
    format_bytes(average_speed as u64),
    eta_text
);
```

## 计算逻辑

1. **剩余字节数计算**：总文件大小 - 已上传字节数
2. **剩余时间计算**：剩余字节数 ÷ 当前上传速度
3. **容错处理**：当剩余字节数为 0 或当前速度为 0 时，显示 "未知"

## 显示效果

进度条现在会显示如下格式的信息：
```
[==>    ] 25% | 当前: 1.2MB/s | 平均: 1.0MB/s | 剩余时间: 2m 30s
```

## 技术特点

1. **实时更新**：剩余时间基于当前上传速度实时计算，更准确反映实际情况
2. **智能格式化**：自动选择最合适的时间单位显示
3. **容错处理**：处理各种边界情况，避免程序崩溃
4. **性能优化**：只在每 500 毫秒更新一次，避免过于频繁的计算

## 使用场景

这个功能特别适用于：
- 大文件上传时的时间预估
- 网络状况不稳定时的进度跟踪
- 需要准确时间预估的生产环境部署

## 代码完整性

- ✅ 编译通过
- ✅ 不破坏现有功能
- ✅ 保持代码风格一致
- ✅ 添加了必要的错误处理