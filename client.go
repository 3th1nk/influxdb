package influxdb

import (
	"fmt"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/3th1nk/easygo/util/logs"
	"github.com/3th1nk/easygo/util/runtimeUtil"
	"github.com/panjf2000/ants/v2"
)

const (
	defaultFlushSize     = 1000
	defaultFlushInterval = 5 * time.Second

	// Close 时等待写协程池中已 Submit 任务完成的最大时长，超时后仍强制释放
	defaultCloseTimeout = 30 * time.Second

	stateNotRun   = 0
	stateRunning  = 1
	stateStopping = 2
)

type Client struct {
	addr            string // influxdb地址, 如：http://127.0.0.1:8086
	httpClient      *http.Client           // 每实例独立的 HTTP 客户端，默认 1m 超时 + 跳过 TLS 校验，可通过 Option 覆盖
	username        string // 用户名
	password        string // 密码 或 token
	mu              sync.RWMutex
	bucketGroups    map[string]*bucketGroup // key: db+rp  value: *bucketGroup
	groupSize       int                     // 每组桶的数量，默认16
	queryEpoch      string                  // 查询返回的时间格式, 默认s
	writePrecision  string                  // 写入数据的时间精度, 默认s
	writeSortTagKey bool                    // 写入数据时是否对tag key排序, 默认开启
	flushInterval   time.Duration           // 异步写入数据的时间间隔, 默认5s
	flushSize       int                     // 异步写入数据的行数，InfluxDB没有硬性限制单次写入行数，官方建议单次写入5000行
	stats           *WriteStats             // 常驻写入指标计数器
	writePoolSize   int                     //
	writePool       *ants.Pool              // 写协程池
	state           int32                   // (异步协程)运行状态 0:未运行 1:运行中 2:正在停止
	stopSignal      chan struct{}           // 信号
	closeOnce       sync.Once               // 保证 Close 幂等，避免重复 close channel / release pool 引发 panic
	logger          logs.Logger
}

func NewClient(addr string, opts ...Option) *Client {
	c := &Client{
		addr:            strings.TrimSuffix(addr, "/"),
		httpClient:      defaultHTTPClient(),
		bucketGroups:    make(map[string]*bucketGroup, 8),
		groupSize:       16,
		queryEpoch:      "s",
		writePrecision:  "s",
		writeSortTagKey: true,
		flushInterval:   defaultFlushInterval,
		flushSize:       defaultFlushSize,
		writePoolSize:   30,
		stopSignal:      make(chan struct{}, 1),
		logger:          logs.Default,
		stats:           &WriteStats{},
	}
	for _, opt := range opts {
		opt(c)
	}

	// 非阻塞模式：池满时 Submit 立即返回 ErrPoolOverflow，
	// 由上层(Write 溢出路径丢弃并计数 / Flush 路径保留至下周期)各自处理，
	// 保证业务调用协程永远不会因写池排队而阻塞
	pool, err := ants.NewPool(c.writePoolSize, ants.WithNonblocking(true), ants.WithLogger(c))
	if err != nil {
		panic(fmt.Sprintf("[InfluxDB] 创建写协程池失败: %v", err))
	}
	c.writePool = pool

	c.startAsyncWrite()
	return c
}

// Close 关闭客户端，释放资源
//
//	！！！务必在程序退出时调用，否则可能会导致数据丢失！！！
//	幂等：重复调用安全，仅首次生效
func (this *Client) Close() {
	this.closeOnce.Do(func() {
		this.stopAsyncWrite()
		// 用 ReleaseTimeout 等待已 Submit 的存量 doBatchWrite 任务完成；
		// 30s 仍未结束则强制释放，避免无限阻塞业务退出
		if err := this.writePool.ReleaseTimeout(defaultCloseTimeout); err != nil {
			this.logger.Warn("[InfluxDB] 释放写协程池超时(%v)，可能丢失少量在途数据: %v", defaultCloseTimeout, err)
		}
	})
}

func (this *Client) startAsyncWrite() {
	if atomic.LoadInt32(&this.state) == stateNotRun {
		go this.asyncWriter()
	}
}

func (this *Client) stopAsyncWrite() {
	atomic.StoreInt32(&this.state, stateStopping)
	this.stopSignal <- struct{}{}

	// 等待存量的写入任务完成
	for atomic.LoadInt32(&this.state) != stateNotRun {
		this.logger.Debug("[InfluxDB] 等待存量数据写入完成...")
		time.Sleep(time.Millisecond * 100)
	}
}

// Printf ants.Logger实现
func (this *Client) Printf(format string, args ...interface{}) {
	if logs.IsErrorEnable(this.logger) {
		this.logger.Error(format, args...)
	}
}

// Flush 强制刷新写入，立即将所有缓存的数据写入InfluxDB，一般无需手动调用
func (this *Client) Flush() {
	defer runtimeUtil.HandleRecover("Flush")

	this.mu.RLock()
	if len(this.bucketGroups) == 0 {
		this.mu.RUnlock()
		return
	}

	groups := make([]*bucketGroup, 0, len(this.bucketGroups))
	for _, group := range this.bucketGroups {
		groups = append(groups, group)
	}
	this.mu.RUnlock()

	for _, group := range groups {
		url := this.buildWriteUrl(group.Db(), group.Rp())
		group.Range(func(bck *bucket) {
			if bck.Len() == 0 {
				return
			}

			if err := this.writePool.Submit(func() {
				lines := bck.Pop()
				if len(lines) == 0 {
					return
				}

				_ = this.doBatchWrite(url, lines)
			}); err != nil {
				// 池满：Pop 尚未执行，数据仍在桶内，下个刷出周期会重试，不丢数据
				this.logger.Warn("[InfluxDB] 写协程池繁忙，本批数据保留至下个刷出周期")
			}
		})
	}
}

// asyncWriter 异步写入
func (this *Client) asyncWriter() {
	atomic.StoreInt32(&this.state, stateRunning)
	defer func() {
		atomic.StoreInt32(&this.state, stateNotRun)
	}()

	ticker := time.NewTicker(this.flushInterval)
	defer ticker.Stop()

	for {
		select {
		case <-this.stopSignal:
			this.logger.Debug("[InfluxDB] 存量数据开始写入")
			this.Flush()
			this.logger.Debug("[InfluxDB] 存量数据写入完成")
			return

		case <-ticker.C:
			this.logger.Debug("[InfluxDB] 定时批量写入数据")
			this.Flush()
		}
	}
}

// doBatchWrite 批量写入
//
//	单批失败时继续尝试后续分片，保证一次失败不会阻断剩余数据写入；最后返回最近一次错误
func (this *Client) doBatchWrite(writeUrl string, lines []string) error {
	if len(lines) == 0 {
		return nil
	}

	var lastErr error
	for i := 0; i < len(lines); i += this.flushSize {
		end := i + this.flushSize
		if end > len(lines) {
			end = len(lines)
		}

		data := strings.Join(lines[i:end], "\n")
		if logs.IsDebugEnable(this.logger) {
			this.logger.Debug("[InfluxDB] 写入数据 url=%v, line_count=%d", writeUrl, len(lines[i:end]))
		}
		resBody, err := this.doRequest(http.MethodPost, writeUrl, "", data, nil)
		if err != nil {
			this.countWrite(false)
			this.logger.Error("[InfluxDB] url=%v, err=%v, resp=%v", writeUrl, err.Error(), string(resBody))
			lastErr = err
			continue
		}
		this.countWrite(true)
	}
	return lastErr
}
