package rest


import (
	"dashboard/controllers"
	
	"dashboard/models"

    "strings"
)

type WorkoutController struct {
	controllers.Controller
}

func (c *WorkoutController) Read(id int64) {
    
    
	conn := c.NewConnection()

	manager := models.NewWorkoutManager(conn)
	item := manager.Get(id)

    
    
    c.Set("item", item)
}

func (c *WorkoutController) Index(page int, pagesize int) {
    
    
	conn := c.NewConnection()

	manager := models.NewWorkoutManager(conn)

    var args []interface{}
    
    _type := c.Get("type")
    if _type != "" {
        args = append(args, models.Where{Column:"type", Value:_type, Compare:"like"})
    }
    _title := c.Get("title")
    if _title != "" {
        args = append(args, models.Where{Column:"title", Value:_title, Compare:"="})
        
    }
    _startworkoutdate := c.Get("startworkoutdate")
    _endworkoutdate := c.Get("endworkoutdate")
    if _startworkoutdate != "" && _endworkoutdate != "" {        
        var v [2]string
        v[0] = _startworkoutdate
        v[1] = _endworkoutdate  
        args = append(args, models.Where{Column:"workoutdate", Value:v, Compare:"between"})    
    } else if  _startworkoutdate != "" {          
        args = append(args, models.Where{Column:"workoutdate", Value:_startworkoutdate, Compare:">="})
    } else if  _endworkoutdate != "" {          
        args = append(args, models.Where{Column:"workoutdate", Value:_endworkoutdate, Compare:"<="})            
    }
    _startstarttime := c.Get("startstarttime")
    _endstarttime := c.Get("endstarttime")
    if _startstarttime != "" && _endstarttime != "" {        
        var v [2]string
        v[0] = _startstarttime
        v[1] = _endstarttime  
        args = append(args, models.Where{Column:"starttime", Value:v, Compare:"between"})    
    } else if  _startstarttime != "" {          
        args = append(args, models.Where{Column:"starttime", Value:_startstarttime, Compare:">="})
    } else if  _endstarttime != "" {          
        args = append(args, models.Where{Column:"starttime", Value:_endstarttime, Compare:"<="})            
    }
    _duration := c.Geti("duration")
    if _duration != 0 {
        args = append(args, models.Where{Column:"duration", Value:_duration, Compare:"="})    
    }
    _calories := c.Geti("calories")
    if _calories != 0 {
        args = append(args, models.Where{Column:"calories", Value:_calories, Compare:"="})    
    }
    _distance := c.Geti("distance")
    if _distance != 0 {
        args = append(args, models.Where{Column:"distance", Value:_distance, Compare:"="})    
    }
    _memo := c.Get("memo")
    if _memo != "" {
        args = append(args, models.Where{Column:"memo", Value:_memo, Compare:"like"})
    }
    _source := c.Get("source")
    if _source != "" {
        args = append(args, models.Where{Column:"source", Value:_source, Compare:"like"})
    }
    _externalid := c.Get("externalid")
    if _externalid != "" {
        args = append(args, models.Where{Column:"externalid", Value:_externalid, Compare:"like"})
    }
    _startcreateddate := c.Get("startcreateddate")
    _endcreateddate := c.Get("endcreateddate")
    if _startcreateddate != "" && _endcreateddate != "" {        
        var v [2]string
        v[0] = _startcreateddate
        v[1] = _endcreateddate  
        args = append(args, models.Where{Column:"createddate", Value:v, Compare:"between"})    
    } else if  _startcreateddate != "" {          
        args = append(args, models.Where{Column:"createddate", Value:_startcreateddate, Compare:">="})
    } else if  _endcreateddate != "" {          
        args = append(args, models.Where{Column:"createddate", Value:_endcreateddate, Compare:"<="})            
    }
    

    
    
    if page != 0 && pagesize != 0 {
        args = append(args, models.Paging(page, pagesize))
    }
    
    orderby := c.Get("orderby")
    if orderby == "" {
        if page != 0 && pagesize != 0 {
            orderby = "id desc"
            args = append(args, models.Ordering(orderby))
        }
    } else {
        orderbys := strings.Split(orderby, ",")

        str := ""
        for i, v := range orderbys {
            if i == 0 {
                str += v
            } else {
                if strings.Contains(v, "_") {                   
                    str += ", " + strings.Trim(v, " ")
                } else {
                    str += ", w_" + strings.Trim(v, " ")                
                }
            }
        }
        
        args = append(args, models.Ordering(str))
    }
    
	items := manager.Find(args)
	c.Set("items", items)

    if page == 1 {
       total := manager.Count(args)
	   c.Set("total", total)
    }
}

