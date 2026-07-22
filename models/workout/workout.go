package workout

type Column int

const (
    _ Column = iota
    
    ColumnId
    ColumnType
    ColumnTitle
    ColumnWorkoutdate
    ColumnStarttime
    ColumnDuration
    ColumnCalories
    ColumnDistance
    ColumnMemo
    ColumnSource
    ColumnExternalid
    ColumnCreateddate
)

type Params struct {
    Column Column
    Value interface{}
}




