package influxdb

import (
	"fmt"
	"github.com/3th1nk/easygo/util/logs"
	"net/http"
	"net/url"
	"strings"
)

func (this *Client) buildQueryUrl(db, rp, sql string) string {
	uri := this.addr + "/query?epoch=" + this.queryEpoch
	if db != "" {
		uri += "&db=" + url.QueryEscape(db)
	}
	if rp != "" {
		uri += "&rp=" + url.QueryEscape(rp)
	}
	if sql != "" {
		uri += "&q=" + url.QueryEscape(sql)
	}
	if this.username != "" {
		uri += "&u=" + url.QueryEscape(this.username)
	}
	if this.password != "" {
		uri += "&p=" + url.QueryEscape(this.password)
	}
	return uri
}

type queryResp struct {
	Results []*struct {
		StatementId int       `json:"statement_id,omitempty"`
		Error       string    `json:"error,omitempty"`
		Series      []*Series `json:"series,omitempty"`
	} `json:"results"`
}

// RawQuery 执行查询语句
//	- db: 数据库名，也可以在sql语句中指定数据库
//	- rp: 保留策略名，为空时使用默认
//	- sql: 查询语句，语法参考：https://docs.influxdata.com/influxdb/v1/query_language/explore-data/#the-basic-select-statement
func (this *Client) RawQuery(db, rp, sql string) ([]*Series, error) {
	queryUrl := this.buildQueryUrl(db, rp, sql)
	var resp queryResp
	resBody, err := doRequest(http.MethodGet, queryUrl, "", nil, &resp)
	if err != nil {
		if logs.IsErrorEnable(this.logger) {
			this.logger.Error("[InfluxDB] url=%v, err=%v, resp=%v", queryUrl, err.Error(), string(resBody))
		}
		return nil, err
	}
	if len(resp.Results) != 0 && resp.Results[0] != nil {
		if resp.Results[0].Error != "" {
			this.logger.Error("[InfluxDB] url=%v, err=%v", queryUrl, resp.Results[0].Error)
			return nil, fmt.Errorf(resp.Results[0].Error)
		}
		return resp.Results[0].Series, nil
	}
	return []*Series{}, nil
}

type Query struct {
	client       *Client
	selects      []string
	db           string
	rp           string
	measurements []string
	cond         ICond
	groupBy      string
	orderBy      string
	limit        string
	tz           string
}

func (this *Client) NewQuery() *Query {
	return &Query{
		client:  this,
		selects: []string{"*"},
		cond:    condEmpty{},
		tz:      "Asia/Shanghai",
	}
}

func (this *Query) Select(fields ...string) *Query {
	if len(fields) > 0 {
		this.selects = fields
	}
	return this
}

// From 设置查询的来源
//	- db: 数据库名，必须指定
//	- rp: 保留策略名，为空时使用默认
//	- measurements: 表名，必须指定，可以多个
func (this *Query) From(db, rp string, measurements ...string) *Query {
	this.db, this.rp, this.measurements = db, rp, measurements
	return this
}

func (this *Query) Where(cond ICond) *Query {
	this.cond = this.cond.And(cond)
	return this
}

// GroupBy 设置分组列
//	注意：内部会自动添加`GROUP BY`前缀，外部需要处理双引号
func (this *Query) GroupBy(groupBy string) *Query {
	this.groupBy = " GROUP BY " + groupBy
	return this
}

// OrderBy 设置排序列
//	注意：内部会自动添加`ORDER BY`前缀，外部需要处理双引号
func (this *Query) OrderBy(orderBy string) *Query {
	this.orderBy = " ORDER BY " + orderBy
	return this
}

func (this *Query) Asc(field string) *Query {
	this.orderBy = " ORDER BY " + QuoteIfNeed(field) + " ASC"
	return this
}

func (this *Query) Desc(field string) *Query {
	this.orderBy = " ORDER BY " + QuoteIfNeed(field) + " DESC"
	return this
}

func (this *Query) Limit(limit int) *Query {
	this.limit = fmt.Sprintf(" LIMIT %d", limit)
	return this
}

func (this *Query) TimeZone(tz string) *Query {
	this.tz = tz
	return this
}

func (this *Query) from() string {
	arr := make([]string, 0, len(this.measurements))
	for _, measurement := range this.measurements {
		arr = append(arr, Quote(measurement))
	}
	return strings.Join(arr, ",")
}

func (this *Query) verify() error {
	if this.db == "" || len(this.measurements) == 0 {
		return fmt.Errorf("missing database or measurement")
	}
	return nil
}

func (this *Query) String() string {
	fields := make([]string, 0, len(this.selects))
	for _, field := range this.selects {
		if field == "*" || usingFunction(field) {
			fields = append(fields, field)
		} else {
			fields = append(fields, QuoteIfNeed(field))
		}
	}

	sql := "SELECT " + strings.Join(fields, ",") + " FROM " + this.from()
	if this.cond.IsValid() {
		sql += " WHERE " + trimParentheses(this.cond.String())
	}
	sql += this.groupBy
	sql += this.orderBy
	sql += this.limit
	sql += " TZ('" + this.tz + "')"
	return sql
}

func (this *Query) Do() ([]*Series, error) {
	if err := this.verify(); err != nil {
		return nil, err
	}

	return this.client.RawQuery(this.db, this.rp, this.String())
}
