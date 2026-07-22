package models

import (
    "dashboard/global/config"
    "dashboard/models/healthmetric"
    "database/sql"
    "errors"
    "fmt"
    "strings"
    "time"

    log "dashboard/global/log"
    _ "github.com/go-sql-driver/mysql"
    _ "github.com/lib/pq"

)

type Healthmetric struct {
            
    Id                int64 `json:"id"`         
    Metricdate                string `json:"metricdate"`         
    Name                string `json:"name"`         
    Qty                Double `json:"qty"`         
    Unit                string `json:"unit"`         
    Createddate                string `json:"createddate"` 
    
    Extra                    map[string]interface{} `json:"extra"`
}

type HealthmetricManager struct {
    Conn    *Connection
    Result  *sql.Result
    Index   string
    Isolation   bool
    SelectQuery  string
    JoinQuery string
    CountQuery   string
    GroupQuery string
    SelectLog bool
    Log bool
}

func (c *Healthmetric) AddExtra(key string, value interface{}) {    
	c.Extra[key] = value     
}

func NewHealthmetricManager(conn *Connection) *HealthmetricManager {
    var item HealthmetricManager


    if conn == nil {
        item.Conn = NewConnection()
        item.Isolation = false
    } else {
        item.Conn = conn 
        item.Isolation = conn.Isolation
    }

    item.Index = ""
    item.SelectLog = config.Log.Database
    item.Log = config.Log.Database

    return &item
}

func (p *HealthmetricManager) Close() {
    if p.Conn != nil {
        p.Conn.Close()
    }
}

func (p *HealthmetricManager) SetIndex(index string) {
    p.Index = index
}

func (p *HealthmetricManager) SetCountQuery(query string) {
    p.CountQuery = query
}

func (p *HealthmetricManager) SetSelectQuery(query string) {
    p.SelectQuery = query
}

func (p *HealthmetricManager) Exec(query string, params ...interface{}) (sql.Result, error) {
    if p.Log {
       if len(params) > 0 {
	       log.Debug().Str("query", query).Any("param", params).Msg("SQL")
       } else {
	       log.Debug().Str("query", query).Msg("SQL")
       }
    }

    return p.Conn.Exec(query, params...)
}

func (p *HealthmetricManager) Query(query string, params ...interface{}) (*sql.Rows, error) {
    if p.Isolation {
        query += " for update"
    }

    if p.SelectLog {
       if len(params) > 0 {
	       log.Debug().Str("query", query).Any("param", params).Msg("SQL")
       } else {
	       log.Debug().Str("query", query).Msg("SQL")
       }
    }

    return p.Conn.Query(query, params...)
}

func (p *HealthmetricManager) GetQuery() string {
    if p.SelectQuery != "" {
        return p.SelectQuery    
    }

    var ret strings.Builder

    ret.WriteString("select hm_id, hm_metricdate, hm_name, hm_qty, hm_unit, hm_createddate from healthmetric_tb")

    if p.Index != "" {
        ret.WriteString(" use index(")
        ret.WriteString(p.Index)
        ret.WriteString(")")
    }

    if p.JoinQuery != "" {
        ret.WriteString(", ")
        ret.WriteString(p.JoinQuery)
    }

    ret.WriteString(" where 1=1 ")
    

    return ret.String()
}

func (p *HealthmetricManager) GetQuerySelect() string {
    if p.CountQuery != "" {
        return p.CountQuery    
    }

    var ret strings.Builder
    
    ret.WriteString("select count(*) from healthmetric_tb")

    if p.Index != "" {
        ret.WriteString(" use index(")
        ret.WriteString(p.Index)
        ret.WriteString(")")
    }

    if p.JoinQuery != "" {
        ret.WriteString(", ")
        ret.WriteString(p.JoinQuery)
    }

    ret.WriteString(" where 1=1 ")
    

    return ret.String()
}

func (p *HealthmetricManager) GetQueryGroup(name string) string {
    if p.SelectQuery != "" {
        return p.SelectQuery    
    }

    var ret strings.Builder
    ret.WriteString("select hm_")
    ret.WriteString(name)
    ret.WriteString(", count(*) from healthmetric_tb ")

    if p.Index != "" {
        ret.WriteString(" use index(")
        ret.WriteString(p.Index)
        ret.WriteString(")")
    }

    ret.WriteString(" where 1=1 ")
    

    return ret.String()
}

