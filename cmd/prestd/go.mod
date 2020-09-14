module github.com/prest/prest/cmd/prestd

go 1.14

require (
	github.com/auth0/go-jwt-middleware v0.0.0-20200810150920-a32d7af194d1 // indirect
	github.com/fsnotify/fsnotify v1.4.9 // indirect
	github.com/magiconair/properties v1.8.3 // indirect
	github.com/mitchellh/mapstructure v1.3.3 // indirect
	github.com/pelletier/go-toml v1.8.1 // indirect
	github.com/prest/prest/adapters v0.0.0-20200913010455-d9d3ebc0d554 // indirect
	github.com/prest/prest/cmd v0.0.0-20200913010455-d9d3ebc0d554
	github.com/prest/prest/config v0.0.0-20200913010455-d9d3ebc0d554
	github.com/prest/prest/controllers v0.0.0-20200913010455-d9d3ebc0d554 // indirect
	github.com/prest/prest/helpers v0.0.0-20200913010455-d9d3ebc0d554 // indirect
	github.com/prest/prest/middlewares v0.0.0-20200913010455-d9d3ebc0d554 // indirect
	github.com/prest/prest/template v0.0.0-20200913010455-d9d3ebc0d554 // indirect
	github.com/spf13/afero v1.3.5 // indirect
	github.com/spf13/cast v1.3.1 // indirect
	github.com/spf13/jwalterweatherman v1.1.0 // indirect
	github.com/spf13/pflag v1.0.5 // indirect
	github.com/spf13/viper v1.7.1 // indirect
	golang.org/x/sys v0.0.0-20200909081042-eff7692f9009 // indirect
	gopkg.in/ini.v1 v1.61.0 // indirect
	gopkg.in/yaml.v2 v2.3.0 // indirect
)

replace github.com/prest/prest/cmd => ../

replace github.com/prest/prest/config => ../../config

replace github.com/prest/prest/adapters => ../../adapters

replace github.com/prest/prest/controllers => ../../controllers

replace github.com/prest/prest/helpers => ../../helpers

replace github.com/prest/prest/middlewares => ../../middlewares

replace github.com/prest/prest/template => ../../template
