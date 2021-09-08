package main

import (
	"database/sql"
	"flag"
	"log"
	"os"
	"path/filepath"
	"strings"
	"text/template"

	"github.com/mattn/entgen/driver"
	"github.com/mattn/entgen/driver/postgres"
	"github.com/mattn/entgen/driver/sqlite3"
)

var base = `package schema

import ({{if .HasTime}}
	"time"
{{end}}
	"entgo.io/ent"{{if .Columns}}
	"entgo.io/ent/schema/field"{{end}}
)

// {{.Name}} holds the schema definition for the {{.Name}} entity.
type {{.Name}} struct {
	ent.Schema
}

// Fields of the {{.Name}}.
func ({{.Name}}) Fields() []ent.Field {
	return []ent.Field{
{{range .Columns}}		field.{{.Type}}("{{.Orig}}"){{if ne .Default nil}}.
			Default({{.Default}}){{end}}{{if .Nullable}}.
			NotEmpty(){{end}}{{if .MaxLen.Valid}}.
			MaxLen({{.MaxLen.Int64}}){{end}}{{if not .Updatable}}.
			Immutable(){{end}},
{{end}}	}
}
`

func main() {
	drivers := map[string]driver.Driver{
		"postgres": postgres.New(),
		"sqlite3":  sqlite3.New(),
	}
	var drv string
	var dsn string
	var dir string
	flag.StringVar(&drv, "driver", "postgres", "driver")
	flag.StringVar(&dsn, "dsn", "", "connect string")
	flag.StringVar(&dir, "dir", "ent/schema", "output directory")
	flag.Parse()

	if drv == "" {
		flag.Usage()
		os.Exit(1)
	}

	var dv driver.Driver = drivers[drv]

	db, err := sql.Open(drv, dsn)
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	tpl := template.New("")
	tpl, err = tpl.Parse(base)
	if err != nil {
		log.Fatal(err)
	}

	tbls, err := dv.Tables(db)
	if err != nil {
		log.Fatal(err)
	}

	err = os.MkdirAll(dir, 0755)
	if err != nil {
		log.Fatal(err)
	}
	for _, tbl := range tbls {
		tbl.Columns, err = dv.Columns(db, tbl.Orig)
		if err != nil {
			log.Fatal(err)
		}
		for i, col := range tbl.Columns {
			if col.Type == "Time" && !col.Nullable && col.Default == nil {
				tbl.Columns[i].Default = "time.Now"
				tbl.HasTime = true
			}
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