func (p *HealthmetricManager) Truncate() error {
    if !p.Conn.IsConnect() {
        return errors.New("Connection Error")
    }
    
    query := "truncate healthmetric_tb "
    _, err := p.Exec(query)

    if err != nil {
       if p.Log {
          log.Error().Str("error", err.Error()).Msg("SQL")
       }
    }

    return nil
}

func (p *HealthmetricManager) Insert(item *Healthmetric) error {
    if !p.Conn.IsConnect() {
        return errors.New("Connection Error")
    }

    if item.Createddate == "" {
        t := time.Now().UTC().Add(time.Hour * 9)
        //t := time.Now()
        item.Createddate = fmt.Sprintf("%04d-%02d-%02d %02d:%02d:%02d", t.Year(), t.Month(), t.Day(), t.Hour(), t.Minute(), t.Second())
    }

    
    if item.Metricdate == "" {
       item.Metricdate = "1000-01-01 00:00:00"
    }
	
    if item.Createddate == "" {
       item.Createddate = "1000-01-01 00:00:00"
    }
	

    query := ""
    var res sql.Result
    var err error
    if item.Id > 0 {
        query = "insert into healthmetric_tb (hm_id, hm_metricdate, hm_name, hm_qty, hm_unit, hm_createddate) values (?, ?, ?, ?, ?, ?)"
        res, err = p.Exec(query, item.Id, item.Metricdate, item.Name, item.Qty, item.Unit, item.Createddate)
    } else {
        query = "insert into healthmetric_tb (hm_metricdate, hm_name, hm_qty, hm_unit, hm_createddate) values (?, ?, ?, ?, ?)"
        res, err = p.Exec(query, item.Metricdate, item.Name, item.Qty, item.Unit, item.Createddate)
    }
    
    if err == nil {
        p.Result = &res
        
    } else {
        if p.Log {
          log.Error().Str("error", err.Error()).Msg("SQL")
        }
        p.Result = nil
    }

    return err
}

func (p *HealthmetricManager) Delete(id int64) error {
    if !p.Conn.IsConnect() {
        return errors.New("Connection Error")
    }

    query := "delete from healthmetric_tb where hm_id = ?"
    _, err := p.Exec(query, id)

    if err != nil {
       if p.Log {
          log.Error().Str("error", err.Error()).Msg("SQL")
       }
    }

    
    return err
}

func (p *HealthmetricManager) DeleteAll() error {
    if !p.Conn.IsConnect() {
        return errors.New("Connection Error")
    }

    query := "delete from healthmetric_tb"
    _, err := p.Exec(query)

    if err != nil {
       if p.Log {
          log.Error().Str("error", err.Error()).Msg("SQL")
       }
    }

    return err
}

func (p *HealthmetricManager) MakeQuery(initQuery string , postQuery string, initParams []interface{}, args []interface{}) (string, []interface{}) {
    var params []interface{}
    if initParams != nil {
        params = append(params, initParams...)
    }

    pos := 1

    var query strings.Builder
	query.WriteString(initQuery)

    for _, arg := range args {
        switch v := arg.(type) {        
        case Where:
            item := v

            if strings.Contains(item.Column, "_") {
                query.WriteString(" and ")
            } else {
                query.WriteString(" and hm_")
            }
            query.WriteString(item.Column)

            if item.Compare == "in" {
                query.WriteString(" in (")
                query.WriteString(strings.Trim(strings.Replace(fmt.Sprint(item.Value), " ", ", ", -1), "[]"))
                query.WriteString(")")
            } else if item.Compare == "not in" {
                query.WriteString(" not in (")
                query.WriteString(strings.Trim(strings.Replace(fmt.Sprint(item.Value), " ", ", ", -1), "[]"))
                query.WriteString(")")
            } else if item.Compare == "between" {
                if config.Database.Type == config.Postgresql {
                    query.WriteString(fmt.Sprintf(" between $%v and $%v", pos, pos + 1))
                    pos += 2
                } else {
                    query.WriteString(" between ? and ?")
                }

                s := item.Value.([2]string)
                params = append(params, s[0])
                params = append(params, s[1])
            } else {
                if config.Database.Type == config.Postgresql {
                    query.WriteString(" ")
                    query.WriteString(item.Compare)
                    query.WriteString(fmt.Sprintf(" $%v", pos))
                    pos++
                } else {
                    query.WriteString(" ")
                    query.WriteString(item.Compare)
                    query.WriteString(" ?")
                }
                if item.Compare == "like" {
                    params = append(params, "%" + item.Value.(string) + "%")
                } else {
                    params = append(params, item.Value)                
                }
            }
        case Custom:
             item := v

            query.WriteString(" and ")
            query.WriteString(item.Query)
        }        
    }

	query.WriteString(postQuery)

    return query.String(), params
}

