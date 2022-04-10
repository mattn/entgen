package mysql

import (
	"database/sql"
	"fmt"
	"strings"

	_ "github.com/go-sql-driver/mysql"
	"github.com/mattn/entgen/driver"
)

func New() driver.Driver {
	return &Driver{}
}

type Driver struct{}

func typeName(s string) string {
	switch s {
	case "binary", "blob":
		return "Bytes"
	case "tinyint", "smallint", "mediumint", "int", "integer", "bigint":
		return "Int"
	case "float", "double", "double precision", "real":
		return "Float"
	case "char", "varchar", "tinytext", "text", "longtext":
		return "String"
	case "datetime", "date", "time", "timestamp":
		return "Time"
	}
	return "Unknown"
}

func database(db *sql.DB) (string, error) {
	stmt, err := db.Prepare(`
    select 
        database()
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
	rows, err := db.Query(`
    show full tables where Table_Type != 'VIEW'
    `)
	defer rows.Close()

	tables := []driver.Table{}
	for rows.Next() {
		var s, t string
		err = rows.Scan(&s, &t)
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
    select 
    	column_name
        , data_type
        , column_default
        , character_maximum_length
        , is_nullable
    from 
    	information_schema.columns 
    where 
        table_catalog = ?
        and table_schema = ?
        and table_name = ?
    order by 
    	ordinal_position;
    `)
	if err != nil {
		return nil, err
	}
	defer stmt.Close()

	dbase, err := database(db)
	if err != nil {
		return nil, err
	}

	rows, err := stmt.Query("def", dbase, name)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	columns := []driver.Column{}
	for rows.Next() {
		var s, t string
		var l sql.NullInt64
		var n string
		var d interface{}
		err = rows.Scan(&s, &t, &d, &l, &n)
		if err != nil {
			return nil, err
		}
		if s == "id" {
			continue
		}
		if d != nil {
			if sd, ok := d.(string); ok {
				pos := strings.Index(sd, "::")
				if pos >= 0 {
					sd = strings.Replace(strings.Trim(sd[:pos], `'`), `''`, `'`, -1)
				}
				d = fmt.Sprintf("%q", sd)
			}
		}
		t = typeName(t)
		columns = append(columns, driver.Column{
			Name:      driver.Camelize(s),
			Orig:      s,
			Default:   d,
			Type:      t,
			MaxLen:    l,
			Nullable:  n == "YES" && (t == "String" || t == "Bytes"),
			Updatable: true,
		})
	}

	return columns, nil
}
