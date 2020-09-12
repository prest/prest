module github.com/prest/prest/controllers

go 1.13

require (
	github.com/gorilla/mux v1.7.4
	github.com/prest/prest/adapters v0.0.0-20200729234819-07cc1b6b492f
	github.com/prest/prest/config v0.0.0-20200729234819-07cc1b6b492f
	github.com/prest/prest/middlewares v0.0.0-20200729234819-07cc1b6b492f
)

replace (
	github.com/prest/prest/adapters => ../adapters
	github.com/prest/prest/config => ../config
	github.com/prest/prest/controllers => ../controllers
	github.com/prest/prest/helpers => ../helpers
	github.com/prest/prest/middlewares => ../middlewares
	github.com/prest/prest/template => ../template
)