func (p *HealthmetricManager) DeleteWhere(args []interface{}) error {
    if !p.Conn.IsConnect() {
        return errors.New("Connection Error")
    }

    query, params := p.MakeQuery("delete from healthmetric_tb where 1=1", "", nil, args)
    _, err := p.Exec(query, params...)

    if err != nil {
       if p.Log {
          log.Error().Str("error", err.Error()).Msg("SQL")
       }
    }

    return err
}

func (p *HealthmetricManager) Update(item *Healthmetric) error {
    if !p.Conn.IsConnect() {
        return errors.New("Connection Error")
    }
    
    
    if item.Metricdate == "" {
       item.Metricdate = "1000-01-01 00:00:00"
    }
	
    if item.Createddate == "" {
       item.Createddate = "1000-01-01 00:00:00"
    }
	

	query := "update healthmetric_tb set hm_metricdate = ?, hm_name = ?, hm_qty = ?, hm_unit = ?, hm_createddate = ? where hm_id = ?"
	_, err := p.Exec(query, item.Metricdate, item.Name, item.Qty, item.Unit, item.Createddate, item.Id)

    if err != nil {
        if p.Log {
          log.Error().Str("error", err.Error()).Msg("SQL")
        }
    }
    
        
    return err
}

func (p *HealthmetricManager) UpdateWhere(columns []healthmetric.Params, args []interface{}) error {
    if !p.Conn.IsConnect() {
        return errors.New("Connection Error")
    }

    var initQuery strings.Builder
    var initParams []interface{}

    initQuery.WriteString("update healthmetric_tb set ")
    for i, v := range columns {
        if i > 0 {
            initQuery.WriteString(", ")
        }

        if v.Column == healthmetric.ColumnId {
        initQuery.WriteString("hm_id = ?")
        initParams = append(initParams, v.Value)
        } else if v.Column == healthmetric.ColumnMetricdate {
        initQuery.WriteString("hm_metricdate = ?")
        initParams = append(initParams, v.Value)
        } else if v.Column == healthmetric.ColumnName {
        initQuery.WriteString("hm_name = ?")
        initParams = append(initParams, v.Value)
        } else if v.Column == healthmetric.ColumnQty {
        initQuery.WriteString("hm_qty = ?")
        initParams = append(initParams, v.Value)
        } else if v.Column == healthmetric.ColumnUnit {
        initQuery.WriteString("hm_unit = ?")
        initParams = append(initParams, v.Value)
        } else if v.Column == healthmetric.ColumnCreateddate {
        initQuery.WriteString("hm_createddate = ?")
        initParams = append(initParams, v.Value)
        } else {
        
        }
    }

    initQuery.WriteString(" where 1=1 ")

    query, params := p.MakeQuery(initQuery.String(), "", initParams, args)
    _, err := p.Exec(query, params...)

    if err != nil {
       if p.Log {
          log.Error().Str("error", err.Error()).Msg("SQL")
       }
    }

    
    return err
}

