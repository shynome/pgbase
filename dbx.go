package pgbase

import (
	"fmt"

	"github.com/pocketbase/dbx"
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
