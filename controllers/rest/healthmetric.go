package rest


import (
	"dashboard/controllers"
	
	"dashboard/models"

    "strings"
)

type HealthmetricController struct {
	controllers.Controller
}

func (c *HealthmetricController) Read(id int64) {
    
    
	conn := c.NewConnection()

	manager := models.NewHealthmetricManager(conn)
	item := manager.Get(id)

    
    
    c.Set("item", item)
}

func (c *HealthmetricController) Index(page int, pagesize int) {
    
    
	conn := c.NewConnection()

	manager := models.NewHealthmetricManager(conn)

    var args []interface{}
    
    _startmetricdate := c.Get("startmetricdate")
    _endmetricdate := c.Get("endmetricdate")
    if _startmetricdate != "" && _endmetricdate != "" {        
        var v [2]string
        v[0] = _startmetricdate
        v[1] = _endmetricdate  
        args = append(args, models.Where{Column:"metricdate", Value:v, Compare:"between"})    
    } else if  _startmetricdate != "" {          
        args = append(args, models.Where{Column:"metricdate", Value:_startmetricdate, Compare:">="})
    } else if  _endmetricdate != "" {          
        args = append(args, models.Where{Column:"metricdate", Value:_endmetricdate, Compare:"<="})            
    }
    _name := c.Get("name")
    if _name != "" {
        args = append(args, models.Where{Column:"name", Value:_name, Compare:"="})
        
    }
    _qty := c.Geti("qty")
    if _qty != 0 {
        args = append(args, models.Where{Column:"qty", Value:_qty, Compare:"="})    
    }
    _unit := c.Get("unit")
    if _unit != "" {
        args = append(args, models.Where{Column:"unit", Value:_unit, Compare:"like"})
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
                    str += ", hm_" + strings.Trim(v, " ")                
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

func (c *HealthmetricController) Count() {
    
    
	conn := c.NewConnection()

	manager := models.NewHealthmetricManager(conn)

    var args []interface{}
    
    _startmetricdate := c.Get("startmetricdate")
    _endmetricdate := c.Get("endmetricdate")

    if _startmetricdate != "" && _endmetricdate != "" {        
        var v [2]string
        v[0] = _startmetricdate
        v[1] = _endmetricdate  
        args = append(args, models.Where{Column:"metricdate", Value:v, Compare:"between"})    
    } else if  _startmetricdate != "" {          
        args = append(args, models.Where{Column:"metricdate", Value:_startmetricdate, Compare:">="})
    } else if  _endmetricdate != "" {          
        args = append(args, models.Where{Column:"metricdate", Value:_endmetricdate, Compare:"<="})            
    }
    _name := c.Get("name")
    if _name != "" {
        args = append(args, models.Where{Column:"name", Value:_name, Compare:"="})
        
        
    }
    _qty := c.Geti("qty")
    if _qty != 0 {
        args = append(args, models.Where{Column:"qty", Value:_qty, Compare:"="})    
    }
    _unit := c.Get("unit")
    if _unit != "" {
        args = append(args, models.Where{Column:"unit", Value:_unit, Compare:"like"})
        
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

func (c *HealthmetricController) Insert(item *models.Healthmetric) {
    
    
    

	conn := c.NewConnection()
    
	manager := models.NewHealthmetricManager(conn)
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

func (c *HealthmetricController) Insertbatch(item *[]models.Healthmetric) {  
    if item == nil || len(*item) == 0 {
        return
    }

    rows := len(*item)
    
    
    
	conn := c.NewConnection()
    
	manager := models.NewHealthmetricManager(conn)

    for i := 0; i < rows; i++ {
        
	    err := manager.Insert(&((*item)[i]))
        if err != nil {
            c.Set("code", "error")    
            c.Set("error", err)
            return
        }
    }
}

func (c *HealthmetricController) Update(item *models.Healthmetric) {
    
    
    

	conn := c.NewConnection()

	manager := models.NewHealthmetricManager(conn)
    err := manager.Update(item)
    if err != nil {
        c.Set("code", "error")    
        c.Set("error", err)
        return
    }
}

func (c *HealthmetricController) Delete(item *models.Healthmetric) {
    
    
    conn := c.NewConnection()

	manager := models.NewHealthmetricManager(conn)

    
	err := manager.Delete(item.Id)
    if err != nil {
        c.Set("code", "error")    
        c.Set("error", err)
    }
}

func (c *HealthmetricController) Deletebatch(item *[]models.Healthmetric) {
    
    
    conn := c.NewConnection()

	manager := models.NewHealthmetricManager(conn)

    for _, v := range *item {
        
    
	    err := manager.Delete(v.Id)
        if err != nil {
            c.Set("code", "error")    
            c.Set("error", err)
            return
        }
    }
}