/*


func (p *HealthmetricManager) UpdateMetricdate(value string, id int64) error {
    if !p.Conn.IsConnect() {
        return errors.New("Connection Error")
    }

	query := "update healthmetric_tb set hm_metricdate = ? where hm_id = ?"
	_, err := p.Exec(query, value, id)

    if err != nil {
        if p.Log {
          log.Error().Str("error", err.Error()).Msg("SQL")
        }
    }

    return err
}

func (p *HealthmetricManager) UpdateName(value string, id int64) error {
    if !p.Conn.IsConnect() {
        return errors.New("Connection Error")
    }

	query := "update healthmetric_tb set hm_name = ? where hm_id = ?"
	_, err := p.Exec(query, value, id)

    if err != nil {
        if p.Log {
          log.Error().Str("error", err.Error()).Msg("SQL")
        }
    }

    return err
}

func (p *HealthmetricManager) UpdateQty(value Double, id int64) error {
    if !p.Conn.IsConnect() {
        return errors.New("Connection Error")
    }

	query := "update healthmetric_tb set hm_qty = ? where hm_id = ?"
	_, err := p.Exec(query, value, id)

    if err != nil {
        if p.Log {
          log.Error().Str("error", err.Error()).Msg("SQL")
        }
    }

    return err
}

func (p *HealthmetricManager) UpdateUnit(value string, id int64) error {
    if !p.Conn.IsConnect() {
        return errors.New("Connection Error")
    }

	query := "update healthmetric_tb set hm_unit = ? where hm_id = ?"
	_, err := p.Exec(query, value, id)

    if err != nil {
        if p.Log {
          log.Error().Str("error", err.Error()).Msg("SQL")
        }
    }

    return err
}

func (p *HealthmetricManager) UpdateCreateddate(value string, id int64) error {
    if !p.Conn.IsConnect() {
        return errors.New("Connection Error")
    }

	query := "update healthmetric_tb set hm_createddate = ? where hm_id = ?"
	_, err := p.Exec(query, value, id)

    if err != nil {
        if p.Log {
          log.Error().Str("error", err.Error()).Msg("SQL")
        }
    }

    return err
}


*/

func (p *HealthmetricManager) GetIdentity() int64 {
    if !p.Conn.IsConnect() {
        return 0
    }

    id, err := (*p.Result).LastInsertId()

    if err != nil {
        if p.Log {
          log.Error().Str("error", err.Error()).Msg("SQL")
        }
        return 0
    } else {
        return id
    }
}

func (p *Healthmetric) InitExtra() {
    p.Extra = map[string]interface{}{

    }
}

func (p *HealthmetricManager) ReadRow(rows *sql.Rows) *Healthmetric {
    var item Healthmetric
    var err error

    

    if rows.Next() {
        err = rows.Scan(&item.Id, &item.Metricdate, &item.Name, &item.Qty, &item.Unit, &item.Createddate)
        
        if item.Metricdate == "0000-00-00 00:00:00" || item.Metricdate == "1000-01-01 00:00:00" || item.Metricdate == "9999-01-01 00:00:00" {
            item.Metricdate = ""
        }

        if config.Database.Type == config.Postgresql {
            item.Metricdate = strings.ReplaceAll(strings.ReplaceAll(item.Metricdate, "T", " "), "Z", "")
        }
		
        if item.Createddate == "0000-00-00 00:00:00" || item.Createddate == "1000-01-01 00:00:00" || item.Createddate == "9999-01-01 00:00:00" {
            item.Createddate = ""
        }

        if config.Database.Type == config.Postgresql {
            item.Createddate = strings.ReplaceAll(strings.ReplaceAll(item.Createddate, "T", " "), "Z", "")
        }
		

    } else {
        return nil
    }

    if err != nil {
        if p.Log {
          log.Error().Str("error", err.Error()).Msg("SQL")
        }
        return nil
    } else {

        item.InitExtra()
        
        return &item
    }
}

func (p *HealthmetricManager) ReadRows(rows *sql.Rows) []Healthmetric {
    var items []Healthmetric

    for rows.Next() {
        var item Healthmetric
        

        err := rows.Scan(&item.Id, &item.Metricdate, &item.Name, &item.Qty, &item.Unit, &item.Createddate)
        if err != nil {
           if p.Log {
             log.Error().Str("error", err.Error()).Msg("SQL")
           }
           break
        }

        
        if item.Metricdate == "0000-00-00 00:00:00" || item.Metricdate == "1000-01-01 00:00:00" || item.Metricdate == "9999-01-01 00:00:00" {
            item.Metricdate = ""
        }

        if config.Database.Type == config.Postgresql {
            item.Metricdate = strings.ReplaceAll(strings.ReplaceAll(item.Metricdate, "T", " "), "Z", "")
        }
		
        if item.Createddate == "0000-00-00 00:00:00" || item.Createddate == "1000-01-01 00:00:00" || item.Createddate == "9999-01-01 00:00:00" {
            item.Createddate = ""
        }

        if config.Database.Type == config.Postgresql {
            item.Createddate = strings.ReplaceAll(strings.ReplaceAll(item.Createddate, "T", " "), "Z", "")
        }
		

        item.InitExtra()
        
        items = append(items, item)
    }


     return items
}

