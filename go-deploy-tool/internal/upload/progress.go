package upload

import "io"

// ProgressSink 把上传写入流接入多进度条展示。
//
// upload 包只依赖标准库 io，不绑定具体进度库：调用方传 nil 表示不显示进度；
// 单元测试可注入 mock 实现验证调用契约，避免依赖真实终端。
type ProgressSink interface {
	// Track 为一次总字节数为 total 的上传创建进度跟踪。
	//
	// 参数：
	//   - w     上传的目标写入器（通常是 SFTP 远程文件句柄）。
	//   - total 文件总字节数，用于计算百分比与剩余时间。
	//   - name  进度条行首标签，如 "admin.jar(dev)"。
	//
	// 返回值：
	//   - wrapped 应替代 w 供 io.Copy 写入，写入时同步推进进度；
	//   - finish  必须在上传结束后调用一次：err==nil 标记成功完成，
	//     err!=nil 中止该进度条，避免进度容器在收尾等待时永久阻塞。
	Track(w io.Writer, total int64, name string) (wrapped io.Writer, finish func(err error))
}
