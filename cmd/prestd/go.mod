module github.com/prest/prest/cmd/prestd

go 1.14

require (
	github.com/prest/prest/cmd v0.0.0-00010101000000-000000000000
	github.com/prest/prest/config v0.0.0-20200729234819-07cc1b6b492f
)

replace github.com/prest/prest/cmd => ../

replace github.com/prest/prest/config => ../../config

replace github.com/prest/prest/adapters => ../../adapters

replace github.com/prest/prest/controllers => ../../controllers

replace github.com/prest/prest/helpers => ../../helpers

replace github.com/prest/prest/middlewares => ../../middlewares

replace github.com/prest/prest/template => ../../template
