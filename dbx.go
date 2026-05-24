package pgbase

import (
	"fmt"

	"github.com/pocketbase/dbx"
	"github.com/pocketbase/pocketbase"
	"github.com/pocketbase/pocketbase/core"
)

func NewSqliteBuilder(db *dbx.DB, executor dbx.Executor) dbx.Builder {
	b := dbx.NewSqliteBuilder(db, executor)
	return &Builder{b}
}

type Builder struct {
	dbx.Builder
}

var _ dbx.Builder = (*Builder)(nil)

func (*Builder) GeneratePlaceholder(i int) string {
	return fmt.Sprintf("$%d", i)
}

func EnableConcurrentWrites(app core.App, config pocketbase.Config) {
	db := app.NonconcurrentDB().(*dbx.DB).DB()
	if config.DataMaxOpenConns <= 0 {
		config.DataMaxOpenConns = core.DefaultDataMaxOpenConns
	}
	if config.DataMaxIdleConns <= 0 {
		config.DataMaxIdleConns = core.DefaultDataMaxIdleConns
	}
	db.SetMaxOpenConns(config.DataMaxOpenConns)
	db.SetMaxIdleConns(config.DataMaxIdleConns)
}
