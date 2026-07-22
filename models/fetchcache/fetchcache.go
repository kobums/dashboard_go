package fetchcache

type Column int

const (
    _ Column = iota
    
    ColumnId
    ColumnCachekey
    ColumnPayload
    ColumnFetchedat
    ColumnCreateddate
)

type Params struct {
    Column Column
    Value interface{}
}




