package influxdb

import (
	"bytes"
	"crypto/tls"
	"fmt"
	"github.com/3th1nk/easygo/util/convertor"
	"github.com/3th1nk/easygo/util/jsonUtil"
	"io"
	"net/http"
	"strings"
	"time"
)

// defaultHTTPClient 构建默认 HTTP 客户端，保留原有默认行为：
// 1 分钟超时、每 Host 100 个空闲连接、跳过 TLS 证书校验。
//
// 之所以从全局单例 + init() 改为按实例构建，是为了让每个 Client
// 都能独立配置超时、连接池与 TLS，互不影响。
func defaultHTTPClient() *http.Client {
	t := http.DefaultTransport.(*http.Transport).Clone()
	t.IdleConnTimeout = time.Minute
	t.MaxIdleConnsPerHost = 100
	t.TLSClientConfig = &tls.Config{InsecureSkipVerify: true}
	return &http.Client{
		Transport: t,
		Timeout:   time.Minute,
	}
}

func (this *Client) doRequest(method string, url, token string, body interface{}, r interface{}) ([]byte, error) {
	if method == "" {
		method = http.MethodPost
	}

	var b io.Reader = nil
	if body != nil {
		switch t := body.(type) {
		case string:
			b = strings.NewReader(t)
		case []byte:
			b = bytes.NewReader(t)
		case io.Reader:
			b = t
		default:
			bodyStr, err := convertor.ToString(body)
			if err != nil {
				return nil, fmt.Errorf("invalid request body: %v", err.Error())
			}
			b = bytes.NewBufferString(bodyStr)
		}
	}

	req, err := http.NewRequest(method, url, b)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "text/plain; charset=UTF-8")
	if token != "" {
		req.Header.Set("Authorization", "Token "+token)
	}

	resp, err := this.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	if resp != nil && resp.Body != nil {
		defer resp.Body.Close()
	}

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNoContent {
		return nil, fmt.Errorf("[%d]: %s", resp.StatusCode, http.StatusText(resp.StatusCode))
	}

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	if r != nil {
		return respBody, jsonUtil.Unmarshal(respBody, r)
	}

	return respBody, nil
}