func (c *WorkoutController) Count() {
    
    
	conn := c.NewConnection()

	manager := models.NewWorkoutManager(conn)

    var args []interface{}
    
    _type := c.Get("type")
    if _type != "" {
        args = append(args, models.Where{Column:"type", Value:_type, Compare:"like"})
        
    }
    _title := c.Get("title")
    if _title != "" {
        args = append(args, models.Where{Column:"title", Value:_title, Compare:"="})
        
        
    }
    _startworkoutdate := c.Get("startworkoutdate")
    _endworkoutdate := c.Get("endworkoutdate")

    if _startworkoutdate != "" && _endworkoutdate != "" {        
        var v [2]string
        v[0] = _startworkoutdate
        v[1] = _endworkoutdate  
        args = append(args, models.Where{Column:"workoutdate", Value:v, Compare:"between"})    
    } else if  _startworkoutdate != "" {          
        args = append(args, models.Where{Column:"workoutdate", Value:_startworkoutdate, Compare:">="})
    } else if  _endworkoutdate != "" {          
        args = append(args, models.Where{Column:"workoutdate", Value:_endworkoutdate, Compare:"<="})            
    }
    _startstarttime := c.Get("startstarttime")
    _endstarttime := c.Get("endstarttime")

    if _startstarttime != "" && _endstarttime != "" {        
        var v [2]string
        v[0] = _startstarttime
        v[1] = _endstarttime  
        args = append(args, models.Where{Column:"starttime", Value:v, Compare:"between"})    
    } else if  _startstarttime != "" {          
        args = append(args, models.Where{Column:"starttime", Value:_startstarttime, Compare:">="})
    } else if  _endstarttime != "" {          
        args = append(args, models.Where{Column:"starttime", Value:_endstarttime, Compare:"<="})            
    }
    _duration := c.Geti("duration")
    if _duration != 0 {
        args = append(args, models.Where{Column:"duration", Value:_duration, Compare:"="})    
    }
    _calories := c.Geti("calories")
    if _calories != 0 {
        args = append(args, models.Where{Column:"calories", Value:_calories, Compare:"="})    
    }
    _distance := c.Geti("distance")
    if _distance != 0 {
        args = append(args, models.Where{Column:"distance", Value:_distance, Compare:"="})    
    }
    _memo := c.Get("memo")
    if _memo != "" {
        args = append(args, models.Where{Column:"memo", Value:_memo, Compare:"like"})
        
    }
    _source := c.Get("source")
    if _source != "" {
        args = append(args, models.Where{Column:"source", Value:_source, Compare:"like"})
        
    }
    _externalid := c.Get("externalid")
    if _externalid != "" {
        args = append(args, models.Where{Column:"externalid", Value:_externalid, Compare:"like"})
        
    }
    _startcreateddate := c.Get("startcreateddate")
    _endcreateddate := c.Get("endcreateddate")

    if _startcreateddate != "" && _endcreateddate != "" {        
        var v [2]string
        v[0] = _startcreateddate
        v[1] = _endcreateddate  
        args = append(args, models.Where{Column:"createddate", Value:v, Compare:"between"})    
    } else if  _startcreateddate != "" {          
        args = append(args, models.Where{Column:"createddate", Value:_startcreateddate, Compare:">="})
    } else if  _endcreateddate != "" {          
        args = append(args, models.Where{Column:"createddate", Value:_endcreateddate, Compare:"<="})            
    }
    
    
    
    
    total := manager.Count(args)
	c.Set("total", total)
}

func (c *WorkoutController) Insert(item *models.Workout) {
    
    
    

	conn := c.NewConnection()
    
	manager := models.NewWorkoutManager(conn)
	err := manager.Insert(item)
    if err != nil {
        c.Set("code", "error")    
        c.Set("error", err)
        return
    }

    id := manager.GetIdentity()
    c.Result["id"] = id
    item.Id = id
}

func (c *WorkoutController) Insertbatch(item *[]models.Workout) {  
    if item == nil || len(*item) == 0 {
        return
    }

    rows := len(*item)
    
    
    
	conn := c.NewConnection()
    
	manager := models.NewWorkoutManager(conn)

    for i := 0; i < rows; i++ {
        
	    err := manager.Insert(&((*item)[i]))
        if err != nil {
            c.Set("code", "error")    
            c.Set("error", err)
            return
        }
    }
}

func (c *WorkoutController) Update(item *models.Workout) {
    
    
    

	conn := c.NewConnection()

	manager := models.NewWorkoutManager(conn)
    err := manager.Update(item)
    if err != nil {
        c.Set("code", "error")    
        c.Set("error", err)
        return
    }
}

func (c *WorkoutController) Delete(item *models.Workout) {
    
    
    conn := c.NewConnection()

	manager := models.NewWorkoutManager(conn)

    
	err := manager.Delete(item.Id)
    if err != nil {
        c.Set("code", "error")    
        c.Set("error", err)
    }
}

func (c *WorkoutController) Deletebatch(item *[]models.Workout) {
    
    
    conn := c.NewConnection()

	manager := models.NewWorkoutManager(conn)

    for _, v := range *item {
        
    
	    err := manager.Delete(v.Id)
        if err != nil {
            c.Set("code", "error")    
            c.Set("error", err)
            return
        }
    }
}


