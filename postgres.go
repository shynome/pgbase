package pgbase

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"encoding/json"
	"fmt"
	"log"
	"strings"

	"github.com/jackc/pgx/v5/stdlib"
	"github.com/maypok86/otter/v2"
	"github.com/pocketbase/dbx"
	polyglot "github.com/tobilg/polyglot/packages/go"
)

func Open(dsn string) (*dbx.DB, error) {
	if err := func() error {
		db, err := dbx.Open("pgx", dsn)
		if err != nil {
			return err
		}
		defer db.Close()
		if err := createSQLiteEquivalentFunctions(db); err != nil {
			return err
		}
		return nil
	}(); err != nil {
		return nil, err
	}
	db, err := dbx.Open("sqlite2pg", dsn)
	if err != nil {
		return nil, err
	}

	return db, nil
}

var d = &Driver{
	Driver: stdlib.GetDefaultDriver(),
	Cache: otter.Must(&otter.Options[string, string]{
		MaximumSize: 1000,
	}),
}

func GetDefaultDriver() *Driver {
	return d
}

func init() {
	dbx.BuilderFuncMap["sqlite2pg"] = NewSqliteBuilder
	sql.Register("sqlite2pg", d)
}

type Driver struct {
	driver.Driver
	Cache *otter.Cache[string, string]
}

var _ driver.Driver = (*Driver)(nil)
var _ driver.DriverContext = (*Driver)(nil)

func (d *Driver) Open(name string) (driver.Conn, error) {
	conn, err := d.Driver.Open(name)
	if err != nil {
		return nil, err
	}
	return &Conn{conn, d.Cache}, nil
}

func (d *Driver) OpenConnector(name string) (driver.Connector, error) {
	dr := d.Driver.(driver.DriverContext)
	cr, err := dr.OpenConnector(name)
	if err != nil {
		return nil, err
	}
	return &Connecter{cr, d.Cache}, nil
}

type Connecter struct {
	driver.Connector
	Cache *otter.Cache[string, string]
}

var _ driver.Connector = (*Connecter)(nil)

func (c *Connecter) Connect(ctx context.Context) (driver.Conn, error) {
	conn, err := c.Connector.Connect(ctx)
	if err != nil {
		return nil, err
	}
	return &Conn{conn, c.Cache}, nil
}

type Conn struct {
	driver.Conn
	Cache *otter.Cache[string, string]
}

var _ driver.Conn = (*Conn)(nil)
var _ driver.ExecerContext = (*Conn)(nil)

func (c *Conn) Prepare(query string) (driver.Stmt, error) {
	q2, err := c.transpile(query)
	if err != nil {
		return nil, err
	}
	stmt, err := c.Conn.Prepare(q2)
	if err != nil {
		// log.Println("prepare raw", query)
		// log.Println("transpiled", q2)
		// log.Println("err", err)
		return nil, err
	}
	// log.Println("prepare", q2)
	return stmt, nil
}

func (c *Conn) ExecContext(ctx context.Context, query string, args []driver.NamedValue) (driver.Result, error) {
	q2, err := c.transpile(query)
	if err != nil {
		return nil, err
	}
	d, ok := c.Conn.(driver.ExecerContext)
	if !ok {
		return nil, ErrNoTranslatedQuery
	}

	r, err := d.ExecContext(ctx, q2, args)
	if err != nil {
		// log.Println("exec raw", query)
		// log.Println("transpiled", q2)
		// log.Println("err", err)
		return nil, err
	}
	// log.Println("exec", q2)
	return r, nil
}

var queryHasTable = "SELECT (1) FROM `sqlite_schema` WHERE (`type` IN ($1, $2)) AND (LOWER(`name`)=LOWER($3)) LIMIT 1"
var pgQueryHasTable = `SELECT 1 FROM information_schema.tables 
WHERE table_schema = current_schema() 
  AND lower(table_name) = lower($3)
  AND ($1::text IS NOT NULL OR $2::text IS NOT NULL OR TRUE)
LIMIT 1`

var queryHasIndex = "SELECT `tbl_name` FROM `sqlite_master` WHERE (((`type`=$1) AND (LOWER(`tbl_name`)!=LOWER($2))) AND (LOWER(`tbl_name`)!=LOWER($3))) AND (LOWER(`name`)=LOWER($4)) LIMIT 1"
var pgQueryHasIndex = `SELECT tablename AS tbl_name
FROM pg_indexes
WHERE schemaname = current_schema()
  AND lower(indexname) = lower($4)
  AND lower(tablename) <> lower($2)
  AND lower(tablename) <> lower($3)
  AND ($1::text IS NOT NULL OR TRUE)
LIMIT 1`

var querySqlite3 = "SELECT `name`, `sql` FROM `sqlite_master` WHERE (`tbl_name`=$1 AND `type`=$2) AND (sql IS NOT NULL AND name NOT LIKE 'sqlite_autoindex_%')"
var pgQuerySqlite3 = `SELECT indexname AS name,
       indexdef AS sql
FROM pg_indexes
WHERE schemaname = current_schema()
  AND tablename = $1
  AND indexdef IS NOT NULL
  AND indexname NOT LIKE 'sqlite_autoindex_%'
  AND ($2::text IS NOT NULL OR TRUE)   -- 消耗多余的 $2`

