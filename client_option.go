package influxdb

import (
	"crypto/tls"
	"net/http"
	"runtime"
	"time"

	"github.com/3th1nk/easygo/util/logs"
	"github.com/modern-go/reflect2"
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

func WithCredential(username, passOrToken string) Option {
	return func(c *Client) {
		c.username = username
		c.password = passOrToken
	}
}

// WithHTTPClient 使用自定义的 http.Client，会整体替换默认的 transport/超时/TLS 配置。
//
//	适合需要完全掌控连接池、代理、TLS 等的场景；传入 nil 时忽略。
func WithHTTPClient(hc *http.Client) Option {
	return func(c *Client) {
		if hc != nil {
			c.httpClient = hc
		}
	}
}

// WithHTTPTimeout 设置 HTTP 请求超时，默认 1m。
//
//	仅对默认 transport 生效；若已通过 WithHTTPClient 替换整个 client，应直接在其上设置 Timeout。
func WithHTTPTimeout(d time.Duration) Option {
	return func(c *Client) {
		if d > 0 {
			c.httpClient.Timeout = d
		}
	}
}

// WithTLSConfig 自定义 TLS 配置。默认会跳过证书校验，需要启用严格校验时传入对应的 *tls.Config。
//
//	仅对默认 transport 生效；若已通过 WithHTTPClient 替换整个 client，应直接在其 transport 上设置 TLSClientConfig。
func WithTLSConfig(tlsConf *tls.Config) Option {
	return func(c *Client) {
		if t, ok := c.httpClient.Transport.(*http.Transport); ok && t != nil {
			t.TLSClientConfig = tlsConf.Clone()
		}
	}
}

// WithInsecureSkipVerify 设置是否跳过 TLS 证书校验，默认跳过（true）。
//
//	生产环境建议传 false 启用严格校验。仅对默认 transport 生效；若已通过 WithHTTPClient 替换整个 client，
//	应直接在其 transport 的 TLSClientConfig 上设置 InsecureSkipVerify。
func WithInsecureSkipVerify(skip bool) Option {
	return func(c *Client) {
		if t, ok := c.httpClient.Transport.(*http.Transport); ok && t != nil {
			if t.TLSClientConfig == nil {
				t.TLSClientConfig = &tls.Config{}
			}
			t.TLSClientConfig.InsecureSkipVerify = skip
		}
	}
}

// WithMaxIdleConnsPerHost 设置每个 host 的最大空闲连接数，默认 100。
//
//	仅对默认 transport 生效；若已通过 WithHTTPClient 替换整个 client，应直接在其 transport 上设置。
//	若 transport 不是 *http.Transport，本 Option 静默失效。
func WithMaxIdleConnsPerHost(n int) Option {
	return func(c *Client) {
		if n > 0 {
			if t, ok := c.httpClient.Transport.(*http.Transport); ok && t != nil {
				t.MaxIdleConnsPerHost = n
			}
		}
	}
}
