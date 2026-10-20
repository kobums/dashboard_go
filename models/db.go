package models

import (
	"fmt"
	"reflect"
	"strings"
	"dashboard/global/config"
	"dashboard/global/log"
	"regexp"

	"database/sql"
	"time"

	_ "github.com/go-sql-driver/mysql"
	_ "github.com/lib/pq"
)

type PagingType struct {
	Page     int
	Pagesize int
}

type OrderingType struct {
	Order string
}

type LimitType struct {
	Limit int
}

type OptionType struct {
	Page     int
	Pagesize int
	Order    string
	Limit    int
}

type Where struct {
	Column  string
	Value   interface{}
	Compare string
}

type Custom struct {
	Query string
	// Params 는 Query 안의 ? 자리표시자에 순서대로 바인딩되는 값이다.
	// 원시 SQL 에 사용자 입력을 직접 결합하지 말고 이 필드로 파라미터 바인딩할 것.
	// 주의: ? 자리표시자는 MySQL/SQLite 전용이다. Where 와 달리 case Custom 은
	// config.Database.Type 별($N) 치환을 하지 않으므로 Postgres 에서는 지원되지 않는다.
	Params []interface{}
}

type Base struct {
	Query string
}

type Groupby struct {
	Value int `json:"value"`
	Count int `json:"count"`
}

func Paging(page int, pagesize int) PagingType {
	return PagingType{Page: page, Pagesize: pagesize}
}

// orderingPattern 은 ORDER BY 절로 허용하는 형식이다: 콤마로 구분된
// "컬럼명 [asc|desc]" 목록 또는 rand(). 클라이언트의 orderby 쿼리 파라미터가
// 그대로 SQL 에 이어붙여지므로, 이 형식이 아니면 버려서 SQL 인젝션을 막는다.
var orderingPattern = regexp.MustCompile(`(?i)^\s*(rand\(\)|[a-z_][a-z0-9_.]*(\s+(asc|desc))?)(\s*,\s*[a-z_][a-z0-9_.]*(\s+(asc|desc))?)*\s*$`)

func Ordering(order string) OrderingType {
	if order != "" && !orderingPattern.MatchString(order) {
		log.Warn().Str("orderby", order).Msg("rejected invalid orderby")
		return OrderingType{Order: ""}
	}
	return OrderingType{Order: order}
}

func Limit(limit int) LimitType {
	return LimitType{Limit: limit}
}

type Connection struct {
	Conn *sql.DB
    Tx    *sql.Tx    
	Transaction bool
	Isolation bool
}

func (c *Connection) Close() {
	c.Conn.Close()
}

func (c *Connection) IsConnect() bool {
	return c.Conn != nil
}

func (c *Connection) Exec(query string, params ...interface{}) (sql.Result, error) {
	if c.Transaction {
       return c.Tx.Exec(query, params...)    
	} else {
       return c.Conn.Exec(query, params...)
    }
}

func (c *Connection) Query(query string, params ...interface{}) (*sql.Rows, error) {
	if c.Transaction {
       return c.Tx.Query(query, params...)    
	} else {
       return c.Conn.Query(query, params...)
    }
}

func (c *Connection) Begin() {
	if c.Transaction {
		return
	}

	c.Tx, _ = c.Conn.Begin()
	c.Transaction = true
	c.Isolation = true
}

func (c *Connection) Commit() error {
	c.Transaction = false
	return c.Tx.Commit()
}

func (c *Connection) Rollback() {
	if !c.Transaction {
		return
	}

	err := c.Tx.Rollback()
	if err != nil {
		log.Error().Msg(err.Error())
	}
	c.Transaction = false
}

func GetConnection() *Connection {
	conn, err := sql.Open(config.Database.TypeString, config.Database.ConnectionString)
	if err != nil {
		log.Error().Msg(err.Error())
		return nil
	}

	conn.SetMaxOpenConns(100)
	conn.SetMaxIdleConns(10)
	conn.SetConnMaxLifetime(5 * time.Minute)

	return &Connection{
		Conn: conn,
		Tx: nil,
		Transaction: false,
	}
}

func NewConnection() *Connection {
	db := GetConnection()

	if db != nil {
		return db
	}

	time.Sleep(100 * time.Millisecond)

	db = GetConnection()

	if db != nil {
		return db
	}

	time.Sleep(500 * time.Millisecond)

	db = GetConnection()

	if db != nil {
		return db
	}

	time.Sleep(1 * time.Second)

	db = GetConnection()

	if db != nil {
		return db
	}

	time.Sleep(2 * time.Second)

	db = GetConnection()

	return db
}

func QueryArray(db *Connection, query string, items []interface{}) (*sql.Rows, error) {
	var rows *sql.Rows
	var err error

	rows, err = db.Conn.Query(query, items...)
	return rows, err
}

func ExecArray(db *Connection, query string, items []interface{}) error {
	var err error

	_, err = db.Conn.Exec(query, items...)
	return err
}

func InitDate() string {
	return "1000-01-01 00:00:00"
}

type Double float64

func (c Double) MarshalJSON() ([]byte, error) {
	if float64(c) == float64(int(c)) {
		return []byte(fmt.Sprintf("%v.0", int64(c))), nil
	}

	return []byte(fmt.Sprintf("%v", float64(c))), nil
}

