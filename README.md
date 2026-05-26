#  Achive

pocketbase 是基于 sqlite 开发的, 并非适用于所有数据库, 有大量 hacker 专门是给 sqlite 用的, 作者也明确说了暂时不会考虑其他数据库, 所以这条路走不通, 还是 [fondoger/pocketbase] 的方案更好.

对于需要多写的场景我的建议是混合 sqlite 和 pg 进行开发

# 简介

成功实现了多写, 通过[自定义 DBConnect](https://pocketbase.io/docs/go-overview/#custom-sqlite-driver) 将 [PocketBase](https://github.com/pocketbase/pocketbase) 的存储后端由 sqlite 切换到 postgres

# 原理

使用 [Polyglot](https://github.com/tobilg/polyglot#go) 将 sqlite 语句翻译成 postgres 语句

# 测试

```sh
docker-compose up -d
cd ./example
# download libpolyglot_sql_ffi.so in https://github.com/tobilg/polyglot/releases
go run -v . serve
```

# 示例

```go
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
	}
	app := pocketbase.NewWithConfig(cfg)
	try.To(app.Bootstrap())

	// required!
	// required!
	// required!
	pgbase.EnableConcurrentWrites(app, cfg)

	try.To(app.Start())
}


```

# 参考

- [fondoger/pocketbase]

[fondoger/pocketbase]: https://github.com/fondoger/pocketbase
