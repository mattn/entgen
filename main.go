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
	"github.com/mattn/entgen/driver/mysql"
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
{{range .Columns}}		field.{{.Type}}("{{.Orig}}"){{if isnotnil .Default}}.
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
		"mysql":    mysql.New(),
	}
	var drv string
	var dsn string
	var dir string
	var rplural bool
	flag.StringVar(&drv, "driver", "postgres", "driver")
	flag.StringVar(&dsn, "dsn", "", "connect string")
	flag.StringVar(&dir, "dir", "ent/schema", "output directory")
	flag.BoolVar(&rplural, "rplural", false, "remove plural")
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

	tpl, err := template.New("").Funcs(map[string]interface{}{
		"isnotnil": func(a interface{}) bool {
			return a != nil
		},
	}).Parse(base)
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
		name := tbl.Name
		fname := filepath.Join(dir, strings.ToLower(name)+".go")
		log.Printf("Generating %v", fname)
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

		f, err := os.Create(fname)
		if err != nil {
			log.Fatal(err)
		}
		if err := tpl.Execute(f, &tbl); err != nil {
			log.Fatal(err)
		}
		f.Close()
	}
}