// whereColumnPattern 은 Where.Column(과 GroupBy 컬럼명)으로 허용하는 형식이다:
// SQL 식별자 하나 또는 "별칭.식별자". 컬럼명과 연산자는 자리표시자로 바인딩할 수 없어
// 쿼리에 그대로 이어붙으므로, 이 형식이 아니면 SQL 에 넣지 않는다.
var whereColumnPattern = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*(\.[A-Za-z_][A-Za-z0-9_]*)?$`)

// whereCompares 는 Where.Compare 로 허용하는 연산자 목록이다(소문자 기준).
var whereCompares = map[string]bool{
	"=": true, "!=": true, "<>": true, "<": true, "<=": true, ">": true, ">=": true,
	"like": true, "not like": true, "in": true, "not in": true, "between": true,
}

// ValidColumn 은 name 을 SQL 식별자로 그대로 이어붙여도 안전한지 확인한다.
func ValidColumn(name string) bool {
	return whereColumnPattern.MatchString(name)
}

// wherePlaceholder 는 DB 종류에 맞는 자리표시자를 돌려준다(Postgres 는 $N, 그 외 ?).
// pos 가 nil 이면 항상 ? 를 쓴다(번호를 추적하지 않는 MySQL 전용 쿼리용).
func wherePlaceholder(pos *int) string {
	if pos != nil && config.Database.Type == config.Postgresql {
		s := fmt.Sprintf("$%v", *pos)
		*pos++
		return s
	}
	return "?"
}

// whereValues 는 in / not in / between 값을 바인딩 값 목록으로 펼친다.
// 슬라이스·배열은 원소별로, 문자열은 콤마로 나눠서(예전 "1,2,3" 형태 호환), 그 외는 값 하나로 본다.
func whereValues(value interface{}) []interface{} {
	if value == nil {
		return nil
	}
	if s, ok := value.(string); ok {
		var ret []interface{}
		for _, part := range strings.Split(s, ",") {
			if part = strings.TrimSpace(part); part != "" {
				ret = append(ret, part)
			}
		}
		return ret
	}
	if _, ok := value.([]byte); ok {
		return []interface{}{value}
	}
	rv := reflect.ValueOf(value)
	if rv.Kind() != reflect.Slice && rv.Kind() != reflect.Array {
		return []interface{}{value}
	}
	ret := make([]interface{}, 0, rv.Len())
	for i := 0; i < rv.Len(); i++ {
		ret = append(ret, rv.Index(i).Interface())
	}
	return ret
}

// WhereClause 는 Where 하나를 " and <컬럼> <연산자> <자리표시자>" 로 만들고 바인딩 값을 돌려준다.
// 생성된 모델의 Find/Count/MakeQuery/GroupBy 가 모두 이 함수를 거친다.
//
//   - prefix 는 "w_" 같은 테이블 컬럼 접두어로, Column 에 "_" 가 없을 때만 붙인다(기존 규칙).
//   - Column 은 식별자 형식, Compare 는 whereCompares 에 있는 연산자만 허용한다.
//   - in / not in 값은 원소마다 자리표시자로 바인딩한다(값을 SQL 에 이어붙이지 않는다).
//
// 형식이 어긋나면 조건을 버리지 않고 " and 1=0" 으로 바꾼다(fail-closed). 조건을 그냥 빼면
// 소유자 필터 같은 조건이 사라져 오히려 더 많은 행이 조회·수정·삭제될 수 있기 때문이다.
func WhereClause(prefix string, item Where, pos *int) (string, []interface{}) {
	compare := strings.ToLower(strings.TrimSpace(item.Compare))
	if !ValidColumn(item.Column) || !whereCompares[compare] {
		log.Warn().Str("column", item.Column).Str("compare", item.Compare).Msg("rejected invalid where")
		return " and 1=0", nil
	}

	column := item.Column
	if !strings.Contains(column, "_") {
		column = prefix + column
	}

	switch compare {
	case "in", "not in":
		values := whereValues(item.Value)
		if len(values) == 0 {
			// 예전 코드는 빈 목록에서 "in ()" 문법 오류로 아무 행도 돌려주지/바꾸지 않았다. 같은 결과를 유지한다.
			return " and 1=0", nil
		}
		marks := make([]string, len(values))
		for i := range values {
			marks[i] = wherePlaceholder(pos)
		}
		return " and " + column + " " + compare + " (" + strings.Join(marks, ", ") + ")", values
	case "between":
		values := whereValues(item.Value)
		if _, isString := item.Value.(string); isString || len(values) != 2 {
			log.Warn().Str("column", item.Column).Msg("rejected invalid between value")
			return " and 1=0", nil
		}
		from := wherePlaceholder(pos)
		to := wherePlaceholder(pos)
		return " and " + column + " between " + from + " and " + to, values
	case "like":
		// 예전 규칙과 같이 like 만 양쪽에 % 를 붙인다(not like 는 값 그대로).
		value := item.Value
		if s, ok := value.(string); ok {
			value = "%" + s + "%"
		}
		return " and " + column + " " + compare + " " + wherePlaceholder(pos), []interface{}{value}
	default:
		return " and " + column + " " + compare + " " + wherePlaceholder(pos), []interface{}{item.Value}
	}
}
