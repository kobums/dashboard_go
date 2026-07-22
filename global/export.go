package global

type Export interface {
	Save(string) string
	Cell(string) string
	CellInt(int)
	CellPrice(int)
	CellImage(string)
	SetHeight(float64)
	Remove()
}
