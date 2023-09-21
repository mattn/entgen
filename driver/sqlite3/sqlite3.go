package sqlite3

import (
	"database/sql"
	"fmt"
	"strings"

	"github.com/mattn/entgen/driver"
	_ "github.com/mattn/go-sqlite3"
)

func New() driver.Driver {
	return &Driver{}
}

type Driver struct{}

func typeName(s string) string {
	switch s {
	case "bytea":
		return "Bytes"
	case "real":
		return "Float"
	case "integer", "int":
		return "Int"
	case "character varying", "text":
		return "String"
	case "datetime", "date", "time":
		return "Time"
	}
	if strings.HasPrefix(s, "varchar(") {
		return "String"
	}
	return "Unknown"
}

func database(db *sql.DB) (string, error) {
	stmt, err := db.Prepare(`
    select 
        current_database()
    `)
	if err != nil {
		return "", err
	}
	defer stmt.Close()

	var s string
	err = stmt.QueryRow().Scan(&s)
	if err != nil {
		return "", err
	}
	return s, nil
}

func (dv *Driver) Tables(db *sql.DB) ([]driver.Table, error) {
	stmt, err := db.Prepare(`
    select 
        tbl_name
    from 
        sqlite_master
    where 
        type = 'table'
    and tbl_name <> 'sqlite_sequence'
    `)
	if err != nil {
		return nil, err
	}
	defer stmt.Close()

	rows, err := stmt.Query()
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	tables := []driver.Table{}
	for rows.Next() {
		var s string
		err = rows.Scan(&s)
		if err != nil {
			return nil, err
		}
		tables = append(tables, driver.Table{
			Name: driver.Camelize(s),
			Orig: s,
		})
	}

	return tables, nil
}

func (*Driver) Columns(db *sql.DB, name string) ([]driver.Column, error) {
	stmt, err := db.Prepare(`
    pragma table_info(` + name + `)
    `)
	if err != nil {
		return nil, err
	}
	defer stmt.Close()

	rows, err := stmt.Query()
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	columns := []driver.Column{}
	for rows.Next() {
		var i int
		var s, t string
		var n int
		var d interface{}
		var pk int
		err = rows.Scan(&i, &s, &t, &n, &d, &pk)
		if err != nil {
			return nil, err
		}
		if s == "id" {
			continue
		}
		if d != nil {
			if sd, ok := d.(string); ok {
				sd = strings.Replace(strings.Trim(sd, `'`), `''`, `'`, -1)
				d = fmt.Sprintf("%q", sd)
			}
		}
		t = typeName(strings.ToLower(t))
		columns = append(columns, driver.Column{
			Name:      driver.Camelize(s),
			Orig:      s,
			Default:   d,
			Type:      t,
			Nullable:  n == 0,
			Updatable: true,
		})
	}

	return columns, nil
}
