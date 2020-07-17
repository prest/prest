module github.com/prest/prest/cmd/prestd

go 1.14

require (
	github.com/auth0/go-jwt-middleware v0.0.0-20200507191422-d30d7b9ece63 // indirect
	github.com/clbanning/mxj v1.8.4 // indirect
	github.com/cznic/ql v1.2.0 // indirect
	github.com/fatih/color v1.9.0 // indirect
	github.com/jmoiron/sqlx v1.2.0 // indirect
	github.com/lib/pq v1.7.0 // indirect
	github.com/mattn/go-sqlite3 v1.14.0 // indirect
	github.com/nuveo/log v0.0.0-20190430190217-44d02db6bdf8 // indirect
	github.com/prest/prest/adapters v0.0.0-00010101000000-000000000000 // indirect
	github.com/prest/prest/cmd v0.0.0-00010101000000-000000000000
	github.com/prest/prest/config v0.0.0-00010101000000-000000000000
	github.com/prest/prest/controllers v0.0.0-00010101000000-000000000000 // indirect
	github.com/prest/prest/helpers v0.0.0-00010101000000-000000000000 // indirect
	github.com/prest/prest/middlewares v0.0.0-00010101000000-000000000000 // indirect
	github.com/prest/prest/template v0.0.0-00010101000000-000000000000 // indirect
	github.com/spf13/cobra v1.0.0 // indirect
	github.com/spf13/viper v1.7.0 // indirect
	gopkg.in/mattes/migrate.v1 v1.3.2 // indirect
)

replace github.com/prest/prest/cmd => ../

replace github.com/prest/prest/config => ../../config

replace github.com/prest/prest/adapters => ../../adapters

replace github.com/prest/prest/controllers => ../../controllers

replace github.com/prest/prest/helpers => ../../helpers

replace github.com/prest/prest/middlewares => ../../middlewares

replace github.com/prest/prest/template => ../../template
