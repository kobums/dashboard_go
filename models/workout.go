package models

import (
    "dashboard/global/config"
    "dashboard/models/workout"
    "database/sql"
    "errors"
    "fmt"
    "strings"
    "time"

    log "dashboard/global/log"
    _ "github.com/go-sql-driver/mysql"
    _ "github.com/lib/pq"

)

type Workout struct {
            
    Id                int64 `json:"id"`         
    Type                string `json:"type"`         
    Title                string `json:"title"`         
    Workoutdate                string `json:"workoutdate"`         
    Starttime                string `json:"starttime"`         
    Duration                int `json:"duration"`         
    Calories                int `json:"calories"`         
    Distance                Double `json:"distance"`         
    Memo                string `json:"memo"`         
    Source                string `json:"source"`         
    Externalid                string `json:"externalid"`         
    Createddate                string `json:"createddate"` 
    
    Extra                    map[string]interface{} `json:"extra"`
}

type WorkoutManager struct {
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

func (c *Workout) AddExtra(key string, value interface{}) {    
	c.Extra[key] = value     
}

func NewWorkoutManager(conn *Connection) *WorkoutManager {
    var item WorkoutManager


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

func (p *WorkoutManager) Close() {
    if p.Conn != nil {
        p.Conn.Close()
    }
}

func (p *WorkoutManager) SetIndex(index string) {
    p.Index = index
}

func (p *WorkoutManager) SetCountQuery(query string) {
    p.CountQuery = query
}

func (p *WorkoutManager) SetSelectQuery(query string) {
    p.SelectQuery = query
}

func (p *WorkoutManager) Exec(query string, params ...interface{}) (sql.Result, error) {
    if p.Log {
       if len(params) > 0 {
	       log.Debug().Str("query", query).Any("param", params).Msg("SQL")
       } else {
	       log.Debug().Str("query", query).Msg("SQL")
       }
    }

    return p.Conn.Exec(query, params...)
}

func (p *WorkoutManager) Query(query string, params ...interface{}) (*sql.Rows, error) {
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

func (p *WorkoutManager) GetQuery() string {
    if p.SelectQuery != "" {
        return p.SelectQuery    
    }

    var ret strings.Builder

    ret.WriteString("select w_id, w_type, w_title, w_workoutdate, w_starttime, w_duration, w_calories, w_distance, w_memo, w_source, w_externalid, w_createddate from workout_tb")

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

func (p *WorkoutManager) GetQuerySelect() string {
    if p.CountQuery != "" {
        return p.CountQuery    
    }

    var ret strings.Builder
    
    ret.WriteString("select count(*) from workout_tb")

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

func (p *WorkoutManager) GetQueryGroup(name string) string {
    if p.SelectQuery != "" {
        return p.SelectQuery    
    }

    var ret strings.Builder
    ret.WriteString("select w_")
    ret.WriteString(name)
    ret.WriteString(", count(*) from workout_tb ")

    if p.Index != "" {
        ret.WriteString(" use index(")
        ret.WriteString(p.Index)
        ret.WriteString(")")
    }

    ret.WriteString(" where 1=1 ")
    

    return ret.String()
}

func (p *WorkoutManager) Truncate() error {
    if !p.Conn.IsConnect() {
        return errors.New("Connection Error")
    }
    
    query := "truncate workout_tb "
    _, err := p.Exec(query)

    if err != nil {
       if p.Log {
          log.Error().Str("error", err.Error()).Msg("SQL")
       }
    }

    return nil
}

func (p *WorkoutManager) Insert(item *Workout) error {
    if !p.Conn.IsConnect() {
        return errors.New("Connection Error")
    }

    if item.Createddate == "" {
        t := time.Now().UTC().Add(time.Hour * 9)
        //t := time.Now()
        item.Createddate = fmt.Sprintf("%04d-%02d-%02d %02d:%02d:%02d", t.Year(), t.Month(), t.Day(), t.Hour(), t.Minute(), t.Second())
    }

    
    if item.Workoutdate == "" {
       item.Workoutdate = "1000-01-01 00:00:00"
    }
	
    if item.Starttime == "" {
       item.Starttime = "1000-01-01 00:00:00"
    }
	
    if item.Createddate == "" {
       item.Createddate = "1000-01-01 00:00:00"
    }
	

    query := ""
    var res sql.Result
    var err error
    if item.Id > 0 {
        query = "insert into workout_tb (w_id, w_type, w_title, w_workoutdate, w_starttime, w_duration, w_calories, w_distance, w_memo, w_source, w_externalid, w_createddate) values (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)"
        res, err = p.Exec(query, item.Id, item.Type, item.Title, item.Workoutdate, item.Starttime, item.Duration, item.Calories, item.Distance, item.Memo, item.Source, item.Externalid, item.Createddate)
    } else {
        query = "insert into workout_tb (w_type, w_title, w_workoutdate, w_starttime, w_duration, w_calories, w_distance, w_memo, w_source, w_externalid, w_createddate) values (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)"
        res, err = p.Exec(query, item.Type, item.Title, item.Workoutdate, item.Starttime, item.Duration, item.Calories, item.Distance, item.Memo, item.Source, item.Externalid, item.Createddate)
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

func (p *WorkoutManager) Delete(id int64) error {
    if !p.Conn.IsConnect() {
        return errors.New("Connection Error")
    }

    query := "delete from workout_tb where w_id = ?"
    _, err := p.Exec(query, id)

    if err != nil {
       if p.Log {
          log.Error().Str("error", err.Error()).Msg("SQL")
       }
    }

    
    return err
}

func (p *WorkoutManager) DeleteAll() error {
    if !p.Conn.IsConnect() {
        return errors.New("Connection Error")
    }

    query := "delete from workout_tb"
    _, err := p.Exec(query)

    if err != nil {
       if p.Log {
          log.Error().Str("error", err.Error()).Msg("SQL")
       }
    }

    return err
}

func (p *WorkoutManager) MakeQuery(initQuery string , postQuery string, initParams []interface{}, args []interface{}) (string, []interface{}) {
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
                query.WriteString(" and w_")
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

func (p *WorkoutManager) DeleteWhere(args []interface{}) error {
    if !p.Conn.IsConnect() {
        return errors.New("Connection Error")
    }

    query, params := p.MakeQuery("delete from workout_tb where 1=1", "", nil, args)
    _, err := p.Exec(query, params...)

    if err != nil {
       if p.Log {
          log.Error().Str("error", err.Error()).Msg("SQL")
       }
    }

    return err
}

func (p *WorkoutManager) Update(item *Workout) error {
    if !p.Conn.IsConnect() {
        return errors.New("Connection Error")
    }
    
    
    if item.Workoutdate == "" {
       item.Workoutdate = "1000-01-01 00:00:00"
    }
	
    if item.Starttime == "" {
       item.Starttime = "1000-01-01 00:00:00"
    }
	
    if item.Createddate == "" {
       item.Createddate = "1000-01-01 00:00:00"
    }
	

	query := "update workout_tb set w_type = ?, w_title = ?, w_workoutdate = ?, w_starttime = ?, w_duration = ?, w_calories = ?, w_distance = ?, w_memo = ?, w_source = ?, w_externalid = ?, w_createddate = ? where w_id = ?"
	_, err := p.Exec(query, item.Type, item.Title, item.Workoutdate, item.Starttime, item.Duration, item.Calories, item.Distance, item.Memo, item.Source, item.Externalid, item.Createddate, item.Id)

    if err != nil {
        if p.Log {
          log.Error().Str("error", err.Error()).Msg("SQL")
        }
    }
    
        
    return err
}

func (p *WorkoutManager) UpdateWhere(columns []workout.Params, args []interface{}) error {
    if !p.Conn.IsConnect() {
        return errors.New("Connection Error")
    }

    var initQuery strings.Builder
    var initParams []interface{}

    initQuery.WriteString("update workout_tb set ")
    for i, v := range columns {
        if i > 0 {
            initQuery.WriteString(", ")
        }

        if v.Column == workout.ColumnId {
        initQuery.WriteString("w_id = ?")
        initParams = append(initParams, v.Value)
        } else if v.Column == workout.ColumnType {
        initQuery.WriteString("w_type = ?")
        initParams = append(initParams, v.Value)
        } else if v.Column == workout.ColumnTitle {
        initQuery.WriteString("w_title = ?")
        initParams = append(initParams, v.Value)
        } else if v.Column == workout.ColumnWorkoutdate {
        initQuery.WriteString("w_workoutdate = ?")
        initParams = append(initParams, v.Value)
        } else if v.Column == workout.ColumnStarttime {
        initQuery.WriteString("w_starttime = ?")
        initParams = append(initParams, v.Value)
        } else if v.Column == workout.ColumnDuration {
        initQuery.WriteString("w_duration = ?")
        initParams = append(initParams, v.Value)
        } else if v.Column == workout.ColumnCalories {
        initQuery.WriteString("w_calories = ?")
        initParams = append(initParams, v.Value)
        } else if v.Column == workout.ColumnDistance {
        initQuery.WriteString("w_distance = ?")
        initParams = append(initParams, v.Value)
        } else if v.Column == workout.ColumnMemo {
        initQuery.WriteString("w_memo = ?")
        initParams = append(initParams, v.Value)
        } else if v.Column == workout.ColumnSource {
        initQuery.WriteString("w_source = ?")
        initParams = append(initParams, v.Value)
        } else if v.Column == workout.ColumnExternalid {
        initQuery.WriteString("w_externalid = ?")
        initParams = append(initParams, v.Value)
        } else if v.Column == workout.ColumnCreateddate {
        initQuery.WriteString("w_createddate = ?")
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


func (p *WorkoutManager) UpdateType(value string, id int64) error {
    if !p.Conn.IsConnect() {
        return errors.New("Connection Error")
    }

	query := "update workout_tb set w_type = ? where w_id = ?"
	_, err := p.Exec(query, value, id)

    if err != nil {
        if p.Log {
          log.Error().Str("error", err.Error()).Msg("SQL")
        }
    }

    return err
}

func (p *WorkoutManager) UpdateTitle(value string, id int64) error {
    if !p.Conn.IsConnect() {
        return errors.New("Connection Error")
    }

	query := "update workout_tb set w_title = ? where w_id = ?"
	_, err := p.Exec(query, value, id)

    if err != nil {
        if p.Log {
          log.Error().Str("error", err.Error()).Msg("SQL")
        }
    }

    return err
}

func (p *WorkoutManager) UpdateWorkoutdate(value string, id int64) error {
    if !p.Conn.IsConnect() {
        return errors.New("Connection Error")
    }

	query := "update workout_tb set w_workoutdate = ? where w_id = ?"
	_, err := p.Exec(query, value, id)

    if err != nil {
        if p.Log {
          log.Error().Str("error", err.Error()).Msg("SQL")
        }
    }

    return err
}

func (p *WorkoutManager) UpdateStarttime(value string, id int64) error {
    if !p.Conn.IsConnect() {
        return errors.New("Connection Error")
    }

	query := "update workout_tb set w_starttime = ? where w_id = ?"
	_, err := p.Exec(query, value, id)

    if err != nil {
        if p.Log {
          log.Error().Str("error", err.Error()).Msg("SQL")
        }
    }

    return err
}

func (p *WorkoutManager) UpdateDuration(value int, id int64) error {
    if !p.Conn.IsConnect() {
        return errors.New("Connection Error")
    }

	query := "update workout_tb set w_duration = ? where w_id = ?"
	_, err := p.Exec(query, value, id)

    if err != nil {
        if p.Log {
          log.Error().Str("error", err.Error()).Msg("SQL")
        }
    }

    return err
}

func (p *WorkoutManager) UpdateCalories(value int, id int64) error {
    if !p.Conn.IsConnect() {
        return errors.New("Connection Error")
    }

	query := "update workout_tb set w_calories = ? where w_id = ?"
	_, err := p.Exec(query, value, id)

    if err != nil {
        if p.Log {
          log.Error().Str("error", err.Error()).Msg("SQL")
        }
    }

    return err
}

func (p *WorkoutManager) UpdateDistance(value Double, id int64) error {
    if !p.Conn.IsConnect() {
        return errors.New("Connection Error")
    }

	query := "update workout_tb set w_distance = ? where w_id = ?"
	_, err := p.Exec(query, value, id)

    if err != nil {
        if p.Log {
          log.Error().Str("error", err.Error()).Msg("SQL")
        }
    }

    return err
}

func (p *WorkoutManager) UpdateMemo(value string, id int64) error {
    if !p.Conn.IsConnect() {
        return errors.New("Connection Error")
    }

	query := "update workout_tb set w_memo = ? where w_id = ?"
	_, err := p.Exec(query, value, id)

    if err != nil {
        if p.Log {
          log.Error().Str("error", err.Error()).Msg("SQL")
        }
    }

    return err
}

func (p *WorkoutManager) UpdateSource(value string, id int64) error {
    if !p.Conn.IsConnect() {
        return errors.New("Connection Error")
    }

	query := "update workout_tb set w_source = ? where w_id = ?"
	_, err := p.Exec(query, value, id)

    if err != nil {
        if p.Log {
          log.Error().Str("error", err.Error()).Msg("SQL")
        }
    }

    return err
}

func (p *WorkoutManager) UpdateExternalid(value string, id int64) error {
    if !p.Conn.IsConnect() {
        return errors.New("Connection Error")
    }

	query := "update workout_tb set w_externalid = ? where w_id = ?"
	_, err := p.Exec(query, value, id)

    if err != nil {
        if p.Log {
          log.Error().Str("error", err.Error()).Msg("SQL")
        }
    }

    return err
}

func (p *WorkoutManager) UpdateCreateddate(value string, id int64) error {
    if !p.Conn.IsConnect() {
        return errors.New("Connection Error")
    }

	query := "update workout_tb set w_createddate = ? where w_id = ?"
	_, err := p.Exec(query, value, id)

    if err != nil {
        if p.Log {
          log.Error().Str("error", err.Error()).Msg("SQL")
        }
    }

    return err
}


*/

func (p *WorkoutManager) GetIdentity() int64 {
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

func (p *Workout) InitExtra() {
    p.Extra = map[string]interface{}{

    }
}

func (p *WorkoutManager) ReadRow(rows *sql.Rows) *Workout {
    var item Workout
    var err error

    

    if rows.Next() {
        err = rows.Scan(&item.Id, &item.Type, &item.Title, &item.Workoutdate, &item.Starttime, &item.Duration, &item.Calories, &item.Distance, &item.Memo, &item.Source, &item.Externalid, &item.Createddate)
        
        if item.Workoutdate == "0000-00-00 00:00:00" || item.Workoutdate == "1000-01-01 00:00:00" || item.Workoutdate == "9999-01-01 00:00:00" {
            item.Workoutdate = ""
        }

        if config.Database.Type == config.Postgresql {
            item.Workoutdate = strings.ReplaceAll(strings.ReplaceAll(item.Workoutdate, "T", " "), "Z", "")
        }
		
        if item.Starttime == "0000-00-00 00:00:00" || item.Starttime == "1000-01-01 00:00:00" || item.Starttime == "9999-01-01 00:00:00" {
            item.Starttime = ""
        }

        if config.Database.Type == config.Postgresql {
            item.Starttime = strings.ReplaceAll(strings.ReplaceAll(item.Starttime, "T", " "), "Z", "")
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

func (p *WorkoutManager) ReadRows(rows *sql.Rows) []Workout {
    var items []Workout

    for rows.Next() {
        var item Workout
        

        err := rows.Scan(&item.Id, &item.Type, &item.Title, &item.Workoutdate, &item.Starttime, &item.Duration, &item.Calories, &item.Distance, &item.Memo, &item.Source, &item.Externalid, &item.Createddate)
        if err != nil {
           if p.Log {
             log.Error().Str("error", err.Error()).Msg("SQL")
           }
           break
        }

        
        if item.Workoutdate == "0000-00-00 00:00:00" || item.Workoutdate == "1000-01-01 00:00:00" || item.Workoutdate == "9999-01-01 00:00:00" {
            item.Workoutdate = ""
        }

        if config.Database.Type == config.Postgresql {
            item.Workoutdate = strings.ReplaceAll(strings.ReplaceAll(item.Workoutdate, "T", " "), "Z", "")
        }
		
        if item.Starttime == "0000-00-00 00:00:00" || item.Starttime == "1000-01-01 00:00:00" || item.Starttime == "9999-01-01 00:00:00" {
            item.Starttime = ""
        }

        if config.Database.Type == config.Postgresql {
            item.Starttime = strings.ReplaceAll(strings.ReplaceAll(item.Starttime, "T", " "), "Z", "")
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

func (p *WorkoutManager) Get(id int64) *Workout {
    if !p.Conn.IsConnect() {
        return nil
    }

    var query strings.Builder
    query.WriteString(p.GetQuery())
    query.WriteString(" and w_id = ?")

    
    
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

func (p *WorkoutManager) GetWhere(args []interface{}) *Workout {
    items := p.Find(args)
    if len(items) == 0 {
        return nil
    }

    return &items[0]
}

func (p *WorkoutManager) Count(args []interface{}) int {
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

func (p *WorkoutManager) FindAll() []Workout {
    return p.Find(nil)
}

func (p *WorkoutManager) Find(args []interface{}) []Workout {
    if !p.Conn.IsConnect() {
        var items []Workout
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
                query.WriteString(" and w_")
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
            orderby = "w_id desc"
        } else {
            if !strings.Contains(orderby, "_") {                   
                if strings.ToUpper(orderby) != "RAND()" {
                    orderby = "w_" + orderby
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
            orderby = "w_id"
        } else {
            if !strings.Contains(orderby, "_") {
                if strings.ToUpper(orderby) != "RAND()" {
                    orderby = "w_" + orderby
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
        items := make([]Workout, 0)
        return items
    }

    defer rows.Close()

    return p.ReadRows(rows)
}





func (p *WorkoutManager) GroupBy(name string, args []interface{}) []Groupby {
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
                query.WriteString(" and w_")
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
    
    query.WriteString(" group by w_")
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
