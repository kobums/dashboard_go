package rest


import (
	"dashboard/controllers"
	
	"dashboard/models"

    "strings"
)

type DevstatController struct {
	controllers.Controller
}

func (c *DevstatController) Read(id int64) {
    
    
	conn := c.NewConnection()

	manager := models.NewDevstatManager(conn)
	item := manager.Get(id)

    
    
    c.Set("item", item)
}

func (c *DevstatController) Index(page int, pagesize int) {
    
    
	conn := c.NewConnection()

	manager := models.NewDevstatManager(conn)

    var args []interface{}
    
    _source := c.Get("source")
    if _source != "" {
        args = append(args, models.Where{Column:"source", Value:_source, Compare:"like"})
    }
    _startstatdate := c.Get("startstatdate")
    _endstatdate := c.Get("endstatdate")
    if _startstatdate != "" && _endstatdate != "" {        
        var v [2]string
        v[0] = _startstatdate
        v[1] = _endstatdate  
        args = append(args, models.Where{Column:"statdate", Value:v, Compare:"between"})    
    } else if  _startstatdate != "" {          
        args = append(args, models.Where{Column:"statdate", Value:_startstatdate, Compare:">="})
    } else if  _endstatdate != "" {          
        args = append(args, models.Where{Column:"statdate", Value:_endstatdate, Compare:"<="})            
    }
    _count := c.Geti("count")
    if _count != 0 {
        args = append(args, models.Where{Column:"count", Value:_count, Compare:"="})    
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
                    str += ", ds_" + strings.Trim(v, " ")                
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

func (c *DevstatController) Count() {
    
    
	conn := c.NewConnection()

	manager := models.NewDevstatManager(conn)

    var args []interface{}
    
    _source := c.Get("source")
    if _source != "" {
        args = append(args, models.Where{Column:"source", Value:_source, Compare:"like"})
        
    }
    _startstatdate := c.Get("startstatdate")
    _endstatdate := c.Get("endstatdate")

    if _startstatdate != "" && _endstatdate != "" {        
        var v [2]string
        v[0] = _startstatdate
        v[1] = _endstatdate  
        args = append(args, models.Where{Column:"statdate", Value:v, Compare:"between"})    
    } else if  _startstatdate != "" {          
        args = append(args, models.Where{Column:"statdate", Value:_startstatdate, Compare:">="})
    } else if  _endstatdate != "" {          
        args = append(args, models.Where{Column:"statdate", Value:_endstatdate, Compare:"<="})            
    }
    _count := c.Geti("count")
    if _count != 0 {
        args = append(args, models.Where{Column:"count", Value:_count, Compare:"="})    
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

func (c *DevstatController) Insert(item *models.Devstat) {
    
    
    

	conn := c.NewConnection()
    
	manager := models.NewDevstatManager(conn)
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

func (c *DevstatController) Insertbatch(item *[]models.Devstat) {  
    if item == nil || len(*item) == 0 {
        return
    }

    rows := len(*item)
    
    
    
	conn := c.NewConnection()
    
	manager := models.NewDevstatManager(conn)

    for i := 0; i < rows; i++ {
        
	    err := manager.Insert(&((*item)[i]))
        if err != nil {
            c.Set("code", "error")    
            c.Set("error", err)
            return
        }
    }
}

func (c *DevstatController) Update(item *models.Devstat) {
    
    
    

	conn := c.NewConnection()

	manager := models.NewDevstatManager(conn)
    err := manager.Update(item)
    if err != nil {
        c.Set("code", "error")    
        c.Set("error", err)
        return
    }
}

func (c *DevstatController) Delete(item *models.Devstat) {
    
    
    conn := c.NewConnection()

	manager := models.NewDevstatManager(conn)

    
	err := manager.Delete(item.Id)
    if err != nil {
        c.Set("code", "error")    
        c.Set("error", err)
    }
}

func (c *DevstatController) Deletebatch(item *[]models.Devstat) {
    
    
    conn := c.NewConnection()

	manager := models.NewDevstatManager(conn)

    for _, v := range *item {
        
    
	    err := manager.Delete(v.Id)
        if err != nil {
            c.Set("code", "error")    
            c.Set("error", err)
            return
        }
    }
}


