module github.com/prest/prest/adapters

go 1.13

require (
	github.com/casbin/casbin/v2 v2.17.0
	github.com/jmoiron/sqlx v1.2.0
	github.com/lib/pq v1.8.0
	github.com/nuveo/log v0.0.0-20190430190217-44d02db6bdf8
	github.com/prest/prest/config v0.0.0-20200729234819-07cc1b6b492f
	github.com/prest/prest/template v0.0.0-20200729234819-07cc1b6b492f
)

replace (
	github.com/prest/prest/adapters => ../adapters
	github.com/prest/prest/config => ../config
	github.com/prest/prest/controllers => ../controllers
	github.com/prest/prest/helpers => ../helpers
	github.com/prest/prest/middlewares => ../middlewares
	github.com/prest/prest/template => ../template
)
