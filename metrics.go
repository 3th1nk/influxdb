package influxdb

import (
	"fmt"
	"sync/atomic"
)

// WriteStats 写入统计
//
//	内部作为常驻原子计数器使用，外部通过 Client.WriteStats() 获取只读快照
type WriteStats struct {
	Total uint64 // 写入批次数（含成功与失败）
	Fail  uint64 // 写入失败批次数（HTTP 错误）
	Drop  uint64 // 因写协程池满被丢弃的批次数
}

func (s *WriteStats) incWrite(success bool) {
	atomic.AddUint64(&s.Total, 1)
	if !success {
		atomic.AddUint64(&s.Fail, 1)
	}
}

func (s *WriteStats) incDrop() {
	atomic.AddUint64(&s.Drop, 1)
}

func (s *WriteStats) snapshot() WriteStats {
	return WriteStats{
		Total: atomic.LoadUint64(&s.Total),
		Fail:  atomic.LoadUint64(&s.Fail),
		Drop:  atomic.LoadUint64(&s.Drop),
	}
}

func (s *WriteStats) reset() {
	atomic.StoreUint64(&s.Total, 0)
	atomic.StoreUint64(&s.Fail, 0)
	atomic.StoreUint64(&s.Drop, 0)
}

// String 以值接收器实现 Stringer，使得 WriteStats 值（如 Client.WriteStats() 返回值）
// 在 fmt 默认格式化时也会调用本方法；调用方应优先传入 Snapshot 副本，避免对常驻计数器直接调用
func (s WriteStats) String() string {
	return fmt.Sprintf("total: %d, fail: %d, drop: %d", s.Total, s.Fail, s.Drop)
}

func (this *Client) countWrite(success bool) {
	this.stats.incWrite(success)
}

func (this *Client) countDrop() {
	this.stats.incDrop()
}

// WriteStats 返回当前写入统计快照
func (this *Client) WriteStats() WriteStats {
	return this.stats.snapshot()
}

// ResetWriteStats 清零写入统计，配合周期性采集使用
func (this *Client) ResetWriteStats() {
	this.stats.reset()
}