func (c *Conn) transpile(query string) (q2 string, err error) {
	if q2, ok := c.Cache.GetIfPresent(query); ok {
		return q2, nil
	}
	defer func() {
		if err == nil {
			c.Cache.Set(query, q2)
		}
	}()
	switch query {
	case "PRAGMA optimize":
		query = "ANALYZE"
		return query, nil
	case queryHasTable:
		query = pgQueryHasTable
		return query, nil
	case queryHasIndex:
		query = pgQueryHasIndex
		return query, nil
	case querySqlite3:
		query = pgQuerySqlite3
		return query, nil
	}
	if strings.Contains(query, "sqlite_") {
		log.Println(query)
		panic("new sqlite_* query is not matched")
	}
	query = strings.Replace(query, ".`_rowid_` DESC", ".`rowid` DESC", 1)
	q2, err = transpile(query, "sqlite", "postgres")
	if err != nil {
		return "", err
	}
	q2, err = fixColTypes(q2)
	return q2, err
}

var ErrNoTranslatedQuery = fmt.Errorf("no translated query")
var ErrBaseDriverNotExecer = fmt.Errorf("base driver don't support driver.ExecerContext")

func fixColTypes(query string) (string, error) {
	switch {
	default:
		return query, nil
	case
		strings.Contains(query, "CREATE TABLE"),
		strings.Contains(query, "DROP INDEX"):
		// 仅转换这两个
	}
	astJson, err := polyglot.Parse(query, "postgres")
	if err != nil {
		return "", err
	}
	var asts []map[string]any
	if err := json.Unmarshal(astJson, &asts); err != nil {
		return "", err
	}
	var changed bool
	for _, ast := range asts {
		func() {
			// 这个在官方修复后可以去掉
			drop, ok := ast["drop_index"].(map[string]any)
			if !ok {
				return
			}
			n, ok := drop["name"].(map[string]any)
			if !ok {
				return
			}
			_, ok = n["quoted"].(bool)
			if !ok {
				return
			}
			n["quoted"] = true
			changed = true
		}()
		func() {
			table, ok := ast["create_table"].(map[string]any)
			if !ok {
				return
			}
			columns, ok := table["columns"].([]any)
			if !ok {
				return
			}
			var hasRowid bool
			for _, col := range columns {
				col, ok := col.(map[string]any)
				if !ok {
					continue
				}
				func() {
					m, ok := col["name"].(map[string]any)
					if !ok {
						return
					}
					name, ok := m["name"].(string)
					if !ok {
						return
					}
					if name == "rowid" {
						hasRowid = true
					}
				}()
				dt, ok := col["data_type"].(map[string]any)
				if !ok {
					continue
				}
				dts, ok := dt["data_type"].(string)
				if !ok {
					continue
				}
				switch dts {
				case "int":
					dt["data_type"] = "big_int"
					changed = true
				}
			}
			if !hasRowid {
				rowid1 := rowid("rowid")
				rowid2 := rowid("_rowid_")
				columns = append(columns, rowid1, rowid2)
				table["columns"] = columns
				changed = true
			}
		}()
	}
	if !changed {
		return query, nil
	}
	buf, err := json.Marshal(asts)
	if err != nil {
		return "", err
	}
	// log.Println(string(buf))
	s, err := polyglot.Generate(buf, "postgres")
	if err != nil {
		return "", err
	}
	query, err = getQ1(s)
	return query, err
}

func transpile(query string, s, t string) (string, error) {
	ss, err := polyglot.Transpile(query, s, t)
	if err != nil {
		return "", err
	}
	return getQ1(ss)
}

func getQ1(ss []string) (string, error) {
	q2 := strings.Join(ss, ";\n")
	return q2, nil
}

func rowid(name string) map[string]any {
	return map[string]any{
		"name":                      map[string]any{"name": name, "quoted": true, "trailing_comments": []any{}},
		"data_type":                 map[string]any{"data_type": "custom", "name": "BIGSERIAL"},
		"nullable":                  false,
		"default":                   nil,
		"primary_key":               false,
		"primary_key_order":         nil,
		"unique":                    false,
		"unique_nulls_not_distinct": false,
		"auto_increment":            false,
		"comment":                   nil,
		"constraints":               []any{},
		"constraint_order":          []string{"NotNull"},
		"format":                    nil,
		"title":                     nil,
		"inline_length":             nil,
		"compress":                  nil,
		"character_set":             nil,
		"uppercase":                 false,
		"casespecific":              nil,
		"auto_increment_start":      nil,
		"auto_increment_increment":  nil,
		"auto_increment_order":      nil,
		"unsigned":                  false,
		"zerofill":                  false,
		"no_type":                   false,
		"not_for_replication":       false,
	}
}
