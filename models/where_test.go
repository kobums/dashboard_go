package models

import (
	"reflect"
	"testing"
)

func TestWhereClause(t *testing.T) {
	tests := []struct {
		name   string
		item   Where
		query  string
		params []interface{}
	}{
		{"prefix", Where{Column: "name", Value: "a", Compare: "="}, " and x_name = ?", []interface{}{"a"}},
		{"full column", Where{Column: "y_name", Value: 1, Compare: ">="}, " and y_name >= ?", []interface{}{1}},
		{"alias column", Where{Column: "t.y_name", Value: 1, Compare: "<>"}, " and t.y_name <> ?", []interface{}{1}},
		{"like", Where{Column: "title", Value: "kw", Compare: "like"}, " and x_title like ?", []interface{}{"%kw%"}},
		{"between", Where{Column: "date", Value: [2]string{"a", "b"}, Compare: "between"}, " and x_date between ? and ?", []interface{}{"a", "b"}},
		{"in ints", Where{Column: "id", Value: []int64{1, 2, 3}, Compare: "in"}, " and x_id in (?, ?, ?)", []interface{}{int64(1), int64(2), int64(3)}},
		{"not in strings", Where{Column: "id", Value: []string{"1) or (1=1"}, Compare: "not in"}, " and x_id not in (?)", []interface{}{"1) or (1=1"}},
		{"in csv string", Where{Column: "id", Value: "1, 2", Compare: "in"}, " and x_id in (?, ?)", []interface{}{"1", "2"}},
		{"in empty", Where{Column: "id", Value: []int{}, Compare: "in"}, " and 1=0", nil},
		{"upper compare", Where{Column: "id", Value: 1, Compare: "IN"}, " and x_id in (?)", []interface{}{1}},

		// 식별자/연산자가 아닌 것은 SQL 에 넣지 않고 거짓 조건으로 바꾼다.
		{"bad column", Where{Column: "id = 1 or 1", Value: 1, Compare: "="}, " and 1=0", nil},
		{"bad column subquery", Where{Column: "(select 1)", Value: 1, Compare: "="}, " and 1=0", nil},
		{"bad column comment", Where{Column: "id--", Value: 1, Compare: "="}, " and 1=0", nil},
		{"empty column", Where{Column: "", Value: 1, Compare: "="}, " and 1=0", nil},
		{"bad compare", Where{Column: "id", Value: 1, Compare: "= 1 or 1 ="}, " and 1=0", nil},
		{"empty compare", Where{Column: "id", Value: 1, Compare: ""}, " and 1=0", nil},
		{"bad between", Where{Column: "date", Value: "a", Compare: "between"}, " and 1=0", nil},
	}

	for _, tt := range tests {
		q, p := WhereClause("x_", tt.item, nil)
		if q != tt.query || !reflect.DeepEqual(p, tt.params) {
			t.Errorf("%s: WhereClause = %q %v, want %q %v", tt.name, q, p, tt.query, tt.params)
		}
	}
}

func TestValidColumn(t *testing.T) {
	for _, ok := range []string{"id", "w_id", "Name", "t.w_id", "_x"} {
		if !ValidColumn(ok) {
			t.Errorf("ValidColumn(%q) = false", ok)
		}
	}
	for _, bad := range []string{"", "1id", "id desc", "id,1", "a.b.c", "count(*)", "id;drop"} {
		if ValidColumn(bad) {
			t.Errorf("ValidColumn(%q) = true", bad)
		}
	}
}