func (p *HealthmetricManager) Get(id int64) *Healthmetric {
    if !p.Conn.IsConnect() {
        return nil
    }

    var query strings.Builder
    query.WriteString(p.GetQuery())
    query.WriteString(" and hm_id = ?")

    
    
    rows, err := p.Query(query.String(), id)

    if err != nil {
       if p.Log {
          log.Error().Str("error", err.Error()).Msg("SQL")
       }
       return nil
    }

    defer rows.Close()

    return p.ReadRow(rows)
}

func (p *HealthmetricManager) GetWhere(args []interface{}) *Healthmetric {
    items := p.Find(args)
    if len(items) == 0 {
        return nil
    }

    return &items[0]
}

func (p *HealthmetricManager) Count(args []interface{}) int {
    if !p.Conn.IsConnect() {
        return 0
    }

    query, params := p.MakeQuery(p.GetQuerySelect(), p.GroupQuery, nil, args)
    rows, err := p.Query(query, params...)

    if err != nil {
       if p.Log {
          log.Error().Str("error", err.Error()).Msg("SQL")
       }
       return 0
    }

    defer rows.Close()

    if !rows.Next() {
        return 0
    }

    cnt := 0
    err = rows.Scan(&cnt)

    if err != nil {
        return 0
    } else {
        return cnt
    }
}

func (p *HealthmetricManager) FindAll() []Healthmetric {
    return p.Find(nil)
}

func (p *HealthmetricManager) Find(args []interface{}) []Healthmetric {
    if !p.Conn.IsConnect() {
        var items []Healthmetric
        return items
    }

    var params []interface{}
    baseQuery := p.GetQuery()

    var query strings.Builder

    page := 0
    pagesize := 0
    orderby := ""

    pos := 1
    
    for _, arg := range args {
        switch v := arg.(type) {
        case PagingType:
            item := v
            page = item.Page
            pagesize = item.Pagesize            
        case OrderingType:
            item := v
            orderby = item.Order
        case LimitType:
            item := v
            page = 1
            pagesize = item.Limit
        case OptionType:
            item := v
            if item.Limit > 0 {
                page = 1
                pagesize = item.Limit
            } else {
                page = item.Page
                pagesize = item.Pagesize                
            }
            orderby = item.Order
        case Where:
            item := v

            if strings.Contains(item.Column, "_") {
                query.WriteString(" and ")
            } else {
                query.WriteString(" and hm_")
            }
            query.WriteString(item.Column)
            
            if item.Compare == "in" {
                query.WriteString(" in (")
                query.WriteString(strings.Trim(strings.Replace(fmt.Sprint(item.Value), " ", ", ", -1), "[]"))
                query.WriteString(")")
            } else if item.Compare == "not in" {
                query.WriteString(" not in (")
                query.WriteString(strings.Trim(strings.Replace(fmt.Sprint(item.Value), " ", ", ", -1), "[]"))
                query.WriteString(")")
            } else if item.Compare == "between" {
                if config.Database.Type == config.Postgresql {
                    query.WriteString(fmt.Sprintf(" between $%v and $%v", pos, pos + 1))
                    pos += 2
                } else {
                    query.WriteString(" between ? and ?")
                }

                s := item.Value.([2]string)
                params = append(params, s[0])
                params = append(params, s[1])
            } else {
                if config.Database.Type == config.Postgresql {
                    query.WriteString(" ")
                    query.WriteString(item.Compare)
                    query.WriteString(fmt.Sprintf(" $%v", pos))
                    pos++
                } else {
                    query.WriteString(" ")
                    query.WriteString(item.Compare)
                    query.WriteString(" ?")
                }
                if item.Compare == "like" {
                    params = append(params, "%" + item.Value.(string) + "%")
                } else {
                    params = append(params, item.Value)                
                }
            }
        case Custom:
             item := v

            query.WriteString(" and ")
            query.WriteString(item.Query)
        case Base:
             item := v

             baseQuery = item.Query
        }
    }

    query.WriteString(p.GroupQuery)
    
    startpage := (page - 1) * pagesize
    
    if page > 0 && pagesize > 0 {
        if orderby == "" {
            orderby = "hm_id desc"
        } else {
            if !strings.Contains(orderby, "_") {                   
                if strings.ToUpper(orderby) != "RAND()" {
                    orderby = "hm_" + orderby
                }
            }
            
        }
        query.WriteString(" order by ")
        query.WriteString(orderby)
        if config.Database.Type == config.Postgresql {
            query.WriteString(fmt.Sprintf(" limit $%v offset $%v", pos, pos + 1))
            params = append(params, pagesize)
            params = append(params, startpage)
        } else if config.Database.Type == config.Mysql {
            query.WriteString(" limit ? offset ?")
            params = append(params, pagesize)
            params = append(params, startpage)
        } else if config.Database.Type == config.Sqlserver {
            query.WriteString("OFFSET ? ROWS FETCH NEXT ? ROWS ONLY")
            params = append(params, startpage)
            params = append(params, pagesize)
        }
    } else {
        if orderby == "" {
            orderby = "hm_id"
        } else {
            if !strings.Contains(orderby, "_") {
                if strings.ToUpper(orderby) != "RAND()" {
                    orderby = "hm_" + orderby
                }
            }
        }
        query.WriteString(" order by ")
        query.WriteString(orderby)
    }

    rows, err := p.Query(baseQuery + query.String(), params...)

    if err != nil {
       if p.Log {
          log.Error().Str("error", err.Error()).Msg("SQL")
       }
        items := make([]Healthmetric, 0)
        return items
    }

    defer rows.Close()

    return p.ReadRows(rows)
}





