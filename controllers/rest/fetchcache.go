package rest


import (
	"dashboard/controllers"
	
	"dashboard/models"

    "strings"
)

type FetchcacheController struct {
	controllers.Controller
}

func (c *FetchcacheController) Read(id int64) {
    
    
	conn := c.NewConnection()

	manager := models.NewFetchcacheManager(conn)
	item := manager.Get(id)

    
    
    c.Set("item", item)
}

func (c *FetchcacheController) Index(page int, pagesize int) {
    
    
	conn := c.NewConnection()

	manager := models.NewFetchcacheManager(conn)

    var args []interface{}
    
    _cachekey := c.Get("cachekey")
    if _cachekey != "" {
        args = append(args, models.Where{Column:"cachekey", Value:_cachekey, Compare:"like"})
    }
    _payload := c.Get("payload")
    if _payload != "" {
        args = append(args, models.Where{Column:"payload", Value:_payload, Compare:"like"})
    }
    _startfetchedat := c.Get("startfetchedat")
    _endfetchedat := c.Get("endfetchedat")
    if _startfetchedat != "" && _endfetchedat != "" {        
        var v [2]string
        v[0] = _startfetchedat
        v[1] = _endfetchedat  
        args = append(args, models.Where{Column:"fetchedat", Value:v, Compare:"between"})    
    } else if  _startfetchedat != "" {          
        args = append(args, models.Where{Column:"fetchedat", Value:_startfetchedat, Compare:">="})
    } else if  _endfetchedat != "" {          
        args = append(args, models.Where{Column:"fetchedat", Value:_endfetchedat, Compare:"<="})            
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
                    str += ", fc_" + strings.Trim(v, " ")                
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

func (c *FetchcacheController) Count() {
    
    
	conn := c.NewConnection()

	manager := models.NewFetchcacheManager(conn)

    var args []interface{}
    
    _cachekey := c.Get("cachekey")
    if _cachekey != "" {
        args = append(args, models.Where{Column:"cachekey", Value:_cachekey, Compare:"like"})
        
    }
    _payload := c.Get("payload")
    if _payload != "" {
        args = append(args, models.Where{Column:"payload", Value:_payload, Compare:"like"})
        
    }
    _startfetchedat := c.Get("startfetchedat")
    _endfetchedat := c.Get("endfetchedat")

    if _startfetchedat != "" && _endfetchedat != "" {        
        var v [2]string
        v[0] = _startfetchedat
        v[1] = _endfetchedat  
        args = append(args, models.Where{Column:"fetchedat", Value:v, Compare:"between"})    
    } else if  _startfetchedat != "" {          
        args = append(args, models.Where{Column:"fetchedat", Value:_startfetchedat, Compare:">="})
    } else if  _endfetchedat != "" {          
        args = append(args, models.Where{Column:"fetchedat", Value:_endfetchedat, Compare:"<="})            
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

func (c *FetchcacheController) Insert(item *models.Fetchcache) {
    
    
    

	conn := c.NewConnection()
    
	manager := models.NewFetchcacheManager(conn)
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

func (c *FetchcacheController) Insertbatch(item *[]models.Fetchcache) {  
    if item == nil || len(*item) == 0 {
        return
    }

    rows := len(*item)
    
    
    
	conn := c.NewConnection()
    
	manager := models.NewFetchcacheManager(conn)

    for i := 0; i < rows; i++ {
        
	    err := manager.Insert(&((*item)[i]))
        if err != nil {
            c.Set("code", "error")    
            c.Set("error", err)
            return
        }
    }
}

func (c *FetchcacheController) Update(item *models.Fetchcache) {
    
    
    

	conn := c.NewConnection()

	manager := models.NewFetchcacheManager(conn)
    err := manager.Update(item)
    if err != nil {
        c.Set("code", "error")    
        c.Set("error", err)
        return
    }
}

func (c *FetchcacheController) Delete(item *models.Fetchcache) {
    
    
    conn := c.NewConnection()

	manager := models.NewFetchcacheManager(conn)

    
	err := manager.Delete(item.Id)
    if err != nil {
        c.Set("code", "error")    
        c.Set("error", err)
    }
}

func (c *FetchcacheController) Deletebatch(item *[]models.Fetchcache) {
    
    
    conn := c.NewConnection()

	manager := models.NewFetchcacheManager(conn)

    for _, v := range *item {
        
    
	    err := manager.Delete(v.Id)
        if err != nil {
            c.Set("code", "error")    
            c.Set("error", err)
            return
        }
    }
}


