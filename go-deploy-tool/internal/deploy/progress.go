package deploy

import (
	"io"

	"deploy-tool/internal/upload"

	"github.com/vbauerster/mpb/v8"
	"github.com/vbauerster/mpb/v8/decor"
)

// mpbSink 是 upload.ProgressSink 的 mpb 实现：把每次上传接入一个共享的 *mpb.Progress
// 容器。多文件并发上传时，每条进度条各占独立一行、由容器统一刷新，互不交错
// （对标 Rust 版的 indicatif::MultiProgress）。
type mpbSink struct {
	p *mpb.Progress
}

// newMPBSink 用一个已有的 mpb 容器构造 sink。容器由 deploy 层统一创建，并贯穿一次
// 部署的所有并发 goroutine（每个上传 goroutine 共享同一个 sink，从而共享同一个容器）。
func newMPBSink(p *mpb.Progress) upload.ProgressSink {
	return mpbSink{p: p}
}

// Track 实现 upload.ProgressSink：
//   - 在容器中新增一条进度条，行首为 name，后接 百分比 / 已传·总量 / 平均速率 / 剩余时间；
//   - 用 bar.ProxyWriter 包装目标写入器，io.Copy 写入即推进进度；
//   - 返回 finish 句柄：上传失败时中止该 bar，避免容器 Wait 永久阻塞。
//
// 装饰器均基于 current/elapsed 计算（Average 系列），不依赖 Ewma 调用契约，
// 因此在 io.Copy 驱动的字节流推进模式下即可正确显示速率与剩余时间。
func (s mpbSink) Track(w io.Writer, total int64, name string) (io.Writer, func(error)) {
	bar := s.p.AddBar(total,
		mpb.PrependDecorators(
			decor.Name(name, decor.WCSyncSpace),
			decor.Percentage(decor.WCSyncSpace),
		),
		mpb.AppendDecorators(
			decor.CountersKibiByte("%.1f/%.1f", decor.WCSyncSpace),
			decor.AverageSpeed(decor.SizeB1024(0), "% .1f", decor.WCSyncSpace),
			decor.OnComplete(decor.AverageETA(decor.ET_STYLE_GO, decor.WCSyncSpace), "✓"),
		),
	)
	wrapped := bar.ProxyWriter(w)
	finish := func(err error) {
		if err != nil {
			bar.Abort(false) // 上传失败：中止该进度条，防止 p.Wait() 卡住
		}
		// 成功：current 已达 total，bar 自动 completed，无需额外操作
	}
	return wrapped, finish
}
