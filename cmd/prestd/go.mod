module github.com/palevi67/prest/cmd/prestd

go 1.14

require (
	github.com/auth0/go-jwt-middleware v0.0.0-20200507191422-d30d7b9ece63 // indirect
	github.com/clbanning/mxj v1.8.4 // indirect
	github.com/cznic/ql v1.2.0 // indirect
	github.com/fatih/color v1.9.0 // indirect
	github.com/fsnotify/fsnotify v1.4.9 // indirect
	github.com/jmoiron/sqlx v1.2.0 // indirect
	github.com/magiconair/properties v1.8.2 // indirect
	github.com/mattn/go-colorable v0.1.7 // indirect
	github.com/mattn/go-sqlite3 v1.14.0 // indirect
	github.com/nuveo/log v0.0.0-20190430190217-44d02db6bdf8 // indirect
	github.com/palevi67/prest/adapters v0.0.0-20200914103049-8b1cf4024d0a // indirect
	github.com/palevi67/prest/cmd v0.0.0-20200914103049-8b1cf4024d0a
	github.com/palevi67/prest/config v0.0.0-20200914103049-8b1cf4024d0a
	github.com/palevi67/prest/controllers v0.0.0-20200914103049-8b1cf4024d0a // indirect
	github.com/palevi67/prest/helpers v0.0.0-20200914103049-8b1cf4024d0a // indirect
	github.com/palevi67/prest/middlewares v0.0.0-20200914103049-8b1cf4024d0a // indirect
	github.com/palevi67/prest/template v0.0.0-20200914103049-8b1cf4024d0a // indirect
	github.com/spf13/cast v1.3.1 // indirect
	github.com/spf13/cobra v1.0.0 // indirect
	github.com/spf13/pflag v1.0.5 // indirect
	github.com/spf13/viper v1.7.1 // indirect
	golang.org/x/text v0.3.3 // indirect
	gopkg.in/ini.v1 v1.51.1 // indirect
	gopkg.in/mattes/migrate.v1 v1.3.2 // indirect
	gopkg.in/yaml.v2 v2.2.8 // indirect
)

replace github.com/palevi67/prest/cmd => ../

replace github.com/palevi67/prest/config => ../../config

replace github.com/palevi67/prest/adapters => ../../adapters

replace github.com/palevi67/prest/controllers => ../../controllers

replace github.com/palevi67/prest/helpers => ../../helpers

replace github.com/palevi67/prest/middlewares => ../../middlewares

replace github.com/palevi67/prest/template => ../../template
