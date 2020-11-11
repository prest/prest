module github.com/prest/prest/cmd

go 1.13

require (
	github.com/aws/aws-lambda-go v1.18.0
	github.com/awslabs/aws-lambda-go-api-proxy v0.8.0
	github.com/gorilla/mux v1.8.0
	github.com/gosidekick/migration/v3 v3.0.0
	github.com/lib/pq v1.8.0
	github.com/nuveo/log v0.0.0-20190430190217-44d02db6bdf8
	github.com/onsi/ginkgo v1.14.0 // indirect
	github.com/prest/prest/adapters v0.0.0-20200729234819-07cc1b6b492f
	github.com/prest/prest/config v0.0.0-20200729234819-07cc1b6b492f
	github.com/prest/prest/controllers v0.0.0-20200729234819-07cc1b6b492f
	github.com/prest/prest/helpers v0.0.0-20200729234819-07cc1b6b492f
	github.com/prest/prest/middlewares v0.0.0-20200729234819-07cc1b6b492f
	github.com/spf13/cobra v1.0.0
	github.com/urfave/negroni v1.0.0
)

replace (
	github.com/prest/prest/adapters => ../adapters
	github.com/prest/prest/config => ../config
	github.com/prest/prest/controllers => ../controllers
	github.com/prest/prest/helpers => ../helpers
	github.com/prest/prest/middlewares => ../middlewares
	github.com/prest/prest/template => ../template
)
