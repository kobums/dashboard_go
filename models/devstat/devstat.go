package devstat

type Column int

const (
    _ Column = iota
    
    ColumnId
    ColumnSource
    ColumnStatdate
    ColumnCount
    ColumnCreateddate
)

type Params struct {
    Column Column
    Value interface{}
}




