package healthmetric

type Column int

const (
    _ Column = iota
    
    ColumnId
    ColumnMetricdate
    ColumnName
    ColumnQty
    ColumnUnit
    ColumnCreateddate
)

type Params struct {
    Column Column
    Value interface{}
}




