package main

import (
	"strings"

	"github.com/pocketbase/dbx"
	"github.com/pocketbase/pocketbase"
	"github.com/pocketbase/pocketbase/core"
	"github.com/shynome/err0/try"
	"github.com/shynome/pgbase"
	polyglot "github.com/tobilg/polyglot/packages/go"
)

func main() {
	tc := try.To1(polyglot.OpenDefault())
	polyglot.SetDefaultClient(tc)
	cfg := pocketbase.Config{
		DBConnect: func(dbPath string) (*dbx.DB, error) {
			if strings.HasSuffix(dbPath, "data.db") {
				return pgbase.Open("postgres://user:pass@127.0.0.1:5432/postgres?sslmode=disable")
			}
			return core.DefaultDBConnect(dbPath)
		},
		DataMaxOpenConns: core.DefaultDataMaxOpenConns,
		DataMaxIdleConns: core.DefaultDataMaxIdleConns,
	}
	app := pocketbase.NewWithConfig(cfg)
	try.To(app.Bootstrap())
	// postgres support concurrent, reset max conns
	if db, ok := app.NonconcurrentDB().(*dbx.DB); ok {
		db.DB().SetMaxOpenConns(cfg.DataMaxOpenConns)
		db.DB().SetMaxIdleConns(cfg.DataMaxIdleConns)
	}
	try.To(app.Start())
}
