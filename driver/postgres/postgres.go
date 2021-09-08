package postgres

import (
	"database/sql"
	"fmt"
	"strings"

	"github.com/mattn/entgen/driver"
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
	case "timestamp without time zone", "timestamp with time zone":
		return "Time"
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
        relname
    from 
        pg_stat_user_tables
    where
        schemaname = 'public'
    order by
        relname
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
    select 
    	column_name
        , data_type
        , column_default
        , character_maximum_length
        , is_nullable
        , is_updatable
    from 
    	information_schema.columns 
    where 
        table_catalog = $1
        and table_name = $2
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

	rows, err := stmt.Query(dbase, name)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	columns := []driver.Column{}
	for rows.Next() {
		var s, t string
		var l sql.NullInt64
		var n, u string
		var d interface{}
		err = rows.Scan(&s, &t, &d, &l, &n, &u)
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
					d = strings.Replace(strings.Trim(sd[:pos], `'`), `''`, `'`, -1)
				}
				d = fmt.Sprintf("%q", d)
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
			Updatable: u == "YES",
		})
	}

	return columns, nil
}
