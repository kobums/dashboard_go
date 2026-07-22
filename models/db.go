package models

import (
	"fmt"
	"dashboard/global/config"
	"dashboard/global/log"

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

func Ordering(order string) OrderingType {
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