func (p *HealthmetricManager) GroupBy(name string, args []interface{}) []Groupby {
    if !p.Conn.IsConnect() {
        var items []Groupby
        return items
    }

    var params []interface{}
    baseQuery := p.GetQueryGroup(name)
    var query strings.Builder
    pos := 1

    for _, arg := range args {
        switch v := arg.(type) {
        case Where:
            item := v

            if strings.Contains(item.Column, "_") {
                query.WriteString(" and ")
            } else {
                query.WriteString(" and hm_")
            }
            query.WriteString(item.Column)
            
            if item.Compare == "in" {
                query.WriteString(" in (")
                query.WriteString(strings.Trim(strings.Replace(fmt.Sprint(item.Value), " ", ", ", -1), "[]"))
                query.WriteString(")")
            } else if item.Compare == "not in" {
                query.WriteString(" not in (")
                query.WriteString(strings.Trim(strings.Replace(fmt.Sprint(item.Value), " ", ", ", -1), "[]"))
                query.WriteString(")")
            } else if item.Compare == "between" {
                if config.Database.Type == config.Postgresql {
                    query.WriteString(fmt.Sprintf(" between $%v and $%v", pos, pos + 1))
                    pos += 2
                } else {
                    query.WriteString(" between ? and ?")
                }

                s := item.Value.([2]string)
                params = append(params, s[0])
                params = append(params, s[1])
            } else {
                if config.Database.Type == config.Postgresql {
                    query.WriteString(" ")
                    query.WriteString(item.Compare)
                    query.WriteString(fmt.Sprintf(" $%v", pos))
                    pos++
                } else {
                    query.WriteString(" ")
                    query.WriteString(item.Compare)
                    query.WriteString(" ?")
                }
                if item.Compare == "like" {
                    params = append(params, "%" + item.Value.(string) + "%")
                } else {
                    params = append(params, item.Value)                
                }
            }
        case Custom:
             item := v

            query.WriteString(" and ")
            query.WriteString(item.Query)
        case Base:
             item := v

             baseQuery = item.Query
        }
    }
    
    query.WriteString(" group by hm_")
    query.WriteString(name)

    rows, err := p.Query(baseQuery + query.String(), params...)

    if err != nil {
       if p.Log {
          log.Error().Str("error", err.Error()).Msg("SQL")
       }
        var items []Groupby
        return items
    }

    defer rows.Close()

    var items []Groupby

    for rows.Next() {
        var item Groupby
        err := rows.Scan(&item.Value, &item.Count)
        if err != nil {
           if p.Log {
                log.Error().Str("error", err.Error()).Msg("SQL")
           }
           break
        }

        items = append(items, item)
    }

    return items
}
