package main

import (
	"database/sql"
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"text/template"

	_ "github.com/lib/pq"
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
}

func typeName(s string) string {
	switch s {
	case "integer", "int":
		return "Int"
	case "character varying":
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

func tables(db *sql.DB) ([]Table, error) {
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

	tables := []Table{}
	for rows.Next() {
		var s string
		err = rows.Scan(&s)
		if err != nil {
			return nil, err
		}
		tables = append(tables, Table{
			Name: camelize(s),
			Orig: s,
		})
	}

	return tables, nil
}

func columns(db *sql.DB, database, table string) ([]Column, error) {
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

	rows, err := stmt.Query(database, table)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	columns := []Column{}
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
		columns = append(columns, Column{
			Name:      camelize(s),
			Orig:      s,
			Default:   d,
			Type:      typeName(t),
			MaxLen:    l,
			Nullable:  n == "YES",
			Updatable: u == "YES",
		})
	}

	return columns, nil
}

func camelize(s string) string {
	var ret []string
	for _, v := range strings.Fields(s) {
		ret = append(ret, strings.Title(v))
	}
	return strings.Join(ret, "")
}

var base = `package schema

import (
	"time"

	"entgo.io/ent"
	"entgo.io/ent/schema/field"
)

// {{.Name}} holds the schema definition for the {{.Name}} entity.
type {{.Name}} struct {
	ent.Schema
}

// Fields of the {{.Name}}.
func ({{.Name}}) Fields() []ent.Field {
	return []ent.Field{
{{range .Columns}}		field.{{.Type}}("{{.Orig}}"){{if ne .Default nil}}
			.Default({{.Default}}){{end}}{{if .Nullable}}
			.NoEmpty(){{end}}{{if .MaxLen.Valid}}
			.MaxLen({{.MaxLen.Int64}}){{end}}{{if not .Updatable}}
			.Immutable(){{end}},
{{end}}	}
}
`

func main() {
	var dsn string
	var dir string
	flag.StringVar(&dsn, "dsn", os.Getenv("PQ_DSN"), "connect string")
	flag.StringVar(&dir, "dir", "ent/schema", "output directory")
	flag.Parse()

	db, err := sql.Open("postgres", dsn)
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	tpl := template.New("")
	tpl, err = tpl.Parse(base)
	if err != nil {
		log.Fatal(err)
	}

	dbase, err := database(db)
	if err != nil {
		log.Fatal(err)
	}

	tbls, err := tables(db)
	if err != nil {
		log.Fatal(err)
	}

	err = os.MkdirAll(dir, 0755)
	if err != nil {
		log.Fatal(err)
	}
	for _, tbl := range tbls {
		tbl.Columns, err = columns(db, dbase, tbl.Orig)
		if err != nil {
			log.Fatal(err)
		}

		f, err := os.Create(filepath.Join(dir, strings.ToLower(tbl.Name)+".go"))
		if err != nil {
			log.Fatal(err)
		}
		if err := tpl.Execute(f, &tbl); err != nil {
			log.Fatal(err)
		}
		f.Close()
	}
}
