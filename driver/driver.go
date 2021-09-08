package driver

import (
	"database/sql"
	"strings"
)

type Column struct {
	Name      string
	Orig      string
	Type      string
	Default   interface{}
	MaxLen    sql.NullInt64
	Nullable  bool
	Updatable bool
}
type Table struct {
	Name    string
	Orig    string
	Columns []Column
	HasTime bool
}

type Driver interface {
	Tables(*sql.DB) ([]Table, error)
	Columns(*sql.DB, string) ([]Column, error)
}

func Camelize(s string) string {
	var ret []string
	for _, v := range strings.Fields(s) {
		ret = append(ret, strings.Title(v))
	}
	return strings.Join(ret, "")
}
