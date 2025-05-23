package influxdb

import (
	"github.com/3th1nk/easygo/util/logs"
	"github.com/modern-go/reflect2"
	"runtime"
	"time"
)

type Option func(*Client)

func WithLogger(logger logs.Logger) Option {
	return func(c *Client) {
		if !reflect2.IsNil(logger) {
			c.logger = logger
		}
	}
}

func WithLoggerLevel(level int) Option {
	return func(c *Client) {
		c.logger.SetLevel(level)
	}
}

// WithGroupSize 设置每组桶的数量，默认16
func WithGroupSize(size int) Option {
	return func(c *Client) {
		if size <= 0 {
			size = 16
		}
		c.groupSize = size
	}
}

// WithWritePoolSize 设置最大写入并发
func WithWritePoolSize(n int) Option {
	return func(c *Client) {
		if n <= 0 {
			n = runtime.NumCPU()
		}
		c.writePoolSize = n
	}
}

// WithWritePrecision 设置写入数据的时间精度, 默认s, 可选值：ns, u, ms, s, m, h
func WithWritePrecision(precision string) Option {
	return func(c *Client) {
		c.writePrecision = precision
	}
}

// WithQueryEpoch 设置查询返回的时间格式, 默认s, 可选值：ns, u, ms, s, m, h
//
//	influxdb默认返回的时间是RFC3339格式，如果需要返回时间戳，需要通过epoch参数指定
func WithQueryEpoch(epoch string) Option {
	return func(c *Client) {
		c.queryEpoch = epoch
	}
}

func WithWriteSortTagKey(sort bool) Option {
	return func(c *Client) {
		c.writeSortTagKey = sort
	}
}

// WithFlushInterval 设置异步写入数据的时间间隔
func WithFlushInterval(interval time.Duration) Option {
	return func(c *Client) {
		if interval <= 0 {
			interval = defaultFlushInterval
		}
		c.flushInterval = interval
	}
}

// WithFlushSize 设置异步写入数据的行数上限, 最大不超过 bucketSize
func WithFlushSize(size int) Option {
	return func(c *Client) {
		if size <= 0 {
			size = defaultFlushSize
		} else if size > bucketSize {
			size = bucketSize
		}
		c.flushSize = size
	}
}

func WithDebugger() Option {
	return func(c *Client) {
		c.debugger = newDebugger()
	}
}

func WithCredential(username, passOrToken string) Option {
	return func(c *Client) {
		c.username = username
		c.password = passOrToken
	}
}
