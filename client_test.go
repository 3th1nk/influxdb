package influxdb

import (
	"github.com/3th1nk/easygo/util/logs"
)

var (
	addr   = "http://172.16.20.250:20005"
	testDB = "test"
	testRP = &RetentionPolicy{
		Name:        "test_rp",
		Duration:    "7d",
		Replication: 1,
	}
	testClient *Client
)

func init() {
	testClient = NewClient(addr,
		WithLogger(logs.Stdout(logs.LevelAll)),
		WithDebugger(),
	)
}
